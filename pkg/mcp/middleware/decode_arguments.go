//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// DecodeArguments accepts an array or object argument that a client sent as a
// JSON string. Claude Desktop and the claude.ai connector serialise every
// argument declared as an array or object, leaving scalars alone, and the SDK's
// schema gate rejects those calls before any tool handler runs.
//
// It must stay innermost (installed first, see the middleware chain): it reads
// the advertised schemas back through `next`, which outside Visibility would
// return a per-caller subset and outside Logging would log a request no client
// made.
func DecodeArguments(lc log.Logger) sdkmcp.Middleware {
	var once sync.Once
	var schemas map[string]*jsonschema.Schema

	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			// A malformed tools/call passes through rather than being rejected
			// here, unlike Auth and Visibility: the gate owns argument shape.
			call, ok := req.(*sdkmcp.CallToolRequest)
			// ok is true even for a nil *CallToolRequest inside the interface.
			if !ok || method != methodToolsCall || call == nil || call.Params == nil {
				return next(ctx, method, req)
			}

			once.Do(func() { schemas = readSchemas(ctx, next, call, lc) })

			// A failed readback leaves schemas nil, and a nil map indexes to nil.
			schema := schemas[call.Params.Name]
			if schema == nil {
				return next(ctx, method, req)
			}
			if touched := decode(schema, call.Params); len(touched) > 0 &&
				(lc.LogLevel() == log.DebugLog || lc.LogLevel() == log.TraceLog) {
				// Property names only: argument values carry customer data.
				lc.Debugf("mcp decoded stringified arguments tool=%s properties=%v", call.Params.Name, touched)
			}
			return next(ctx, method, req)
		}
	}
}

// readSchemas returns the server's live schema pointers, read and never mutated.
// A nil return disables decoding, and every way that happens is permanent, so it
// is read once — only safe because of the WithoutCancel below.
//
// One request, no cursor: DefaultPageSize is 1000 against a typical surface and
// the server sets no PageSize. A surface that outgrows a page fails loudly instead
// of leaving a partial map.
func readSchemas(
	ctx context.Context,
	next sdkmcp.MethodHandler,
	call *sdkmcp.CallToolRequest,
	lc log.Logger,
) map[string]*jsonschema.Schema {
	// WithoutCancel, so neither a caller's disconnect nor a tool-call ceiling can
	// latch a nil map for the process's lifetime. ⚠ Both are now reachable — the
	// ceiling nests outside this middleware and PropagateRequestCancellation is on
	// — so do not "simplify" it to a plain ctx. It must also not depend on go-sdk's
	// listTools discarding its ctx: that is theirs to change, and the failure would
	// be silent and permanent.
	res, err := next(context.WithoutCancel(ctx), methodToolsList,
		&sdkmcp.ListToolsRequest{Session: call.Session})
	if err != nil {
		lc.Errorf("mcp decode arguments could not read tool schemas, decoding disabled: %v", err)
		return nil
	}
	list, ok := res.(*sdkmcp.ListToolsResult)
	// ok is true even for a nil *ListToolsResult inside the interface.
	if !ok || list == nil {
		lc.Errorf("mcp decode arguments got %T from tools/list, decoding disabled", res)
		return nil
	}
	if list.NextCursor != "" {
		lc.Errorf("mcp decode arguments: the tool surface outgrew one tools/list page, decoding disabled")
		return nil
	}

	out := make(map[string]*jsonschema.Schema, len(list.Tools))
	for _, tool := range list.Tools {
		if s, ok := tool.InputSchema.(*jsonschema.Schema); ok && s != nil {
			out[tool.Name] = s
		}
	}
	return out
}

// decode rewrites params.Arguments in place — the request pointer is shared down
// the chain and the generated handler re-reads it — and returns what it
// unwrapped. Top level only: the client stringifies the outer value and nothing
// below it.
func decode(schema *jsonschema.Schema, params *sdkmcp.CallToolParamsRaw) []string {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return nil
	}

	var touched []string
	for name, raw := range args {
		// Succeeds only if the value is a JSON string: the stringified test.
		var asString string
		if err := json.Unmarshal(raw, &asString); err != nil {
			continue
		}
		var inner json.RawMessage
		if err := json.Unmarshal([]byte(asString), &inner); err != nil {
			continue // not JSON: leave it for the gate
		}
		if !unwrappable(schema.Properties[name], inner) {
			continue
		}
		args[name] = inner // original bytes; a re-marshal rounds wide integers
		touched = append(touched, name)
	}
	if len(touched) == 0 {
		return nil
	}

	fixed, err := json.Marshal(args)
	if err != nil {
		return nil
	}
	params.Arguments = fixed
	return touched
}

// unwrappable reports whether inner may replace the string it decoded from.
//
// The declared set must permit inner's kind and must not permit a string — a set
// test because our slices are advertised as ["null","array"], and refusing when a
// string is permitted is what protects a genuine string whose value looks like
// JSON. Requiring the kind to match stops "null" passing a ["null","array"] gate
// as an omitted field, and stops an object being rewritten into an array-only
// property. Go `any` is a union including "string", so `any` leaves are excluded.
func unwrappable(prop *jsonschema.Schema, inner json.RawMessage) bool {
	// The len check is what makes inner[0] below safe; decode cannot reach it.
	if prop == nil || len(inner) == 0 {
		return false
	}

	var kind string
	switch inner[0] { // already validated, and leading whitespace is stripped
	case '[':
		kind = "array"
	case '{':
		kind = "object"
	default:
		return false
	}

	types := prop.Types
	if prop.Type != "" {
		types = []string{prop.Type}
	}
	declaresKind, allowsString := false, false
	for _, t := range types {
		switch t {
		case kind:
			declaresKind = true
		case "string":
			allowsString = true
		}
	}
	return declaresKind && !allowsString
}
