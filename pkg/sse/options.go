//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import (
	"context"
	"time"
)

// HandlerConfig holds the configuration for the SSE handler.
type HandlerConfig struct {
	PollingService PollingService
	CustomTopic    string
	JWTDeadline    bool
	FirstMessage   FirstMessageFunc
}

// FirstMessageFunc returns a subscriber's own first message. service is the
// polling service bound to the topic's broadcaster when it was created (nil
// when it has none), not necessarily the one the current request passed.
//
// Only an untyped nil writes nothing: a nil pointer returned as any is not
// nil and is written as "data: null", so return nil explicitly when there is
// no payload. A payload that cannot be marshalled to JSON is logged and
// skipped, as a broadcast payload is.
type FirstMessageFunc func(ctx context.Context, service PollingService) any

// HandlerOption is a function that modifies the HandlerConfig.
type HandlerOption func(*HandlerConfig)

// WithPollingService returns a HandlerOption that sets the PollingService in the HandlerConfig.
func WithPollingService(service PollingService) HandlerOption {
	return func(config *HandlerConfig) {
		config.PollingService = service
	}
}

// WithFirstMessage writes the payload fn returns to each new subscriber before
// anything broadcast on the topic, including payloads already in its queue. fn
// runs after the subscriber is registered and, for the topic's first
// subscriber, after polling has started.
//
// ctx is the request context: it ends when the client leaves or its JWT
// deadline passes, but not on Manager.Shutdown. fn must return promptly and
// honour ctx, as heartbeats, request cancellation and shutdown are handled
// only after it returns.
func WithFirstMessage(fn FirstMessageFunc) HandlerOption {
	return func(config *HandlerConfig) {
		config.FirstMessage = fn
	}
}

// WithCustomTopic returns a HandlerOption that sets a custom topic in the HandlerConfig.
func WithCustomTopic(topic string) HandlerOption {
	return func(config *HandlerConfig) {
		config.CustomTopic = topic
	}
}

// WithJWTDeadline closes each SSE connection when the Bearer JWT in its Authorization header expires.
// An already-expired JWT is rejected with 403; a JWT without exp, or sent only in a cookie, gets no deadline.
// The signature is NOT verified, so use this only behind an authentication middleware.
func WithJWTDeadline() HandlerOption {
	return func(config *HandlerConfig) {
		config.JWTDeadline = true
	}
}

type PollingConfig struct {
	interval      time.Duration
	ApiVersion    string
	StopCondition func(data any) bool
	StopCallback  func()
}

// PollingOption is a function that modifies the PollingConfig.
type PollingOption func(*PollingConfig)

// WithCustomPollingInterval returns a PollingOption that sets a custom polling interval in the PollingConfig.
// Default is 5 seconds if not set.
func WithCustomPollingInterval(interval time.Duration) PollingOption {
	return func(config *PollingConfig) {
		config.interval = interval
	}
}

// WithCustomApiVersion returns a PollingOption that sets a custom API version in the PollingConfig,
// which is used to present the API version for the error response when polling fails.
// Default is common.ApiVersion set in go-mod-edge-utils if not set.
func WithCustomApiVersion(apiVersion string) PollingOption {
	return func(config *PollingConfig) {
		config.ApiVersion = apiVersion
	}
}

// WithStopCondition returns a PollingOption that sets a stop condition function in the PollingConfig.
// Polling will stop after publishing data if the function returns true.
func WithStopCondition(fn func(any) bool) PollingOption {
	return func(config *PollingConfig) {
		config.StopCondition = fn
	}
}

// WithStopCallback returns a PollingOption that sets a callback to be invoked when polling stops.
// The callback is called regardless of the reason polling stopped (stop condition met, explicit Stop(), or context cancellation).
func WithStopCallback(fn func()) PollingOption {
	return func(config *PollingConfig) {
		config.StopCallback = fn
	}
}
