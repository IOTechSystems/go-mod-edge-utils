//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
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
