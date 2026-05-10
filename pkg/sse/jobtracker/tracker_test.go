//
// Copyright (C) 2026 IOTech Ltd
//

package jobtracker

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// envelope mirrors the wire shape a real caller would publish + store as terminal.
type envelope struct {
	Details progressEvent `json:"details"`
}

type progressEvent struct {
	Progress int    `json:"progress"`
	Message  string `json:"message,omitempty"`
}

func newTestTracker(t *testing.T, retention time.Duration) *Tracker {
	t.Helper()
	lc := log.InitLogger("test", "INFO", nil)
	return New(context.Background(), retention, time.Second, lc)
}

func TestNew_PanicsOnNonPositiveRetention(t *testing.T) {
	lc := log.InitLogger("test", "INFO", nil)
	require.Panics(t, func() { New(context.Background(), 0, time.Second, lc) })
	require.Panics(t, func() { New(context.Background(), -time.Second, time.Second, lc) })
}

func TestNew_PanicsOnNonPositiveHeartbeat(t *testing.T) {
	lc := log.InitLogger("test", "INFO", nil)
	require.Panics(t, func() { New(context.Background(), time.Second, 0, lc) })
	require.Panics(t, func() { New(context.Background(), time.Second, -time.Second, lc) })
}

func TestLookupJob_ReturnsFalseWhenAbsent(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	state, ok := tr.LookupJob("missing")
	require.False(t, ok)
	require.Nil(t, state)
}

func TestStartJob_ExposesFinishedAndState(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, replaced := tr.StartJob("topic1")
	require.NotNil(t, job)
	require.False(t, replaced, "first StartJob on a fresh topic must not report replaced")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.Nil(t, state.Terminal, "terminal must be nil while job is running")
	require.NotNil(t, state.Finished, "finished channel must be created at StartJob time")

	select {
	case <-state.Finished:
		t.Fatal("finished channel must not be closed while job is running")
	default:
	}
}

func TestStartJob_ReplacedFlagOnExistingEntry(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	tr.StartJob("topic1")
	_, replaced := tr.StartJob("topic1")
	require.True(t, replaced, "second StartJob on the same topic must report replaced")
}

func TestFinish_ClosesFinishedAndExposesTerminalThenRemoves(t *testing.T) {
	retention := 50 * time.Millisecond
	tr := newTestTracker(t, retention)
	job, _ := tr.StartJob("topic1")

	before, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.NotNil(t, before.Finished)

	job.Finish(envelope{Details: progressEvent{Progress: 100}})

	select {
	case <-before.Finished:
	default:
		t.Fatal("finished channel must be closed when Finish is called")
	}

	after, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.NotNil(t, after.Terminal, "terminal must be visible during retention window")
	env, ok := after.Terminal.(envelope)
	require.True(t, ok, "terminal must round-trip as the published shape, got %T", after.Terminal)
	require.Equal(t, 100, env.Details.Progress)

	require.Eventually(t, func() bool {
		_, stillThere := tr.LookupJob("topic1")
		return !stillThere
	}, retention*5, 5*time.Millisecond, "entry must be removed after retention expires")
}

// Replacing a finished entry during its retention window must not be undone by
// the prior Finish's retention timer.
func TestStartJob_ResetsRetentionTimer(t *testing.T) {
	retention := 50 * time.Millisecond
	tr := newTestTracker(t, retention)

	job1, _ := tr.StartJob("topic1")
	job1.Finish(envelope{Details: progressEvent{Progress: 100}})

	time.Sleep(20 * time.Millisecond)
	tr.StartJob("topic1") // replaces the prior entry

	require.Never(t, func() bool {
		_, ok := tr.LookupJob("topic1")
		return !ok
	}, retention*3, 5*time.Millisecond, "fresh entry must survive the prior retention timer")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.Nil(t, state.Terminal)
	require.NotNil(t, state.Finished, "fresh entry must have an open finished channel")
}

// Cancelling the tracker's context must cause the retention goroutine to wake
// early AND run cleanup, so no entries leak across shutdown.
func TestNew_ShutdownTriggersCleanup(t *testing.T) {
	lc := log.InitLogger("test", "INFO", nil)
	ctx, cancel := context.WithCancel(context.Background())
	tr := New(ctx, time.Hour, time.Second, lc) // retention is large; only shutdown can wake cleanup in time

	job, _ := tr.StartJob("topic1")
	job.Finish(envelope{Details: progressEvent{Progress: 100}})

	cancel()

	require.Eventually(t, func() bool {
		_, ok := tr.LookupJob("topic1")
		return !ok
	}, time.Second, 5*time.Millisecond, "shutdown must trigger entry cleanup")
}

// Finish broadcasts the terminal payload to live subscribers, so callers do
// not need to call Publish for the last event.
func TestFinish_BroadcastsToLiveSubscribers(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	ch := state.Subscribe()
	defer state.Unsubscribe(ch)

	payload := envelope{Details: progressEvent{Progress: 100, Message: "done"}}
	job.Finish(payload)

	select {
	case got := <-ch:
		require.Equal(t, payload, got, "live subscriber must receive the terminal payload")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("live subscriber did not receive terminal event from Finish")
	}
}

// jobtracker has no payload deduplication, so two consecutive identical
// payloads (e.g. progress=100 via Publish, then Finish(progress=100)) both
// reach live subscribers.
func TestFinish_TerminalReachesLiveSubscriberAfterIdenticalPublish(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	ch := state.Subscribe()
	defer state.Unsubscribe(ch)

	payload := envelope{Details: progressEvent{Progress: 100, Message: "done"}}

	job.Publish(payload)
	select {
	case got := <-ch:
		require.Equal(t, payload, got)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("subscriber did not receive Publish")
	}

	job.Finish(payload)
	select {
	case got := <-ch:
		require.Equal(t, payload, got, "live subscriber must receive terminal even when payload matches recent Publish")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("live subscriber did not receive terminal event from Finish")
	}
}

// Finish is idempotent.
func TestFinish_IsIdempotent(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	first := envelope{Details: progressEvent{Progress: 100, Message: "first"}}
	second := envelope{Details: progressEvent{Progress: -1, Message: "second"}}

	job.Finish(first)
	require.NotPanics(t, func() { job.Finish(second) }, "second Finish must not panic")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.Equal(t, first, state.Terminal, "second Finish must not overwrite the retained terminal")
}

// Calling Finish on a Job whose entry has already been replaced must not
// affect the new entry's terminal/finished state.
func TestFinish_OnReplacedEntryDoesNotAffectFreshEntry(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	stale, _ := tr.StartJob("topic1")
	tr.StartJob("topic1") // replace; stale.entry is no longer in the map

	stale.Finish(envelope{Details: progressEvent{Progress: -1, Message: "stale"}})

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.Nil(t, state.Terminal, "fresh entry must not have a terminal set by stale Job")
	require.NotNil(t, state.Finished)
	select {
	case <-state.Finished:
		t.Fatal("fresh entry's finished channel must not be closed by stale Job")
	default:
	}
}

// A Finish on a stale entry must still close that entry's finished channel so
// any subscribers stranded on the orphaned entry are released rather than
// blocking on it forever. The fresh entry is unaffected (covered by the test
// above).
func TestFinish_OnStaleEntryReleasesOrphanedSubscribers(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	stale, _ := tr.StartJob("topic1")

	// Capture the stale entry's state BEFORE replacement; afterwards LookupJob
	// returns the fresh entry, not this one.
	staleState, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	staleCh := staleState.Subscribe() // simulate a subscriber attached to the orphaned entry
	defer staleState.Unsubscribe(staleCh)

	tr.StartJob("topic1") // replace; staleState now points at the orphaned entry

	stale.Finish(envelope{Details: progressEvent{Progress: -1, Message: "stale"}})

	select {
	case <-staleState.Finished:
	default:
		t.Fatal("stale entry's finished channel must be closed so orphaned subscribers can exit")
	}
}

// Same orphan-release contract as Finish, for Abort.
func TestAbort_OnStaleEntryReleasesOrphanedSubscribers(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	stale, _ := tr.StartJob("topic1")

	staleState, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	staleCh := staleState.Subscribe()
	defer staleState.Unsubscribe(staleCh)

	tr.StartJob("topic1") // replace

	stale.Abort()

	select {
	case <-staleState.Finished:
	default:
		t.Fatal("stale entry's finished channel must be closed by Abort to release orphaned subscribers")
	}
	// Fresh entry stays put.
	fresh, ok := tr.LookupJob("topic1")
	require.True(t, ok, "Abort on stale must not delete the fresh entry")
	require.NotNil(t, fresh.Finished)
	select {
	case <-fresh.Finished:
		t.Fatal("Abort on stale must not close the fresh entry's finished channel")
	default:
	}
}

// The README's canonical pattern: a deferred Finish (with a pessimistic
// terminal payload) acts as a safety net so that if the publisher panics
// before reporting a terminal, the entry is still closed and the safety-net
// terminal is retained for late subscribers. This guards against goroutine
// leaks and "stuck forever" behaviour on subscriber side when the publisher
// crashes.
func TestFinish_DeferredSafetyNetRetainsTerminalOnPublisherPanic(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	safetyNet := envelope{Details: progressEvent{Progress: -1, Message: "did not complete"}}

	require.NotPanics(t, func() {
		defer job.Finish(safetyNet)
		defer func() { _ = recover() }()
		panic("publisher boom")
	})

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	require.NotNil(t, state.Terminal, "deferred safety-net Finish must retain a terminal")
	require.Equal(t, safetyNet, state.Terminal)

	select {
	case <-state.Finished:
	default:
		t.Fatal("deferred safety-net Finish must close the entry's finished channel")
	}
}

func newEchoContext(t *testing.T) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/listen", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// liveWriter is a concurrency-safe http.ResponseWriter for live-stream tests.
//
// Two test-only requirements drive this:
//   - SetWriteDeadline must succeed (httptest.ResponseRecorder doesn't
//     implement it, so http.NewResponseController returns ErrNotSupported and
//     streamLive's per-write deadline fails on the first event).
//   - Body reads from the test goroutine race with body writes from the
//     stream goroutine; httptest.ResponseRecorder's bytes.Buffer is not
//     concurrent-safe.
//
// Header/WriteHeader/Flush delegate to a real ResponseRecorder because Echo
// touches them but the test doesn't need to assert on them concurrently.
type liveWriter struct {
	rec *httptest.ResponseRecorder
	mu  sync.Mutex
	buf bytes.Buffer
}

func newLiveWriter() *liveWriter {
	return &liveWriter{rec: httptest.NewRecorder()}
}

func (w *liveWriter) Header() http.Header              { return w.rec.Header() }
func (w *liveWriter) WriteHeader(code int)             { w.rec.WriteHeader(code) }
func (w *liveWriter) Flush()                           { w.rec.Flush() }
func (w *liveWriter) SetWriteDeadline(time.Time) error { return nil }

func (w *liveWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *liveWriter) body() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func newEchoContextLive(t *testing.T) (echo.Context, *liveWriter) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/listen", nil)
	w := newLiveWriter()
	return e.NewContext(req, w), w
}

func TestStream_NoJobReturnsErrNoJob(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	c, rec := newEchoContext(t)

	err := tr.Stream(c, "missing")
	require.ErrorIs(t, err, ErrNoJob)
	require.Empty(t, rec.Body.String(), "no body should be written when there is no job")
}

func TestStream_FinishedJobReplaysTerminal(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")
	job.Finish(envelope{Details: progressEvent{Progress: 100, Message: "done"}})

	c, rec := newEchoContext(t)
	err := tr.Stream(c, "topic1")
	require.NoError(t, err)

	require.Equal(t, "text/event-stream", rec.Header().Get(echo.HeaderContentType))
	require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	require.Equal(t, "keep-alive", rec.Header().Get("Connection"))

	body := rec.Body.String()
	require.True(t, strings.HasPrefix(body, "data: "), "body must be SSE-formatted, got: %q", body)
	require.True(t, strings.HasSuffix(body, "\n\n"), "SSE event must terminate with blank line, got: %q", body)
	require.Contains(t, body, `"progress":100`)
	require.Contains(t, body, `"message":"done"`)
}

// End-to-end check of the live Stream path: a subscriber connects while the
// job is running, receives a non-terminal Publish, then receives the terminal
// Finish event before Stream returns. This exercises streamLive's writeEvent
// loop AND the Finish-time drain branch (the drain is sometimes the path that
// delivers the terminal, depending on goroutine scheduling).
func TestStream_LivePathDeliversProgressAndTerminal(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	c, w := newEchoContextLive(t)

	streamDone := make(chan error, 1)
	go func() { streamDone <- tr.Stream(c, "topic1") }()

	// Wait until the stream goroutine has Subscribed, otherwise our Publish
	// races the Subscribe and may be dropped (Publish fans out only to current
	// subscribers).
	require.Eventually(t, func() bool {
		state, ok := tr.LookupJob("topic1")
		if !ok {
			return false
		}
		state.entry.mu.RLock()
		defer state.entry.mu.RUnlock()
		return len(state.entry.subscribers) > 0
	}, time.Second, 5*time.Millisecond, "stream goroutine did not Subscribe in time")

	job.Publish(envelope{Details: progressEvent{Progress: 50, Message: "halfway"}})

	require.Eventually(t, func() bool {
		return strings.Contains(w.body(), `"progress":50`)
	}, time.Second, 5*time.Millisecond, "progress event was not written")

	job.Finish(envelope{Details: progressEvent{Progress: 100, Message: "done"}})

	select {
	case err := <-streamDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Stream did not return after Finish")
	}

	require.Equal(t, "text/event-stream", w.Header().Get(echo.HeaderContentType))
	body := w.body()
	require.Contains(t, body, `"progress":50`, "progress event must reach client")
	require.Contains(t, body, `"progress":100`, "terminal event must reach client (live or via drain)")
}

// Subscriber churn (subscribe + unsubscribe) on a running job must not affect
// the entry — jobtracker entries are publisher-owned, not subscriber-owned.
// This is the property the previous design sacrificed by routing through
// sse.Manager's auto-remove broadcaster.
func TestStartJob_EntrySurvivesSubscriberChurn(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)

	ch := state.Subscribe()
	state.Unsubscribe(ch)

	require.Never(t, func() bool {
		_, found := tr.LookupJob("topic1")
		return !found
	}, 200*time.Millisecond, 10*time.Millisecond,
		"entry must survive subscriber churn on a running job")

	state2, ok := tr.LookupJob("topic1")
	require.True(t, ok)
	ch2 := state2.Subscribe()
	defer state2.Unsubscribe(ch2)

	payload := envelope{Details: progressEvent{Progress: 100, Message: "done"}}
	job.Finish(payload)

	select {
	case got := <-ch2:
		require.Equal(t, payload, got, "post-churn subscriber must receive terminal from Finish")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("post-churn subscriber did not receive terminal event from Finish")
	}
}

// Abort removes the entry immediately, releases live subscribers via the
// finished channel, and does not retain a terminal payload.
func TestAbort_RemovesEntryAndReleasesSubscribers(t *testing.T) {
	tr := newTestTracker(t, time.Hour)
	job, _ := tr.StartJob("topic1")

	state, ok := tr.LookupJob("topic1")
	require.True(t, ok)

	job.Abort()

	_, stillThere := tr.LookupJob("topic1")
	require.False(t, stillThere, "Abort must remove the entry immediately")

	select {
	case <-state.Finished:
	default:
		t.Fatal("Abort must close the entry's finished channel")
	}
}

// Abort and Finish share the once-guard: whichever runs first wins.
func TestAbort_SharesOnceGuardWithFinish(t *testing.T) {
	t.Run("Finish then Abort: Finish wins", func(t *testing.T) {
		tr := newTestTracker(t, time.Hour)
		job, _ := tr.StartJob("topic1")

		payload := envelope{Details: progressEvent{Progress: 100, Message: "done"}}
		job.Finish(payload)
		require.NotPanics(t, func() { job.Abort() }, "Abort after Finish must not panic")

		state, ok := tr.LookupJob("topic1")
		require.True(t, ok, "Finish-then-Abort must leave the entry retained, not removed")
		require.Equal(t, payload, state.Terminal, "terminal must remain set by Finish")
	})

	t.Run("Abort then Finish: Abort wins", func(t *testing.T) {
		tr := newTestTracker(t, time.Hour)
		job, _ := tr.StartJob("topic1")

		job.Abort()
		require.NotPanics(t, func() { job.Finish(envelope{}) }, "Finish after Abort must not panic")

		_, ok := tr.LookupJob("topic1")
		require.False(t, ok, "Abort must keep entry removed even if Finish runs after")
	})
}
