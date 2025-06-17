//
// Copyright (C) 2025 IOTech Ltd
//

package sse

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler creates an SSE handler that listens for messages on a specific topic and sends the data to the client.
// It can be configured with options such as a PollingService to periodically fetch data and publish it to subscribers.
func Handler(m *Manager, opts ...HandlerOption) echo.HandlerFunc {
	// Apply options to the HandlerConfig if provided
	config := &HandlerConfig{}
	for _, opt := range opts {
		opt(config)
	}

	return func(c echo.Context) error {
		topic := ConstructSSETopic(c)
		m.lc.Debugf("sse: Creating SSE handler for topic '%s'", topic)

		b, isNew := m.CreateOrGetBroadcaster(topic)
		// Only set the PollingService if it is provided in the configuration and the broadcaster is new.
		// Otherwise, the handler will just listen for messages without polling.
		// That is, the user should publish messages through the broadcaster manually.
		if config.PollingService != nil && isNew {
			m.lc.Debugf("sse: Setting up polling service for topic '%s'", topic)
			b.SetPollingService(config.PollingService)
			b.StartPolling()
		}

		return HandleSSE(c, b)
	}
}

// ConstructSSETopic constructs a unique topic string based on the request context.
//
// e.g. "/api/v3/device/all/sse?offset=10&labels=label1,label2"
func ConstructSSETopic(c echo.Context) string {
	if c.QueryString() == "" {
		return c.Path()
	}
	return c.Path() + "?" + c.QueryString()
}

// HandleSSE accepts an echo.Context and a Broadcaster (created by users), provides a more flexible way to handle Server-Sent Events (SSE) compared to the Handler function.
// e.g., The users want to define their own SSE topics and use the broadcaster to publish messages to subscribers manually.
func HandleSSE(c echo.Context, b *Broadcaster) error {
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	setSSEHeaders(c)

	if f, ok := c.Response().Writer.(http.Flusher); ok {
		f.Flush()
	}

	for {
		select {
		case msg := <-ch:
			msgJSON, err := json.Marshal(msg)
			if err != nil {
				b.lc.Errorf("failed to serialize message: %v", err)
				continue
			}
			_, err = fmt.Fprintf(c.Response().Writer, "data: %s\n\n", msgJSON)
			if err != nil {
				b.lc.Errorf("failed to write message: %v", err)
				return err
			}
			c.Response().Flush()
		case <-c.Request().Context().Done():
			b.lc.Debugf("sse: Request cancelled or timed out")
			return nil
		}
	}
}

func setSSEHeaders(c echo.Context) {
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
}
