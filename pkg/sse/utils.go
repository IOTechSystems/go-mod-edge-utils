//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// WriteSSEHeaders applies a write deadline, sets the three SSE response
// headers on c, and flushes them immediately so the client sees the
// text/event-stream content type before any payload arrives.
//
// The deadline is applied BEFORE the initial flush so the goroutine
// can't hang on a slow/dead client while writing headers. writeDeadline
// should match the per-event/heartbeat value (typically the heartbeat
// interval). A failure to set the deadline is fatal and returned to the
// caller before any bytes are committed — the caller can then surface a
// proper 5xx rather than committing 200 and dying on the first event.
// ErrNotSupported here means the writer chain (middleware/wrappers)
// doesn't expose SetWriteDeadline; fix the writer, not this code.
func WriteSSEHeaders(c echo.Context, writeDeadline time.Duration, lc log.Logger) error {
	if err := applyWriteDeadline(c, writeDeadline, lc); err != nil {
		return err
	}
	h := c.Response().Header()
	h.Set(echo.HeaderContentType, "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	flush(c, lc)
	return nil
}

// WriteSSEEvent JSON-encodes payload, writes a single SSE data frame, and
// flushes. A marshal failure is logged and the event is skipped without
// killing the stream. The returned error signals an underlying write
// failure (broken pipe, deadline expired, or a missing write-deadline
// capability — see applyWriteDeadline).
//
// writeDeadline bounds the time the underlying connection has to accept
// the write; pass the same value as the heartbeat interval. The caller's
// ResponseWriter MUST support http.ResponseController write deadlines
// (stock Echo + raw net/http does; test fixtures must implement
// SetWriteDeadline as a no-op).
func WriteSSEEvent(c echo.Context, payload any, writeDeadline time.Duration, lc log.Logger) error {
	body, err := json.Marshal(payload)
	if err != nil {
		lc.Errorf("sse: failed to marshal SSE payload: %v", err)
		return nil
	}
	if err := applyWriteDeadline(c, writeDeadline, lc); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Response().Writer, "data: %s\n\n", body); err != nil {
		return err
	}
	flush(c, lc)
	return nil
}

// WriteHeartbeat writes an SSE heartbeat frame (`:\n\n`) and flushes.
// Same writeDeadline semantics as WriteSSEEvent.
func WriteHeartbeat(c echo.Context, writeDeadline time.Duration, lc log.Logger) error {
	if err := applyWriteDeadline(c, writeDeadline, lc); err != nil {
		return err
	}
	if _, err := fmt.Fprint(c.Response().Writer, ":\n\n"); err != nil {
		return err
	}
	flush(c, lc)
	return nil
}

// applyWriteDeadline sets a per-write deadline on the response's
// underlying connection. A non-positive d would resolve to an
// immediate/past deadline and fail every subsequent write, so it is
// defensively replaced with defaultHeartbeatInterval and a Warnf is
// emitted — exported helpers shouldn't surprise callers with timeouts
// from arithmetic. Any SetWriteDeadline failure (including
// http.ErrNotSupported) is treated as fatal so the SSE handler exits
// instead of writing without slow-client protection; ErrNotSupported
// indicates the writer (or its wrappers) doesn't expose
// deadline-setting, which is a middleware fix, not a runtime one.
func applyWriteDeadline(c echo.Context, d time.Duration, lc log.Logger) error {
	if d <= 0 {
		lc.Warnf("sse: non-positive write deadline %v; using default %v", d, defaultHeartbeatInterval)
		d = defaultHeartbeatInterval
	}
	rc := http.NewResponseController(c.Response().Writer)
	if err := rc.SetWriteDeadline(time.Now().Add(d)); err != nil {
		lc.Errorf("sse: failed to set write deadline: %v", err)
		return err
	}
	return nil
}

// flush emits buffered response bytes to the network. Writers that don't
// implement http.Flusher get a warn log; SSE streams effectively stall
// without flushing, but the process continues.
func flush(c echo.Context, lc log.Logger) {
	if f, ok := c.Response().Writer.(http.Flusher); ok {
		f.Flush()
		return
	}
	lc.Warn("sse: ResponseWriter does not support flushing; SSE may stall")
}
