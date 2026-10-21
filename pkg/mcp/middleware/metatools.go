//
// Copyright (C) 2026 IOTech Ltd
//

package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/metatool"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/tool"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/toolsearch"
)

// MetaTools serves the meta-tool surface: it answers search_tools and rewrites
// call_read_tool / call_write_tool into a call of the inner tool, so every
// middleware inside sees a direct call. behaviourOf reports a tool's Behaviour,
// false for an unknown name. ⚠ Install it last, so it nests outermost.
func MetaTools(lc log.Logger, pinned []string, behaviourOf func(name string) (tool.Behaviour, bool)) sdkmcp.Middleware {
	listed := map[string]bool{
		metatool.SearchTools:   true,
		metatool.CallReadTool:  true,
		metatool.CallWriteTool: true,
	}
	// pinned are the tools tools/list shows beside the meta-tools, so a client
	// can call them directly without search_tools.
	for _, name := range pinned {
		listed[name] = true
	}

	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			switch method {
			case methodToolsList:
				return listMetaTools(ctx, next, method, req, listed)
			case methodToolsCall:
				call, ok := req.(*sdkmcp.CallToolRequest)
				if !ok || call.Params == nil || !metatool.IsMetaTool(call.Params.Name) {
					break
				}
				m := metaToolCall{lc: lc, next: next, call: call, behaviourOf: behaviourOf}
				if call.Params.Name == metatool.SearchTools {
					return m.search(ctx)
				}
				return m.forward(ctx, call.Params.Name == metatool.CallWriteTool)
			}
			return next(ctx, method, req)
		}
	}
}

// listMetaTools trims the tools/list result to the listed tools, keeping its order.
func listMetaTools(ctx context.Context, next sdkmcp.MethodHandler, method string, req sdkmcp.Request, listed map[string]bool) (sdkmcp.Result, error) {
	res, err := next(ctx, method, req)
	if err != nil {
		return res, err
	}
	list, ok := res.(*sdkmcp.ListToolsResult)
	if !ok || list == nil {
		return res, err
	}
	kept := make([]*sdkmcp.Tool, 0, len(listed))
	for _, t := range list.Tools {
		if listed[t.Name] {
			kept = append(kept, t)
		}
	}
	list.Tools = kept
	return list, nil
}

// metaToolCall handles one tools/call of a meta-tool.
type metaToolCall struct {
	lc          log.Logger
	next        sdkmcp.MethodHandler
	call        *sdkmcp.CallToolRequest
	behaviourOf func(name string) (tool.Behaviour, bool)
}

// log writes one debug line per meta-tool call. metaTool is passed in because
// forward rewrites Params.Name to the inner tool.
func (m metaToolCall) log(metaTool, inner, outcome string) {
	if inner == "" {
		inner = "-"
	}
	m.lc.Debugf("mcp metatool=%s tool=%s outcome=%s", metaTool, inner, outcome)
}

// catalogue is the caller-visible full surface, read through next so the
// visibility filter inside applies, page by page whatever PageSize the server
// sets. Extra must be the outer call's: visibility rejects a list request that
// carries no bearer.
func (m metaToolCall) catalogue(ctx context.Context) ([]*sdkmcp.Tool, error) {
	var out []*sdkmcp.Tool
	cursor := ""
	for {
		res, err := m.next(ctx, methodToolsList, &sdkmcp.ListToolsRequest{
			Session: m.call.Session, Params: &sdkmcp.ListToolsParams{Cursor: cursor}, Extra: m.call.Extra,
		})
		if err != nil {
			return nil, err
		}
		list, ok := res.(*sdkmcp.ListToolsResult)
		if !ok || list == nil {
			return nil, fmt.Errorf("metatool: tools/list returned %T", res)
		}
		for _, t := range list.Tools {
			if !metatool.IsMetaTool(t.Name) {
				out = append(out, t)
			}
		}
		if list.NextCursor == "" {
			return out, nil
		}
		cursor = list.NextCursor
	}
}

func (m metaToolCall) search(ctx context.Context) (sdkmcp.Result, error) {
	var in struct {
		Query *string `json:"query"`
	}
	if !decodeObject(m.call.Params.Arguments, &in) {
		m.log(metatool.SearchTools, "", "rejected")
		return errorResult("search_tools needs {query?: string}"), nil
	}
	// Logged before the catalogue read, so a failed read still leaves its line.
	m.log(metatool.SearchTools, "", "answered")
	query := ""
	if in.Query != nil {
		query = *in.Query
	}

	tools, err := m.catalogue(ctx)
	if err != nil {
		return nil, err
	}
	out := toolsearch.Search(tools, query, m.kindOf)
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &sdkmcp.CallToolResult{
		StructuredContent: json.RawMessage(b),
		Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: string(b)}},
	}, nil
}

func (m metaToolCall) forward(ctx context.Context, write bool) (sdkmcp.Result, error) {
	metaTool := m.call.Params.Name
	name, args, ok := callInput(m.call.Params.Arguments)
	if !ok {
		m.log(metaTool, "", "rejected")
		return errorResult(metaTool + " needs {name: string, arguments?: object}"), nil
	}
	if msg := m.rejection(name, write); msg != "" {
		m.log(metaTool, name, "rejected")
		return errorResult(msg), nil
	}

	p := *m.call.Params
	p.Name = name
	p.Arguments = args
	m.call.Params = &p
	m.log(metaTool, name, "forwarded")

	res, err := m.next(ctx, methodToolsCall, m.call)
	if err != nil {
		return res, err
	}
	if result, ok := res.(*sdkmcp.CallToolResult); ok && result != nil && result.IsError && startsWithValidationError(result) {
		if schema := m.schemaOf(ctx, name); schema != "" {
			result.Content = append(result.Content, &sdkmcp.TextContent{Text: "inputSchema of " + name + ": " + schema})
		}
	}
	return res, nil
}

// callInput reads {name, arguments?} from a meta-tool call. arguments may also
// arrive as a JSON string holding an object, as models sometimes send it.
func callInput(raw json.RawMessage) (name string, args json.RawMessage, ok bool) {
	var in struct {
		Name      *string         `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if !decodeObject(raw, &in) || in.Name == nil || *in.Name == "" {
		return "", nil, false
	}

	args = in.Arguments
	if isAbsent(args) {
		return *in.Name, json.RawMessage(`{}`), true
	}
	var s string
	if json.Unmarshal(args, &s) == nil {
		args = json.RawMessage(s)
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(args, &obj) != nil || obj == nil {
		return "", nil, false
	}
	return *in.Name, args, true
}

// decodeObject unmarshals raw into v when raw is absent or a JSON object; a
// field of the wrong type fails it.
func decodeObject(raw json.RawMessage, v any) bool {
	if isAbsent(raw) {
		return true
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return false
	}
	return json.Unmarshal(raw, v) == nil
}

func isAbsent(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s == "" || s == "null"
}

// schemaOf is name's inputSchema as this caller lists it, or "" if it cannot be
// had; the validation error is returned either way.
func (m metaToolCall) schemaOf(ctx context.Context, name string) string {
	tools, err := m.catalogue(ctx)
	if err != nil {
		return ""
	}
	for _, t := range tools {
		if t.Name == name {
			b, err := json.Marshal(t.InputSchema)
			if err != nil {
				return ""
			}
			return string(b)
		}
	}
	return ""
}

// rejection is the reason a meta-tool refuses name before any inner call, or "".
func (m metaToolCall) rejection(name string, write bool) string {
	if metatool.IsMetaTool(name) {
		return fmt.Sprintf("%q is a meta-tool; call it directly", name)
	}
	b, ok := m.behaviourOf(name)
	if !ok {
		return fmt.Sprintf("unknown tool %q; call search_tools to see the tools you can use", name)
	}
	readOnly := b == tool.ReadOnly
	if !write && !readOnly {
		return fmt.Sprintf("%q changes state; call it with call_write_tool", name)
	}
	if write && readOnly {
		return fmt.Sprintf("%q only reads; call it with call_read_tool", name)
	}
	return ""
}

// kindOf is "read" only for a known ReadOnly tool: an unknown tool is never
// advertised as safe.
func (m metaToolCall) kindOf(name string) string {
	if b, ok := m.behaviourOf(name); ok && b == tool.ReadOnly {
		return "read"
	}
	return "write"
}

func startsWithValidationError(res *sdkmcp.CallToolResult) bool {
	if len(res.Content) == 0 {
		return false
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	return ok && strings.HasPrefix(tc.Text, `validating "arguments"`)
}

func errorResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{IsError: true, Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}}}
}
