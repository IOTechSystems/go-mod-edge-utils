//
// Copyright (C) 2025 IOTech Ltd
//

package ws

import (
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

// ConnectionContext holds a connection and its metadata
type ConnectionContext struct {
	Conn   WebSocketConn
	Topics map[string]struct{} // Set of topics this connection is subscribed to
}

// WebSocketManager manages WebSocket connections and subscriptions
type WebSocketManager struct {
	// Single map of connections with their metadata
	connections map[string]*ConnectionContext
	connMu      sync.RWMutex

	// Topic index for efficient topic-based operations
	topics  map[string]map[string]struct{} // topic -> map of connection IDs
	topicMu sync.RWMutex

	lc log.Logger
}

// NewWebSocketManager creates a new WebSocketManager
func NewWebSocketManager(lc log.Logger) *WebSocketManager {
	return &WebSocketManager{
		connections: make(map[string]*ConnectionContext),
		topics:      make(map[string]map[string]struct{}),
		lc:          lc,
	}
}

// AddConnection adds a WebSocket connection to the manager
func (w *WebSocketManager) AddConnection(conn WebSocketConn) {
	w.connMu.Lock()
	defer w.connMu.Unlock()

	connID := conn.ID()
	w.connections[connID] = &ConnectionContext{
		Conn:   conn,
		Topics: make(map[string]struct{}),
	}

	w.lc.Debugf("Added connection: %s", connID)
}

// RemoveConnection closes the WebSocket connection, unsubscribes it from all topics, and removes it from the manager
func (w *WebSocketManager) RemoveConnection(conn WebSocketConn) {
	connID := conn.ID()

	// First get the connection context
	w.connMu.RLock()
	connCtx, exists := w.connections[connID]
	w.connMu.RUnlock()

	if !exists {
		w.lc.Warnf("Attempted to remove non-existent connection: %s", connID)
		return
	}

	// Unsubscribe from all topics
	w.topicMu.Lock()
	for topic := range connCtx.Topics {
		// Remove connection ID from topic index
		if conns, exists := w.topics[topic]; exists {
			delete(conns, connID)

			// Remove topic if no more connections
			if len(conns) == 0 {
				delete(w.topics, topic)
			}
		}
	}
	w.topicMu.Unlock()

	// Close the connection
	if err := conn.Close(); err != nil {
		w.lc.Errorf("Error closing connection %s: %v", connID, err)
	}

	// Remove the connection from the map
	w.connMu.Lock()
	delete(w.connections, connID)
	w.connMu.Unlock()

	w.lc.Debugf("Removed connection: %s", connID)
}

// GetConnectionCtxByID returns the ConnectionContext for the specified connection ID
func (w *WebSocketManager) GetConnectionCtxByID(id string) (*ConnectionContext, bool) {
	w.connMu.RLock()
	defer w.connMu.RUnlock()

	connCtx, exists := w.connections[id]
	return connCtx, exists
}

// Subscribe adds a WebSocket connection to the specified topic
func (w *WebSocketManager) Subscribe(conn WebSocketConn, topics []string) errors.Error {
	connID := conn.ID()

	// Get the connection context
	w.connMu.RLock()
	connCtx, exists := w.connections[connID]
	w.connMu.RUnlock()

	if !exists {
		return errors.NewBaseError(errors.KindServerError, "connection not found", nil, nil)
	}

	// Subscribe to each topic
	for _, topic := range topics {
		w.subscribe(connCtx, connID, topic)
	}

	return nil
}

func (w *WebSocketManager) subscribe(connCtx *ConnectionContext, connID, topic string) {
	// Update topic index
	w.topicMu.Lock()
	if _, exists := w.topics[topic]; !exists {
		w.topics[topic] = make(map[string]struct{})
	}
	w.topics[topic][connID] = struct{}{}
	w.topicMu.Unlock()

	// Update connection's topics
	w.connMu.Lock()
	connCtx.Topics[topic] = struct{}{}
	w.connMu.Unlock()

	w.lc.Debugf("Connection %s subscribed to topic: %s", connID, topic)
}

// UnsubscribeAll unsubscribes a connection from all topics
func (w *WebSocketManager) UnsubscribeAll(conn WebSocketConn) {
	connID := conn.ID()

	// Get the connection context
	w.connMu.RLock()
	ctx, exists := w.connections[connID]
	w.connMu.RUnlock()

	if !exists {
		w.lc.Warnf("Attempted to unsubscribe non-existent connection: %s", connID)
		return
	}

	// Get topics to unsubscribe from
	w.connMu.RLock()
	topics := make([]string, 0, len(ctx.Topics))
	for topic := range ctx.Topics {
		topics = append(topics, topic)
	}
	w.connMu.RUnlock()

	// Unsubscribe from each topic
	for _, topic := range topics {
		w.Unsubscribe(conn, topic)
	}
}

// Unsubscribe unsubscribes a connection from a specific topic
func (w *WebSocketManager) Unsubscribe(conn WebSocketConn, topic string) {
	connID := conn.ID()

	// Update topic index
	w.topicMu.Lock()
	if connIDs, exists := w.topics[topic]; exists {
		delete(connIDs, connID)

		// Remove topic if no more connections
		if len(connIDs) == 0 {
			delete(w.topics, topic)
		}
	}
	w.topicMu.Unlock()

	// Update connection's topics
	w.connMu.Lock()
	if ctx, exists := w.connections[connID]; exists {
		delete(ctx.Topics, topic)
	}
	w.connMu.Unlock()

	w.lc.Debugf("Connection %s unsubscribed from topic: %s", connID, topic)
}

// IsTopicSubscribed checks if any connections are subscribed to the specified topic
func (w *WebSocketManager) IsTopicSubscribed(topic string) bool {
	w.topicMu.RLock()
	connIDs, exists := w.topics[topic]
	w.topicMu.RUnlock()

	return exists && len(connIDs) > 0
}

// Send sends a message to the WebSocket connection
func (w *WebSocketManager) Send(conn WebSocketConn, msg string) errors.Error {
	return conn.Send(msg)
}

// SendJSON sends a JSON message to the WebSocket connection
func (w *WebSocketManager) SendJSON(conn WebSocketConn, v any) errors.Error {
	return conn.SendJSON(v)
}

// Receive receives a message from the WebSocket connection
func (w *WebSocketManager) Receive(conn WebSocketConn) (string, errors.Error) {
	return conn.Receive()
}

// ReceiveJSON receives a JSON message from the WebSocket connection
func (w *WebSocketManager) ReceiveJSON(conn WebSocketConn) (any, errors.Error) {
	return conn.ReceiveJSON()
}

// Broadcast sends a message to all connections subscribed to the specified topic
func (w *WebSocketManager) Broadcast(topic string, msg string) {
	// Get connections subscribed to the topic
	w.topicMu.RLock()
	connIDs, exists := w.topics[topic]
	w.topicMu.RUnlock()

	if !exists || len(connIDs) == 0 {
		w.lc.Warnf("No connections subscribed to topic: %s", topic)
		return
	}

	// Send message to each connection
	w.connMu.RLock()
	for connID := range connIDs {
		if ctx, exists := w.connections[connID]; exists {
			if err := ctx.Conn.Send(msg); err != nil {
				w.lc.Errorf("Error sending message to connection %s: %v", connID, err)
			}
		}
	}
	w.connMu.RUnlock()
}

// BroadcastJSON sends a JSON message to all connections subscribed to the specified topic
func (w *WebSocketManager) BroadcastJSON(topic string, v any) {
	// Get connections subscribed to the topic
	w.topicMu.RLock()
	connIDs, exists := w.topics[topic]
	w.topicMu.RUnlock()

	if !exists || len(connIDs) == 0 {
		w.lc.Warnf("No connections subscribed to topic: %s", topic)
		return
	}

	// Send message to each connection
	w.connMu.RLock()
	for connID := range connIDs {
		if ctx, exists := w.connections[connID]; exists {
			if err := ctx.Conn.SendJSON(v); err != nil {
				w.lc.Errorf("Error sending JSON message to connection %s: %v", connID, err)
			}
		}
	}
	w.connMu.RUnlock()
}

// ShutDown closes all WebSocket connections and cleans up resources
func (w *WebSocketManager) ShutDown() {
	// Get all connections
	w.connMu.Lock()
	connections := make([]WebSocketConn, 0, len(w.connections))
	for _, connCtx := range w.connections {
		connections = append(connections, connCtx.Conn)
	}

	// Clear the maps
	w.connections = make(map[string]*ConnectionContext)
	w.connMu.Unlock()

	w.topicMu.Lock()
	w.topics = make(map[string]map[string]struct{})
	w.topicMu.Unlock()

	// Close all connections
	for _, conn := range connections {
		if err := conn.Close(); err != nil {
			w.lc.Warnf("Error closing connection %s: %v", conn.ID(), err)
		}
	}

	w.lc.Info("WebSocket manager shut down")
}
