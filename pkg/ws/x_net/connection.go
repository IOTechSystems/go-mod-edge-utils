//
// Copyright (C) 2025 IOTech Ltd
//

package x_net

import (
	"github.com/google/uuid"
	"golang.org/x/net/websocket"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
)

// XNetWebSocketConn implements WebSocketConn interface
type XNetWebSocketConn struct {
	conn *websocket.Conn
	id   string
}

// NewXNetWebSocketConn creates a new XNetWebSocketConn instance with the specified websocket connection and ID
func NewXNetWebSocketConn(conn *websocket.Conn, id string) *XNetWebSocketConn {
	if len(id) == 0 {
		id = uuid.NewString()
	}
	return &XNetWebSocketConn{
		conn: conn,
		id:   id,
	}
}

func (w *XNetWebSocketConn) ID() string {
	return w.id
}

func (w *XNetWebSocketConn) Send(msg string) errors.Error {
	if err := websocket.Message.Send(w.conn, msg); err != nil {
		return errors.NewBaseError(errors.KindServerError, "failed to send message", err, nil)
	}
	return nil
}

func (w *XNetWebSocketConn) SendJSON(v any) errors.Error {
	if err := websocket.JSON.Send(w.conn, v); err != nil {
		return errors.NewBaseError(errors.KindServerError, "failed to send JSON message", err, nil)
	}
	return nil
}

func (w *XNetWebSocketConn) Receive() (string, errors.Error) {
	var msg string
	if err := websocket.Message.Receive(w.conn, &msg); err != nil {
		return "", errors.NewBaseError(errors.KindServerError, "failed to receive message", err, nil)
	}
	return msg, nil
}

func (w *XNetWebSocketConn) ReceiveJSON() (any, errors.Error) {
	var v any
	if err := websocket.JSON.Receive(w.conn, &v); err != nil {
		return "", errors.NewBaseError(errors.KindServerError, "failed to receive JSON message", err, nil)
	}
	return v, nil
}

func (w *XNetWebSocketConn) Close() errors.Error {
	if err := w.conn.Close(); err != nil {
		return errors.NewBaseError(errors.KindServerError, "failed to close websocket connection", err, nil)
	}
	return nil
}
