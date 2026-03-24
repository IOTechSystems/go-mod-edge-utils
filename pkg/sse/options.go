//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import "time"

// HandlerConfig holds the configuration for the SSE handler.
type HandlerConfig struct {
	PollingService PollingService
	CustomTopic    string
}

// HandlerOption is a function that modifies the HandlerConfig.
type HandlerOption func(*HandlerConfig)

// WithPollingService returns a HandlerOption that sets the PollingService in the HandlerConfig.
func WithPollingService(service PollingService) HandlerOption {
	return func(config *HandlerConfig) {
		config.PollingService = service
	}
}

// WithCustomTopic returns a HandlerOption that sets a custom topic in the HandlerConfig.
func WithCustomTopic(topic string) HandlerOption {
	return func(config *HandlerConfig) {
		config.CustomTopic = topic
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
