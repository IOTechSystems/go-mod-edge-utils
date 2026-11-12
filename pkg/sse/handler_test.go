//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const handlerTestTopic = "/sse"

// serveSSE runs h in a goroutine for a request with the given Authorization
// header (none when empty) and returns a channel that receives its result.
func serveSSE(h echo.HandlerFunc, ctx context.Context, auth string) <-chan error {
	req := httptest.NewRequest(http.MethodGet, handlerTestTopic, nil).WithContext(ctx)
	if auth != "" {
		req.Header.Set(echo.HeaderAuthorization, auth)
	}
	rec := &deadlineCapableRecorder{ResponseRecorder: httptest.NewRecorder()}
	c := echo.New().NewContext(req, rec)

	done := make(chan error, 1)
	go func() {
		done <- h(c)
	}()
	return done
}

// expiresIn returns an exp at least d from now, as exp is truncated to the second.
func expiresIn(d time.Duration) time.Time {
	return time.Now().Truncate(time.Second).Add(d + time.Second)
}

// bearerJWT returns an Authorization header value for a JWT expiring at exp.
func bearerJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)}).
		SignedString([]byte("test-key"))
	require.NoError(t, err)
	return "Bearer " + token
}

func waitForDone(t *testing.T, done <-chan error, timeout time.Duration, msg string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatal(msg)
		return nil
	}
}

func waitForSubscribers(t *testing.T, m *Manager, topic string, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		m.mu.RLock()
		defer m.mu.RUnlock()
		b, ok := m.broadcasters[topic]
		if !ok {
			return want == 0
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		return len(b.subscribers) == want
	}, time.Second, 5*time.Millisecond)
}

// TestHandler_WithJWTDeadline_ClosesConnectionOnExpiry verifies that the
// stream ends once the JWT expires, and that leaving as the last subscriber
// stops the topic's polling service.
func TestHandler_WithJWTDeadline_ClosesConnectionOnExpiry(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	svc := &trackingPollingService{}
	h := Handler(m, WithPollingService(svc), WithJWTDeadline())
	done := serveSSE(h, context.Background(), bearerJWT(t, expiresIn(time.Second)))

	err := waitForDone(t, done, 3*time.Second, "handler did not return after the JWT expired")
	assert.NoError(t, err)

	waitForSubscribers(t, m, handlerTestTopic, 0)
	assert.Eventually(t, func() bool { return svc.stops.Load() == 1 }, time.Second, 5*time.Millisecond,
		"polling should stop once the only subscriber's JWT expires")
}

// TestHandler_WithJWTDeadline_PerRequestExpiry verifies that a single Handler
// built once with WithJWTDeadline closes each connection at its own JWT expiry.
func TestHandler_WithJWTDeadline_PerRequestExpiry(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	svc := &trackingPollingService{}
	h := Handler(m, WithPollingService(svc), WithJWTDeadline())

	longCtx, cancelLong := context.WithCancel(context.Background())
	defer cancelLong()
	longDone := serveSSE(h, longCtx, bearerJWT(t, time.Now().Add(time.Hour)))
	waitForSubscribers(t, m, handlerTestTopic, 1)

	shortDone := serveSSE(h, context.Background(), bearerJWT(t, expiresIn(time.Second)))
	waitForSubscribers(t, m, handlerTestTopic, 2)

	err := waitForDone(t, shortDone, 3*time.Second, "short-lived JWT connection did not close after the JWT expired")
	assert.NoError(t, err)

	waitForSubscribers(t, m, handlerTestTopic, 1)
	select {
	case <-longDone:
		t.Fatal("long-lived JWT connection must keep streaming after another connection's JWT expired")
	default:
	}
	assert.Equal(t, int32(0), svc.stops.Load(), "shared polling must keep running while a subscriber remains")

	cancelLong()
	assert.NoError(t, waitForDone(t, longDone, time.Second, "long-lived JWT connection did not return after its request was cancelled"))
}

// TestHandler_WithJWTDeadline_ExpiredJWT verifies that a request whose JWT has
// already expired is rejected with 403 without subscribing.
func TestHandler_WithJWTDeadline_ExpiredJWT(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	svc := &trackingPollingService{}
	done := serveSSE(Handler(m, WithPollingService(svc), WithJWTDeadline()), context.Background(), bearerJWT(t, time.Now().Add(-time.Minute)))
	err := waitForDone(t, done, time.Second, "handler did not return for an expired JWT")

	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusForbidden, httpErr.Code)
	assert.Contains(t, httpErr.Message, "token has expired", "the UI refreshes the token only when the message says so")

	m.mu.RLock()
	_, exists := m.broadcasters[handlerTestTopic]
	m.mu.RUnlock()
	assert.False(t, exists, "an expired JWT must not create a broadcaster")
	assert.Equal(t, int32(0), svc.starts.Load(), "an expired JWT must not start polling")
}

// TestHandler_WithJWTDeadline_NoJWT verifies that a request without a JWT is
// served without a deadline instead of being rejected.
func TestHandler_WithJWTDeadline_NoJWT(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := serveSSE(Handler(m, WithJWTDeadline()), ctx, "")
	waitForSubscribers(t, m, handlerTestTopic, 1)

	select {
	case <-done:
		t.Fatal("a request without a JWT must not be closed by WithJWTDeadline")
	case <-time.After(200 * time.Millisecond):
	}

	cancel()
	assert.NoError(t, waitForDone(t, done, time.Second, "handler did not return after its request was cancelled"))
}

// syncRecorder is a ResponseRecorder whose body can be read while the
// handler is still writing to it.
type syncRecorder struct {
	mu  sync.Mutex
	rec *httptest.ResponseRecorder
}

func newSyncRecorder() *syncRecorder { return &syncRecorder{rec: httptest.NewRecorder()} }

func (s *syncRecorder) Header() http.Header { return s.rec.Header() }

func (s *syncRecorder) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Write(b)
}

func (s *syncRecorder) WriteHeader(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.WriteHeader(code)
}

func (s *syncRecorder) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rec.Flush()
}

func (s *syncRecorder) SetWriteDeadline(_ time.Time) error { return nil }

func (s *syncRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rec.Body.String()
}

type ctxKey struct{}

// openStream serves h on handlerTestTopic until the returned close function is
// called, which waits for the handler to return.
func openStream(t *testing.T, h echo.HandlerFunc) (*syncRecorder, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "request"))
	req := httptest.NewRequest(http.MethodGet, handlerTestTopic, nil).WithContext(ctx)
	rec := newSyncRecorder()
	c := echo.New().NewContext(req, rec)

	done := make(chan error, 1)
	go func() { done <- h(c) }()

	var once sync.Once
	closeStream := func() {
		once.Do(func() {
			cancel()
			assert.NoError(t, waitForDone(t, done, time.Second, "handler did not return after its request was cancelled"))
		})
	}
	t.Cleanup(closeStream)
	return rec, closeStream
}

func waitForBody(t *testing.T, rec *syncRecorder, want string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(rec.body(), want) }, time.Second, 5*time.Millisecond,
		"body never contained %q, got %q", want, rec.body())
}

// TestHandler_WithFirstMessage_BeforeQueuedPayload verifies that the first
// message is written before a payload that was already waiting in the
// subscriber's queue when the option returned.
func TestHandler_WithFirstMessage_BeforeQueuedPayload(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	first := func(_ context.Context, _ PollingService) any {
		m.Publish(handlerTestTopic, "queued")
		return "first"
	}
	rec, closeStream := openStream(t, Handler(m, WithFirstMessage(first)))
	waitForBody(t, rec, `data: "queued"`)
	closeStream()

	assert.Equal(t, "data: \"first\"\n\ndata: \"queued\"\n\n", rec.body())
}

// TestHandler_WithFirstMessage_NilWritesNothing verifies that an option
// returning nil produces the same output as a handler without the option.
func TestHandler_WithFirstMessage_NilWritesNothing(t *testing.T) {
	cases := map[string][]HandlerOption{
		"without option": nil,
		"nil payload": {WithFirstMessage(func(_ context.Context, _ PollingService) any {
			return nil
		})},
	}
	for name, opts := range cases {
		t.Run(name, func(t *testing.T) {
			m := newTestManager(t)
			defer m.Shutdown()

			rec, closeStream := openStream(t, Handler(m, opts...))
			waitForSubscribers(t, m, handlerTestTopic, 1)
			m.Publish(handlerTestTopic, "broadcast")
			waitForBody(t, rec, `data: "broadcast"`)
			closeStream()

			assert.Equal(t, "data: \"broadcast\"\n\n", rec.body())
			assert.Equal(t, http.StatusOK, rec.rec.Code)
			assert.Equal(t, "text/event-stream", rec.Header().Get(echo.HeaderContentType))
		})
	}
}

// TestHandler_WithFirstMessage_AfterPollingStarted verifies that for the
// topic's first subscriber the option runs after polling has started, and
// receives the request context and the topic's polling service.
func TestHandler_WithFirstMessage_AfterPollingStarted(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	svc := &trackingPollingService{}
	type call struct {
		starts  int32
		service PollingService
		value   any
	}
	calls := make(chan call, 1)
	first := func(ctx context.Context, service PollingService) any {
		calls <- call{starts: svc.starts.Load(), service: service, value: ctx.Value(ctxKey{})}
		return "first"
	}
	rec, _ := openStream(t, Handler(m, WithPollingService(svc), WithFirstMessage(first)))

	select {
	case got := <-calls:
		assert.Equal(t, int32(1), got.starts, "polling must have started before the option runs")
		assert.Same(t, svc, got.service)
		assert.Equal(t, "request", got.value, "the option must receive the request context")
	case <-time.After(time.Second):
		t.Fatal("first message option was not called")
	}
	waitForBody(t, rec, `data: "first"`)
}

// TestHandler_WithFirstMessage_BoundPollingService verifies that a later
// subscriber's option receives the polling service bound when the topic's
// broadcaster was created, not the one its own request passed.
func TestHandler_WithFirstMessage_BoundPollingService(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	bound := &trackingPollingService{}
	_, _ = openStream(t, Handler(m, WithPollingService(bound)))
	waitForSubscribers(t, m, handlerTestTopic, 1)

	own := &trackingPollingService{}
	services := make(chan PollingService, 1)
	first := func(_ context.Context, service PollingService) any {
		services <- service
		return nil
	}
	_, _ = openStream(t, Handler(m, WithPollingService(own), WithFirstMessage(first)))

	select {
	case got := <-services:
		assert.Same(t, bound, got, "a later subscriber must get the broadcaster's polling service")
	case <-time.After(time.Second):
		t.Fatal("first message option was not called")
	}
	assert.Equal(t, int32(1), bound.starts.Load())
	assert.Equal(t, int32(0), own.starts.Load(), "a later subscriber's own polling service must not start")
}
