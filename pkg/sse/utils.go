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

// WriteSSEHeaders sets the three SSE response headers on c and flushes
// them immediately so the client sees the text/event-stream content type
// before any payload arrives. It also runs a one-shot probe verifying
// that the underlying writer chain exposes SetWriteDeadline; a failure
// is logged loudly so a misconfigured middleware/wrapper surfaces at
// stream start rather than as a confusing first-event write error.
func WriteSSEHeaders(c echo.Context, lc log.Logger) {
	h := c.Response().Header()
	h.Set(echo.HeaderContentType, "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	flush(c, lc)
	probeWriteDeadline(c, lc)
}

// probeWriteDeadline runs once per stream — right after the SSE headers
// flush — to surface writer-chain misconfigurations early. Passing the
// zero time clears any deadline, so a successful probe leaves no
// lingering side effect; a failure means the per-write deadlines in
// WriteSSEEvent / WriteHeartbeat will also fail (and terminate the
// stream), so logging here gives ops the diagnostic earlier than the
// first event/heartbeat would.
func probeWriteDeadline(c echo.Context, lc log.Logger) {
	rc := http.NewResponseController(c.Response().Writer)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		lc.Errorf("sse: writer chain does not support SetWriteDeadline — slow-client protection unavailable; check middleware (err=%v)", err)
	}
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
// underlying connection. Any failure — including http.ErrNotSupported —
// is treated as fatal so the SSE handler exits instead of writing
// without slow-client protection. ErrNotSupported indicates the writer
// (or its wrappers) doesn't expose deadline-setting; the right place to
// fix that is the writer/middleware, not here.
func applyWriteDeadline(c echo.Context, d time.Duration, lc log.Logger) error {
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
