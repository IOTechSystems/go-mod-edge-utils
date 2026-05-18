//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// flushableRecorder wraps httptest.ResponseRecorder with http.Flusher
// and a no-op SetWriteDeadline (httptest's buffer never blocks, so the
// production deadline check has nothing to enforce; implementing it lets
// http.ResponseController succeed without forcing a tolerance branch
// into pkg/sse/utils.go).
type flushableRecorder struct {
	*httptest.ResponseRecorder
	flushes int
	mu      sync.Mutex
}

func (f *flushableRecorder) Flush() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.flushes++
	f.ResponseRecorder.Flush()
}

func (f *flushableRecorder) SetWriteDeadline(_ time.Time) error { return nil }

func newSubscribeFixture(t *testing.T) (*flushableRecorder, echo.Context, context.CancelFunc) {
	t.Helper()
	rec := &flushableRecorder{ResponseRecorder: httptest.NewRecorder()}
	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/sse", nil).WithContext(reqCtx)
	e := echo.New()
	c := e.NewContext(req, rec)
	return rec, c, cancel
}

func TestSubscribe_ReturnsErrNoTopic_WhenTopicMissing(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))
	_, c, cancel := newSubscribeFixture(t)
	defer cancel()

	err := tr.Subscribe(c, "never-started")
	assert.True(t, errors.Is(err, ErrNoTopic), "expected ErrNoTopic, got %v", err)
}

// readSSEEvents parses `data: <json>\n\n` frames from body; heartbeats
// (`:\n\n`) are skipped.
func readSSEEvents(t *testing.T, body string) []string {
	t.Helper()
	var events []string
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			events = append(events, strings.TrimPrefix(line, "data: "))
		}
	}
	require.NoError(t, scanner.Err())
	return events
}

func TestSubscribe_RunningJob_DeliversLiveEvents(t *testing.T) {
	tr := New(context.Background(), time.Second, 5*time.Second, newTestLogger(t))
	job, _ := tr.Start("topic-a")

	rec, c, cancelReq := newSubscribeFixture(t)
	subscribeDone := make(chan error, 1)
	go func() {
		subscribeDone <- tr.Subscribe(c, "topic-a")
	}()

	time.Sleep(20 * time.Millisecond) // let attach happen

	job.Publish(map[string]any{"progress": 50, "message": "halfway"})
	job.Finish(map[string]any{"progress": 100, "message": "done"})

	select {
	case err := <-subscribeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		cancelReq()
		t.Fatal("Subscribe did not exit after Finish")
	}

	events := readSSEEvents(t, rec.Body.String())
	require.GreaterOrEqual(t, len(events), 2)
	assert.Contains(t, events[len(events)-1], `"progress":100`)

	assert.Equal(t, "text/event-stream", rec.Header().Get(echo.HeaderContentType))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
}

func TestSubscribe_LateSubscriberSeesOnlyLatest(t *testing.T) {
	// Cache-last semantics: late subscribers replay only the most recent
	// event. Intermediate publishes are coalesced — callers must design
	// events as monotonic snapshots (see package README "Scope").
	tr := New(context.Background(), time.Second, 5*time.Second, newTestLogger(t))
	job, _ := tr.Start("topic-a")

	job.Publish(map[string]any{"progress": 10, "message": "step 1"})
	job.Publish(map[string]any{"progress": 30, "message": "step 2"})
	job.Publish(map[string]any{"progress": 50, "message": "step 3"})

	rec, c, cancelReq := newSubscribeFixture(t)
	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	time.Sleep(50 * time.Millisecond)
	cancelReq()
	select {
	case <-subscribeDone:
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not exit on request cancellation")
	}

	events := readSSEEvents(t, rec.Body.String())
	require.GreaterOrEqual(t, len(events), 1, "got: %v", events)
	assert.Contains(t, events[0], `"progress":50`)
	for _, e := range events {
		assert.NotContains(t, e, `"progress":10`, "older snapshot must not appear: %s", e)
		assert.NotContains(t, e, `"progress":30`, "older snapshot must not appear: %s", e)
	}
}

func TestSubscribe_RetainedReplay_WritesTerminalAndCloses(t *testing.T) {
	tr := New(context.Background(), 5*time.Second, 5*time.Second, newTestLogger(t))
	job, _ := tr.Start("topic-a")
	job.Publish(map[string]any{"progress": 0, "message": "started"})
	job.Publish(map[string]any{"progress": 50, "message": "halfway"})
	job.Finish(map[string]any{"progress": 100, "message": "done"})

	rec, c, cancelReq := newSubscribeFixture(t)
	defer cancelReq()

	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	select {
	case err := <-subscribeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not return promptly on retained replay")
	}

	events := readSSEEvents(t, rec.Body.String())
	require.Len(t, events, 1, "expected only the cached terminal, got: %v", events)
	assert.Contains(t, events[0], `"progress":100`)
}

func TestSubscribe_RetainedReplay_FinishWithoutPriorPublish(t *testing.T) {
	// Finish without any preceding Publish: Finish unconditionally sets
	// hasLast=true with the terminal payload, so a late subscriber on the
	// retained entry must still see exactly one frame (the terminal).
	tr := New(context.Background(), 5*time.Second, 5*time.Second, newTestLogger(t))
	job, _ := tr.Start("topic-a")
	job.Finish(map[string]any{"progress": 100, "message": "done"})

	rec, c, cancelReq := newSubscribeFixture(t)
	defer cancelReq()

	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	select {
	case err := <-subscribeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Subscribe did not return promptly on retained replay")
	}

	events := readSSEEvents(t, rec.Body.String())
	require.Len(t, events, 1, "expected the terminal frame, got: %v", events)
	assert.Contains(t, events[0], `"progress":100`)
}

func TestSubscribe_LiveEventsAfterReplay(t *testing.T) {
	// Verify the seam: the latest cached event replays first, then live
	// publishes follow in order, ending with the terminal.
	tr := New(context.Background(), time.Second, 5*time.Second, newTestLogger(t))
	job, _ := tr.Start("topic-a")

	job.Publish(map[string]any{"progress": 10, "message": "history-1"})
	job.Publish(map[string]any{"progress": 20, "message": "history-2"})

	rec, c, cancelReq := newSubscribeFixture(t)
	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	time.Sleep(50 * time.Millisecond) // let replay drain

	job.Publish(map[string]any{"progress": 30, "message": "live-1"})
	job.Finish(map[string]any{"progress": 100, "message": "done"})

	select {
	case err := <-subscribeDone:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		cancelReq()
		t.Fatal("Subscribe did not exit after Finish")
	}

	events := readSSEEvents(t, rec.Body.String())
	require.GreaterOrEqual(t, len(events), 2, "got: %v", events)
	// Replay must be the most recent pre-subscribe payload (progress=20),
	// not progress=10 (coalesced away). Match on the unique message
	// rather than `"progress":10}` — the latter depends on `progress`
	// being the last JSON field, which only holds while it sorts last
	// alphabetically among the keys (json.Marshal sorts map keys).
	assert.Contains(t, events[0], `"progress":20`)
	for _, e := range events {
		assert.NotContains(t, e, `"history-1"`, "older snapshot must not appear: %s", e)
	}
	// Terminal is last.
	assert.Contains(t, events[len(events)-1], `"progress":100`)
}

func TestSubscribe_Heartbeat_EmittedOnIdleStream(t *testing.T) {
	tr := New(context.Background(), time.Second, 25*time.Millisecond, newTestLogger(t))
	_, _ = tr.Start("topic-a")

	rec, c, cancelReq := newSubscribeFixture(t)

	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	// Wait for ≥1 heartbeat, then cancel and read body only after
	// the subscribe goroutine has fully exited (avoid racing on
	// rec.Body's *bytes.Buffer).
	time.Sleep(80 * time.Millisecond)
	cancelReq()
	select {
	case <-subscribeDone:
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not exit on cancel")
	}

	assert.Contains(t, rec.Body.String(), ":\n\n", "expected heartbeat frame")
}

func TestSubscribe_ExitsOnRequestCancellation(t *testing.T) {
	tr := New(context.Background(), time.Second, 5*time.Second, newTestLogger(t))
	_, _ = tr.Start("topic-a")

	_, c, cancelReq := newSubscribeFixture(t)

	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	time.Sleep(20 * time.Millisecond)
	cancelReq()

	select {
	case err := <-subscribeDone:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not exit promptly on request cancel")
	}
}

func TestSubscribe_ExitsOnTrackerShutdown(t *testing.T) {
	parentCtx, cancelTracker := context.WithCancel(context.Background())
	tr := New(parentCtx, time.Second, 5*time.Second, newTestLogger(t))
	_, _ = tr.Start("topic-a")

	_, c, cancelReq := newSubscribeFixture(t)
	defer cancelReq()

	subscribeDone := make(chan error, 1)
	go func() { subscribeDone <- tr.Subscribe(c, "topic-a") }()

	time.Sleep(20 * time.Millisecond)
	cancelTracker()

	select {
	case err := <-subscribeDone:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Subscribe did not exit on tracker shutdown")
	}
}
