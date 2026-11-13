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

// FirstMessageFunc returns a subscriber's first message and its skipDuplicate
// predicate. service is the topic's polling service (nil when none), not
// necessarily the one this request passed. Return an untyped nil for no first
// message; a nil pointer is written as "data: null".
//
// The subscriber is registered before fn runs, so a broadcast that first
// already contains can be waiting in its queue. skipDuplicate drops such a
// broadcast. The library does not look inside messages: deciding that one is
// such a duplicate is the caller's job, typically by a sequence number its
// messages carry. It is not a general filter: a dropped message counts as
// sent, so an identical broadcast right after it is suppressed by
// de-duplication. It is called for each message taken from the subscriber's
// queue, in order, also when first is nil, and must return promptly; nil
// drops none.
type FirstMessageFunc func(ctx context.Context, service PollingService) (first any, skipDuplicate func(msg any) bool)

// HandlerOption is a function that modifies the HandlerConfig.
type HandlerOption func(*HandlerConfig)

// WithPollingService returns a HandlerOption that sets the PollingService in the HandlerConfig.
func WithPollingService(service PollingService) HandlerOption {
	return func(config *HandlerConfig) {
		config.PollingService = service
	}
}

// WithFirstMessage writes the payload fn returns to each new subscriber before
// anything broadcast on the topic, including payloads already in its queue,
// then drops the broadcasts its skipDuplicate predicate reports as duplicates
// (see FirstMessageFunc). fn runs after the subscriber is registered and, for
// the topic's first subscriber, after polling has started.
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
