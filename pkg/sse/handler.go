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

// Handler creates an SSE handler that listens for messages on a specific topic and sends the data to the client.
// It can be configured with options such as a PollingService to periodically fetch data and publish it to subscribers.
func Handler(m *Manager, opts ...HandlerOption) echo.HandlerFunc {
	// Apply options to the HandlerConfig if provided
	config := &HandlerConfig{}
	for _, opt := range opts {
		opt(config)
	}

	return func(c echo.Context) error {
		var topic string
		if config.CustomTopic != "" {
			// If a custom topic is provided, use it directly.
			m.lc.Debugf("sse: Creating SSE handler for custom topic '%s'", config.CustomTopic)
			topic = config.CustomTopic
		} else {
			// Construct the topic based on the request context.
			topic = ConstructSSETopic(c)
			m.lc.Debugf("sse: Creating SSE handler for topic '%s'", topic)
		}

		b, isNew := m.CreateOrGetBroadcaster(topic)
		// Only set the PollingService if it is provided in the configuration and the broadcaster is new.
		// Otherwise, the handler will just listen for messages without polling.
		// That is, the user should publish messages through the broadcaster manually.
		if config.PollingService != nil && isNew {
			m.lc.Debugf("sse: Setting up polling service for topic '%s'", topic)
			b.SetPollingService(config.PollingService)
			b.StartPolling()
		}

		return handleSSE(c, m.ctx, b, m.heartbeatInterval)
	}
}

// ConstructSSETopic constructs a unique topic string based on the request context.
//
// e.g. "/api/v3/device/all/sse?offset=10&labels=label1,label2"
func ConstructSSETopic(c echo.Context) string {
	if c.QueryString() == "" {
		return c.Request().URL.Path
	}
	return c.Request().URL.Path + "?" + c.QueryString()
}

func handleSSE(c echo.Context, serviceCtx context.Context, b *Broadcaster, heartbeatInterval time.Duration) error {
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Force any pending HTTP headers (such as "Content-Type: text/event-stream")
	// to be sent immediately so the client knows this is an SSE stream.
	// Without this, some servers or frameworks buffer headers by default,
	// and some clients will not start processing events until the headers
	// have actually been received. SetHeaders flushes internally when the
	// writer is a Flusher; the explicit check below is only to surface a
	// warning when the writer cannot flush, since an SSE stream that buffers
	// headers will misbehave for clients that wait on the content type.
	SetHeaders(c)
	if _, ok := c.Response().Writer.(http.Flusher); !ok {
		b.lc.Warn("sse: ResponseWriter does not support flushing, SSE may not work as expected")
	}

	// Fallback to the default heartbeat interval if it is unset or invalid.
	if heartbeatInterval <= 0 {
		b.lc.Debug("sse: Heartbeat interval is not set or invalid, using default value: 30s")
		heartbeatInterval = defaultHeartbeatInterval
	}
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	// Create an ResponseController so we can set write deadlines manually.
	// Write deadlines are applied to the underlying network connection (net.Conn) used by
	// the ResponseWriter. If sending data to the client takes longer than the deadline,
	// the write will fail, allowing us to detect broken or extremely slow connections sooner.
	rc := http.NewResponseController(c.Response().Writer)

	for {
		select {
		case msg := <-ch:
			if !writeMessage(c, b, rc, msg, heartbeatInterval) {
				return nil
			}

		case <-heartbeatTicker.C:
			// Send a comment line as a heartbeat to keep the connection alive.
			// Also set a write deadline to avoid blocking indefinitely.
			if err := rc.SetWriteDeadline(time.Now().Add(heartbeatInterval)); err != nil {
				b.lc.Errorf("sse: failed to set write deadline or not supported for hearbeat messsage: %v", err)
				return nil
			}

			_, err := fmt.Fprintf(c.Response().Writer, ":\n\n")
			if err != nil {
				// Log the error and exit the loop to clean up the connection
				b.lc.Warnf("sse: heartbeat write failed: %v", err)
				return nil
			}

			c.Response().Flush()

		case <-c.Request().Context().Done():
			// The client cancelled the request or the context timed out.
			b.lc.Debug("sse: Request cancelled or timed out")
			return nil

		case <-serviceCtx.Done():
			// The server is shutting down; close all active SSE connections.
			b.lc.Info("sse: Service shutting down, closing all SSE connection")
			return nil
		}
	}
}

// SetHeaders sets the standard SSE response headers (Content-Type,
// Cache-Control, Connection) on c and flushes them so proxies and clients see
// the SSE content type before the first event byte arrives. Most callers
// should use Handler, which calls this internally; this is exported for
// one-shot SSE responses that do not subscribe to a broadcaster (e.g.
// jobtracker replaying a retained terminal event).
//
// If the underlying ResponseWriter does not implement http.Flusher (typically
// only in tests with a custom wrapper), the flush is silently skipped — the
// headers will still go out implicitly on first write.
func SetHeaders(c echo.Context) {
	h := c.Response().Header()
	h.Set(echo.HeaderContentType, "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	if f, ok := c.Response().Writer.(http.Flusher); ok {
		f.Flush()
	}
}

// writeMessage marshals msg, sets a write deadline, and emits msg as an SSE
// event. Returns false only on connection-level failure (write deadline /
// network write); the caller should return nil in that case. A marshalling
// failure is logged and skipped — a single bad payload should not tear down
// the stream.
func writeMessage(c echo.Context, b *Broadcaster, rc *http.ResponseController, msg any, deadline time.Duration) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		b.lc.Errorf("sse: marshal event payload: %v", err)
		return true
	}
	if err := rc.SetWriteDeadline(time.Now().Add(deadline)); err != nil {
		b.lc.Errorf("sse: set write deadline: %v", err)
		return false
	}
	if _, err := fmt.Fprintf(c.Response().Writer, "data: %s\n\n", data); err != nil {
		b.lc.Errorf("sse: write event: %v", err)
		return false
	}
	c.Response().Flush()
	return true
}

// WriteEvent marshals payload as JSON, writes it as a single SSE "data:" event,
// and flushes the response. Returns an error if marshalling or writing fails.
//
// Headers are NOT set here — call SetHeaders first. SetHeaders flushes
// immediately when the writer supports http.Flusher, so the client sees
// "Content-Type: text/event-stream" before the first event without an
// additional flush call.
func WriteEvent(c echo.Context, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}
	if _, err := fmt.Fprintf(c.Response().Writer, "data: %s\n\n", data); err != nil {
		return fmt.Errorf("write event: %w", err)
	}
	c.Response().Flush()
	return nil
}
