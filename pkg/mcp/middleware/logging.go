//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package middleware holds the receiving-side MCP middlewares an MCP resource
// server installs: request logging, tools/call authorization (Auth) and
// tools/list visibility filtering (Visibility).
package middleware

import (
	"context"
	"encoding/json"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// methodToolsCall mirrors the MCP SDK's internal `methodCallTool` constant,
// which is unexported in github.com/modelcontextprotocol/go-sdk/mcp.
const methodToolsCall = "tools/call"

// approxCharsPerToken is the heuristic divisor used to estimate the token
// footprint of an MCP response from the JSON byte length. Roughly accurate to
// ±20% for English JSON; Claude tokens are 3–5 chars on average.
const approxCharsPerToken = 4

// Logging returns a pass-through middleware that logs every incoming MCP
// request's method, tool name (when method is tools/call), duration, an
// approximate response-token count, and error.
func Logging(lc log.Logger) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			start := time.Now()
			toolField := toolField(method, req)

			result, err := next(ctx, method, req)

			dur := time.Since(start)
			switch {
			case err != nil:
				lc.Errorf("mcp method=%s%s duration=%s error=%v", method, toolField, dur, err)
			case lc.LogLevel() == log.DebugLog || lc.LogLevel() == log.TraceLog:
				// Gate approxTokens behind the level check: marshalling the
				// result to JSON is wasted work when the debug line won't emit.
				lc.Debugf("mcp method=%s%s duration=%s approx_tokens=%d", method, toolField, dur, approxTokens(result))
			}
			return result, err
		}
	}
}

// toolField returns " tool=<name>" for tools/call requests, or "" otherwise.
// Using %s without surrounding quotes keeps logfmt-encoded output free of
// escaped quotes; MCP tool names are identifiers and do not need quoting.
func toolField(method string, req sdkmcp.Request) string {
	if method != methodToolsCall {
		return ""
	}
	call, ok := req.(*sdkmcp.CallToolRequest)
	if !ok || call == nil || call.Params == nil {
		return ""
	}
	return " tool=" + call.Params.Name
}

// approxTokens estimates the token cost of an MCP result by serializing it to
// JSON (matching what the agent eventually sees) and dividing the byte count by
// approxCharsPerToken. Returns 0 if the result is nil or unserializable.
func approxTokens(result sdkmcp.Result) int {
	if result == nil {
		return 0
	}
	b, err := json.Marshal(result)
	if err != nil {
		return 0
	}
	return len(b) / approxCharsPerToken
}
