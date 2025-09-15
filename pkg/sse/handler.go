//
// Copyright (C) 2025 IOTech Ltd
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
	// have actually been received.
	setSSEHeaders(c)
	if f, ok := c.Response().Writer.(http.Flusher); ok {
		f.Flush()
	} else {
		// In normal Echo deployments, c.Response().Writer implements http.Flusher,
		// so flushing will work. This check is mainly for tests or custom middlewares
		// that may wrap the ResponseWriter without flushing support.
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
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				b.lc.Errorf("failed to serialize message: %v", err)
				continue
			}

			// Set a write deadline to avoid blocking indefinitely when writing
			// to a slow or broken connection.
			if err := rc.SetWriteDeadline(time.Now().Add(heartbeatInterval)); err != nil {
				b.lc.Errorf("sse: failed to set write deadline or not supported: %v", err)
				return nil
			}

			_, err = fmt.Fprintf(c.Response().Writer, "data: %s\n\n", msgJSON)
			if err != nil {
				// If writing fails, log the error and close the connection.
				b.lc.Errorf("failed to write message: %v", err)
				return nil
			}

			c.Response().Flush()

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

func setSSEHeaders(c echo.Context) {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
}
