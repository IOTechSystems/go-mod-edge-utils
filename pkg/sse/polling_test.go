//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	iotechErrors "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"
	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPollingService is a minimal PollingService for use in HandlerConfig tests.
type mockPollingService struct{}

func (m *mockPollingService) Start(_ Publisher) {}
func (m *mockPollingService) Stop() error       { return nil }

// mockPublisher records published values.
type mockPublisher struct {
	mu     sync.Mutex
	values []any
}

func (p *mockPublisher) Publish(data any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.values = append(p.values, data)
}

func (p *mockPublisher) published() []any {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]any, len(p.values))
	copy(result, p.values)
	return result
}

// newTestLogger returns a mock logger with all expected calls stubbed via maybe-match.
func newTestLogger(t *testing.T) *loggerMocks.Logger {
	lc := &loggerMocks.Logger{}
	lc.On("Debugf", mock.Anything, mock.Anything, mock.Anything).Maybe()
	lc.On("Debug", mock.Anything, mock.Anything).Maybe()
	lc.On("Errorf", mock.Anything, mock.Anything, mock.Anything).Maybe()
	return lc
}

// TestNewPolling_Defaults verifies that NewPolling applies default interval and API version.
func TestNewPolling_Defaults(t *testing.T) {
	lc := newTestLogger(t)
	p := NewPolling(lc, func(_ context.Context) (any, error) { return nil, nil })

	assert.Equal(t, 5*time.Second, p.interval)
	assert.Equal(t, "v1", p.apiVersion)
	assert.Nil(t, p.stopCondition)
}

// TestNewPolling_WithOptions verifies that options are applied correctly.
func TestNewPolling_WithOptions(t *testing.T) {
	lc := newTestLogger(t)
	fn := func(data any) bool { return data == "stop" }

	p := NewPolling(lc,
		func(_ context.Context) (any, error) { return nil, nil },
		WithCustomPollingInterval(2*time.Second),
		WithCustomApiVersion("v2"),
		WithStopCondition(fn),
	)

	assert.Equal(t, 2*time.Second, p.interval)
	assert.Equal(t, "v2", p.apiVersion)
	require.NotNil(t, p.stopCondition)
	assert.True(t, p.stopCondition("stop"))
	assert.False(t, p.stopCondition("other"))
}

// TestPolling_PublishesData verifies that the polling function result is published.
func TestPolling_PublishesData(t *testing.T) {
	lc := newTestLogger(t)
	published := make(chan any, 1)

	pub := &mockPublisher{}
	called := make(chan struct{}, 1)

	p := NewPolling(lc,
		func(_ context.Context) (any, error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return "result", nil
		},
		WithCustomPollingInterval(100*time.Millisecond),
	)

	p.Start(pub)
	// Wait for at least one poll
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("polling function was not called within timeout")
	}
	_ = published
	require.NoError(t, p.Stop())

	vals := pub.published()
	require.NotEmpty(t, vals)
	assert.Equal(t, "result", vals[0])
}

// TestPolling_PublishesErrorResponse verifies that an error from the polling function
// results in a BaseResponse being published with the correct status code.
func TestPolling_PublishesErrorResponse(t *testing.T) {
	lc := newTestLogger(t)
	pub := &mockPublisher{}
	done := make(chan struct{})

	httpErr := iotechErrors.NewHTTPError(iotechErrors.NewBaseError(iotechErrors.KindEntityDoesNotExist, "not found", nil))

	p := NewPolling(lc,
		func(_ context.Context) (any, error) {
			select {
			case <-done:
			default:
				close(done)
			}
			return nil, httpErr
		},
		WithCustomPollingInterval(100*time.Millisecond),
	)

	p.Start(pub)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("polling function was not called within timeout")
	}
	require.NoError(t, p.Stop())

	vals := pub.published()
	require.NotEmpty(t, vals)
	resp, ok := vals[0].(rest.BaseResponse)
	require.True(t, ok, "expected rest.BaseResponse")
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestPolling_PublishesErrorResponse_PlainError verifies that a plain (non-iotechErrors.Error)
// error results in a 500 status code.
func TestPolling_PublishesErrorResponse_PlainError(t *testing.T) {
	lc := newTestLogger(t)
	pub := &mockPublisher{}
	done := make(chan struct{})

	p := NewPolling(lc,
		func(_ context.Context) (any, error) {
			select {
			case <-done:
			default:
				close(done)
			}
			return nil, errors.New("something broke")
		},
		WithCustomPollingInterval(100*time.Millisecond),
	)

	p.Start(pub)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("polling function was not called within timeout")
	}
	require.NoError(t, p.Stop())

	vals := pub.published()
	require.NotEmpty(t, vals)
	resp, ok := vals[0].(rest.BaseResponse)
	require.True(t, ok, "expected rest.BaseResponse")
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// TestPolling_StopCondition verifies that polling stops when the stop condition is met.
func TestPolling_StopCondition(t *testing.T) {
	lc := newTestLogger(t)
	pub := &mockPublisher{}

	p := NewPolling(lc,
		func(_ context.Context) (any, error) {
			return "final", nil
		},
		WithCustomPollingInterval(50*time.Millisecond),
		WithStopCondition(func(data any) bool { return data == "final" }),
	)

	p.Start(pub)
	// Polling should self-terminate; Stop should not hang
	done := make(chan struct{})
	go func() {
		require.NoError(t, p.Stop())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within timeout after stop condition was met")
	}
}

// TestPolling_Stop_BeforeStart verifies that Stop does not panic when called before Start.
func TestPolling_Stop_BeforeStart(t *testing.T) {
	lc := newTestLogger(t)
	p := NewPolling(lc, func(_ context.Context) (any, error) { return nil, nil })
	assert.NoError(t, p.Stop())
}

// TestNewPolling_WithStopCallback verifies that WithStopCallback wires the callback into the Polling struct.
func TestNewPolling_WithStopCallback(t *testing.T) {
	lc := newTestLogger(t)
	called := false
	fn := func() { called = false } // value doesn't matter; we just check it is non-nil and stored
	_ = called

	p := NewPolling(lc,
		func(_ context.Context) (any, error) { return nil, nil },
		WithStopCallback(fn),
	)

	require.NotNil(t, p.stopCallback)
}

// TestPolling_StopCallback_CalledOnStop verifies that the stop callback is invoked when Stop() is called.
func TestPolling_StopCallback_CalledOnStop(t *testing.T) {
	lc := newTestLogger(t)
	pub := &mockPublisher{}
	callbackCh := make(chan struct{}, 1)

	p := NewPolling(lc,
		func(_ context.Context) (any, error) { return "data", nil },
		WithCustomPollingInterval(50*time.Millisecond),
		WithStopCallback(func() { callbackCh <- struct{}{} }),
	)

	p.Start(pub)
	require.NoError(t, p.Stop())

	select {
	case <-callbackCh:
	case <-time.After(time.Second):
		t.Fatal("stop callback was not invoked within timeout after Stop()")
	}
}

// TestPolling_StopCallback_CalledOnStopCondition verifies that the stop callback is invoked
// when the stop condition causes polling to self-terminate.
func TestPolling_StopCallback_CalledOnStopCondition(t *testing.T) {
	lc := newTestLogger(t)
	pub := &mockPublisher{}
	callbackCh := make(chan struct{}, 1)

	p := NewPolling(lc,
		func(_ context.Context) (any, error) { return "final", nil },
		WithCustomPollingInterval(50*time.Millisecond),
		WithStopCondition(func(data any) bool { return data == "final" }),
		WithStopCallback(func() { callbackCh <- struct{}{} }),
	)

	p.Start(pub)
	// Wait for the callback — self-termination via stop condition should trigger it.
	select {
	case <-callbackCh:
	case <-time.After(2 * time.Second):
		t.Fatal("stop callback was not invoked within timeout after stop condition was met")
	}
	require.NoError(t, p.Stop())
}
