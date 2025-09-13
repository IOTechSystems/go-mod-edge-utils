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

	setSSEHeaders(c)

	if f, ok := c.Response().Writer.(http.Flusher); ok {
		f.Flush()
	}

	if heartbeatInterval <= 0 {
		b.lc.Debug("sse: Heartbeat interval is not set or invalid, using default value: 30s")
		heartbeatInterval = defaultHeartbeatInterval
	}
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	// Try to get the ResponseController to set write deadlines if supported by the underlying ResponseWriter
	// This helps to detect broken connections more quickly
	var rc *http.ResponseController
	if ctrl := http.NewResponseController(c.Response().Writer); ctrl != nil {
		rc = ctrl
	}

	for {
		select {
		case msg := <-ch:
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				b.lc.Errorf("failed to serialize message: %v", err)
				continue
			}

			// Set a write deadline to avoid blocking indefinitely on a slow or broken connection
			if rc != nil {
				if err := rc.SetWriteDeadline(time.Now().Add(heartbeatInterval)); err != nil {
					b.lc.Errorf("sse: failed to set write deadline: %v", err)
					return nil
				}
			}

			_, err = fmt.Fprintf(c.Response().Writer, "data: %s\n\n", msgJSON)
			if err != nil {
				// Log the error and exit the loop to clean up the connection
				b.lc.Errorf("failed to write message: %v", err)
				return nil
			}
			c.Response().Flush()
		case <-heartbeatTicker.C:
			// Set a write deadline to avoid blocking indefinitely on a slow or broken connection
			if rc != nil {
				if err := rc.SetWriteDeadline(time.Now().Add(heartbeatInterval)); err != nil {
					b.lc.Errorf("sse: failed to set write deadline for hearbeat messsage: %v", err)
					return nil
				}
			}

			_, err := fmt.Fprintf(c.Response().Writer, ":\n\n")
			if err != nil {
				// Log the error and exit the loop to clean up the connection
				b.lc.Warnf("sse: heartbeat write failed: %v", err)
				return nil
			}
			c.Response().Flush()
		case <-c.Request().Context().Done():
			b.lc.Debug("sse: Request cancelled or timed out")
			return nil
		case <-serviceCtx.Done():
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
