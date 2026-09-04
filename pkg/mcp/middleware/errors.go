//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// isErrorResult builds a tools/call result carrying an error message (IsError=true).
func isErrorResult(message string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: message}},
	}
}
