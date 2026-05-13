//
// Copyright (C) 2025 IOTech Ltd
//

package sse

// Publisher is an interface for publishing data to subscribers.
type Publisher interface {
	Publish(data any)
}

// PollingService periodically fetches data and publishes it to subscribers.
// One PollingService is attached to a broadcaster when the first subscriber
// connects (via WithPollingService).
type PollingService interface {
	// Start begins fetching. Called once per broadcaster, on the first
	// subscribe.
	Start(publisher Publisher)

	// Stop ends fetching. Called in a background goroutine when the last
	// subscriber leaves; Manager does not wait for it. If Stop never
	// returns, that goroutine leaks.
	//
	// Stop may also be called when Start was never called, so guard
	// against nil fields.
	//
	// See NewPolling for the bundled implementation and an example.
	Stop() error
}
