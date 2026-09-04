//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package callstate answers whether the CALLER ended a call, as opposed to the
// MCP service's own tool-call ceiling. Both arrive as context.DeadlineExceeded,
// so ctx.Err() cannot separate them; the ceiling stamps a cause and Abandoned is
// the one place it is compared. What each caller does with the answer, and why
// one place: pkg/mcp/middleware/README.md.
package callstate

import (
	"context"
	"errors"
)

// ErrCeilingExpired is stamped by pkg/mcp/middleware (Deadline), read only here.
var ErrCeilingExpired = errors.New("the tool call exceeded its ceiling")

// Abandoned reports that ctx ended and the tool-call ceiling was not what ended
// it — so the failure is evidence about the caller and about nothing else. An
// unstamped expiry counts: only the ceiling stamps. ⚠ Pass the context the CALL
// owns, never one narrowed inside it, and never nil — every caller gets one from
// net/http or from a handler chain, so a guard here would only hide a wiring bug.
func Abandoned(ctx context.Context) bool {
	cause := context.Cause(ctx)
	return cause != nil && !errors.Is(cause, ErrCeilingExpired)
}
