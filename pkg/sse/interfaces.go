//
// Copyright (C) 2025 IOTech Ltd
//

package sse

// Publisher is an interface for publishing data to subscribers.
type Publisher interface {
	Publish(data any)
}

// PollingService is an interface for a service that periodically fetches data and publishes it to subscribers.
type PollingService interface {
	Start(publisher Publisher)
	Stop() error
}
