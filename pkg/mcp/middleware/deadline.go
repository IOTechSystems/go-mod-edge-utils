//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// deadline.go — the one bound on a whole MCP call. See README.md, "What the
// ceiling bounds".

package middleware

import (
	"context"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/callstate"
)

// ToolCallCeiling bounds the authorization sub-call, the upstream request and its
// body — not any registry/address lookup a service resolves outside the call.
const ToolCallCeiling = 30 * time.Second

// ceilinged is the set of methods that get one. ⚠ A long-lived stream must never
// be in it: a ceiling there cuts the connection, not a call.
var ceilinged = map[string]bool{
	methodToolsCall: true,
	methodToolsList: true,
}

// Deadline bounds every ceilinged method at ceiling, on the caller's own context.
// ceiling is a parameter so a test can pass 50 ms instead of waiting 30 s.
// ⚠ Register it LAST so it nests OUTERMOST — only from there does it bound the
// whole call.
func Deadline(ceiling time.Duration) sdkmcp.Middleware {
	// A non-positive one expires every call from the outermost layer. Loud at boot.
	if ceiling <= 0 {
		panic("middleware: Deadline needs a positive ceiling")
	}
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if !ceilinged[method] {
				return next(ctx, method, req)
			}
			// WithTimeoutCause, not WithTimeout: the cause is the only thing that
			// separates this deadline from one the caller set. The one place it is
			// stamped; callstate.Abandoned is the one place it is read.
			ctx, cancel := context.WithTimeoutCause(ctx, ceiling, callstate.ErrCeilingExpired)
			defer cancel()

			return next(ctx, method, req)
		}
	}
}
