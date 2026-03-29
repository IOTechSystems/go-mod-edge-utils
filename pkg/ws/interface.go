//
// Copyright (C) 2025 IOTech Ltd
//

package ws

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
)

// WebSocketConn abstracts the WebSocket connection operations
type WebSocketConn interface {
	// ID returns the connection ID
	ID() string
	// Close closes the WebSocket connection
	Close() errors.Error

	// Send sends a message to the WebSocket connection
	Send(msg string) errors.Error
	// SendJSON sends a JSON message to the WebSocket connection
	SendJSON(v any) errors.Error
	// Receive receives a message from the WebSocket connection
	Receive() (msg string, err errors.Error)
	// ReceiveJSON receives a JSON message from the WebSocket connection
	ReceiveJSON() (v any, err errors.Error)
}
