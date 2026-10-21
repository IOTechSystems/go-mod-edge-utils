//
// Copyright (C) 2026 IOTech Ltd
//

// Package metatool declares the three meta-tools of a meta-tool surface:
// search_tools, call_read_tool and call_write_tool. A meta-tool server registers
// every tool plus these three; middleware.MetaTools, outermost on that server,
// lists only the meta-tools and the pinned tools and answers every call to the
// meta-tools, rewriting call_read_tool / call_write_tool into a call of the inner
// tool so RBAC, argument validation, logging and the deadline are a direct call's.
package metatool

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/toolsearch"
)

const (
	SearchTools   = "search_tools"
	CallReadTool  = "call_read_tool"
	CallWriteTool = "call_write_tool"
)

// IsMetaTool reports whether name is one of the three meta-tools. They are not
// in the service's tool registry, so the visibility filter must treat them as
// Local.
func IsMetaTool(name string) bool {
	switch name {
	case SearchTools, CallReadTool, CallWriteTool:
		return true
	}
	return false
}

const callReadToolDescription = "Call a read tool (kind read in search_tools) by name with its arguments. " +
	"On an argument error the result carries the tool's inputSchema."

const callWriteToolDescription = "Call a write tool (kind write in search_tools) by name with its arguments; it changes state. " +
	"On an argument error the result carries the tool's inputSchema."

// searchToolsDescription puts domains, one sentence on what the catalogue
// covers, after the first sentence; "" leaves it out.
func searchToolsDescription(domains string) string {
	parts := []string{"Find the tools you can call through call_read_tool and call_write_tool."}
	if d := strings.TrimSpace(domains); d != "" {
		parts = append(parts, d)
	}
	parts = append(parts,
		"Returns at most "+strconv.Itoa(toolsearch.Cap)+" tools with a one-line summary and kind (read or write).",
		"A query that is a tool's exact name, or that matches one tool only, also returns its description and inputSchema.")
	return strings.Join(parts, " ")
}

// searchToolsInput and callToolInput only declare the meta-tools' input schemas;
// the middleware reads the raw arguments itself.
type searchToolsInput struct {
	Query string `json:"query,omitempty" jsonschema:"words to match against tool names and descriptions, or one tool's exact name; omit to list all"`
}

type callToolInput struct {
	Name      string         `json:"name" jsonschema:"the tool to call, as search_tools names it"`
	Arguments map[string]any `json:"arguments,omitempty" jsonschema:"the tool's arguments, as its inputSchema describes them"`
}

// errNotIntercepted is what a meta-tool answers when middleware.MetaTools is not
// installed in front of it. Returned as an error from a raw handler, so the
// caller gets a JSON-RPC error rather than an isError result.
var errNotIntercepted = errors.New("handled by the meta-tool middleware")

func notIntercepted(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return nil, errNotIntercepted
}

// Register adds the three meta-tools to server. domains is one sentence on what
// the catalogue covers, put in search_tools' description; "" leaves it out. Their
// handler is never reached while middleware.MetaTools is installed: it answers
// every call to them.
func Register(server *mcp.Server, domains string) {
	notDestructive, destructive := false, true
	server.AddTool(&mcp.Tool{
		Name:        SearchTools,
		Description: searchToolsDescription(domains),
		InputSchema: schemaFor[searchToolsInput](),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &notDestructive},
	}, notIntercepted)
	server.AddTool(&mcp.Tool{
		Name:        CallReadTool,
		Description: callReadToolDescription,
		InputSchema: schemaFor[callToolInput](),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &notDestructive},
	}, notIntercepted)
	server.AddTool(&mcp.Tool{
		Name:        CallWriteTool,
		Description: callWriteToolDescription,
		InputSchema: schemaFor[callToolInput](),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false, DestructiveHint: &destructive},
	}, notIntercepted)
}

func schemaFor[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	return s
}
