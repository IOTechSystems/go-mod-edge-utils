//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import (
	"context"
	"time"

	"github.com/labstack/echo/v4"
)

const defaultHeartbeatInterval = 30 * time.Second

// Handler returns an echo.HandlerFunc that opens an SSE stream for the
// caller. It atomically subscribes to the topic (creating the topic's
// broadcaster on demand) and forwards every published event to the response.
//
// Options:
//   - WithCustomTopic: override the auto-derived topic (defaults to the
//     request URL path + query).
//   - WithPollingService: attach a polling service that produces events for
//     the topic. The service starts on the first subscribe and stops when
//     the last subscriber leaves.
func Handler(m *Manager, opts ...HandlerOption) echo.HandlerFunc {
	config := &HandlerConfig{}
	for _, opt := range opts {
		opt(config)
	}

	return func(c echo.Context) error {
		topic := config.CustomTopic
		if topic == "" {
			topic = ConstructSSETopic(c)
		}
		m.lc.Debugf("sse: handler subscribing to topic %q", topic)

		b, ch, isNew := m.subscribe(topic, config.PollingService)
		defer m.unsubscribe(topic, ch)

		if isNew && config.PollingService != nil {
			m.lc.Debugf("sse: starting polling service for topic %q", topic)
			b.startPolling()
		}

		return handleSSE(c, m.ctx, b, ch, m.heartbeatInterval)
	}
}

// ConstructSSETopic derives a topic from the request URL — path on its own,
// or path with the query string appended when one is present.
//
// e.g. "/api/v3/device/all/sse?offset=10&labels=label1,label2"
func ConstructSSETopic(c echo.Context) string {
	if c.QueryString() == "" {
		return c.Request().URL.Path
	}
	return c.Request().URL.Path + "?" + c.QueryString()
}

func handleSSE(c echo.Context, serviceCtx context.Context, b *broadcaster, ch subscriberCh, heartbeatInterval time.Duration) error {
	if heartbeatInterval <= 0 {
		b.lc.Debug("sse: heartbeat interval is not set or invalid, using default value: 30s")
		heartbeatInterval = defaultHeartbeatInterval
	}

	if err := WriteSSEHeaders(c, heartbeatInterval, b.lc); err != nil {
		return err
	}

	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case msg, open := <-ch:
			if !open {
				// Channel closed during unsubscribe (e.g. shutdown). End gracefully.
				b.lc.Debug("sse: subscriber channel closed")
				return nil
			}
			if err := WriteSSEEvent(c, msg, heartbeatInterval, b.lc); err != nil {
				b.lc.Errorf("sse: failed to write message: %v", err)
				return nil
			}

		case <-heartbeatTicker.C:
			if err := WriteHeartbeat(c, heartbeatInterval, b.lc); err != nil {
				b.lc.Warnf("sse: heartbeat write failed: %v", err)
				return nil
			}

		case <-c.Request().Context().Done():
			b.lc.Debug("sse: request cancelled or timed out")
			return nil

		case <-serviceCtx.Done():
			b.lc.Info("sse: service shutting down, closing SSE connection")
			return nil
		}
	}
}
