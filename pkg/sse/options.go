//
// Copyright (C) 2025 IOTech Ltd
//

package sse

// HandlerConfig holds the configuration for the SSE handler.
type HandlerConfig struct {
	PollingService PollingService
}

// HandlerOption is a function that modifies the HandlerConfig.
type HandlerOption func(*HandlerConfig)

// WithPollingService returns a HandlerOption that sets the PollingService in the HandlerConfig.
func WithPollingService(service PollingService) HandlerOption {
	return func(config *HandlerConfig) {
		config.PollingService = service
	}
}
