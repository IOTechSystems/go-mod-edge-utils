//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	// Flush the SSE response headers immediately so clients waiting for the
	// "text/event-stream" content type can start processing the stream.
	setSSEHeaders(c)
	if f, ok := c.Response().Writer.(http.Flusher); ok {
		f.Flush()
	} else {
		// Normal Echo deployments support flushing; this branch is mainly for
		// tests or custom middleware that wrap the ResponseWriter.
		b.lc.Warn("sse: ResponseWriter does not support flushing, SSE may not work as expected")
	}

	if heartbeatInterval <= 0 {
		b.lc.Debug("sse: heartbeat interval is not set or invalid, using default value: 30s")
		heartbeatInterval = defaultHeartbeatInterval
	}
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	// ResponseController lets us apply per-write deadlines on the underlying
	// network connection so a slow or broken client surfaces as a write
	// error instead of an indefinite block.
	rc := http.NewResponseController(c.Response().Writer)

	for {
		select {
		case msg, open := <-ch:
			if !open {
				// Channel closed during unsubscribe (e.g. shutdown). End gracefully.
				b.lc.Debug("sse: subscriber channel closed")
				return nil
			}
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				b.lc.Errorf("sse: failed to serialize message: %v", err)
				continue
			}

			if err := rc.SetWriteDeadline(time.Now().Add(heartbeatInterval)); err != nil {
				b.lc.Errorf("sse: failed to set write deadline or not supported: %v", err)
				return nil
			}

			if _, err = fmt.Fprintf(c.Response().Writer, "data: %s\n\n", msgJSON); err != nil {
				b.lc.Errorf("sse: failed to write message: %v", err)
				return nil
			}

			c.Response().Flush()

		case <-heartbeatTicker.C:
			if err := rc.SetWriteDeadline(time.Now().Add(heartbeatInterval)); err != nil {
				b.lc.Errorf("sse: failed to set write deadline or not supported for heartbeat: %v", err)
				return nil
			}

			if _, err := fmt.Fprintf(c.Response().Writer, ":\n\n"); err != nil {
				b.lc.Warnf("sse: heartbeat write failed: %v", err)
				return nil
			}

			c.Response().Flush()

		case <-c.Request().Context().Done():
			b.lc.Debug("sse: request cancelled or timed out")
			return nil

		case <-serviceCtx.Done():
			b.lc.Info("sse: service shutting down, closing SSE connection")
			return nil
		}
	}
}

func setSSEHeaders(c echo.Context) {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
}
