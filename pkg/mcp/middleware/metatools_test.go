//
// Copyright (C) 2026 IOTech Ltd
//

package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/metatool"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/tool"
)

const metaToolBearer = "Bearer t1"

type readInput struct {
	Limit int `json:"limit,omitempty"`
}

type hiddenInput struct {
	Q string `json:"q"`
}

type writeInput struct {
	Name  string   `json:"name"`
	Items []string `json:"items,omitempty"`
}

type addInput struct {
	Name string `json:"name"`
}

// probeSeen is one request the probe saw, innermost on the server.
type probeSeen struct {
	method string
	tool   string
	auth   string
}

// probe records every request that reaches the bottom of the chain and can be
// set to fail tools/list or tools/call with a JSON-RPC error.
type probe struct {
	mu       sync.Mutex
	seen     []probeSeen
	failList *jsonrpc.Error
	failCall *jsonrpc.Error
	// failLaterPage fails only a tools/list that asks for a page after the first.
	failLaterPage *jsonrpc.Error
}

func (p *probe) middleware(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
	return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
		s := probeSeen{method: method}
		if extra := req.GetExtra(); extra != nil && extra.Header != nil {
			s.auth = extra.Header.Get("Authorization")
		}
		if call, ok := req.(*sdkmcp.CallToolRequest); ok && call.Params != nil {
			s.tool = call.Params.Name
		}
		p.mu.Lock()
		p.seen = append(p.seen, s)
		failList, failCall, failLaterPage := p.failList, p.failCall, p.failLaterPage
		p.mu.Unlock()

		switch {
		case method == methodToolsList && failList != nil:
			return nil, failList
		case method == methodToolsList && failLaterPage != nil && laterPage(req):
			return nil, failLaterPage
		case method == methodToolsCall && failCall != nil:
			return nil, failCall
		}
		return next(ctx, method, req)
	}
}

func laterPage(req sdkmcp.Request) bool {
	list, ok := req.(*sdkmcp.ListToolsRequest)
	return ok && list.Params != nil && list.Params.Cursor != ""
}

func (p *probe) setLaterPage(err *jsonrpc.Error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failLaterPage = err
}

func (p *probe) set(failList, failCall *jsonrpc.Error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failList, p.failCall = failList, failCall
}

func (p *probe) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = nil
}

func (p *probe) requests() []probeSeen {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]probeSeen(nil), p.seen...)
}

type metaToolOpts struct {
	PageSize int
	// behaviours replaces the default behaviourOf map when set.
	behaviours map[string]tool.Behaviour
}

type metaToolFixture struct {
	cs    *sdkmcp.ClientSession
	probe *probe
	lc    *recordingLogger
}

// echo is a handler that answers with its tool name and the arguments it got.
func echo[In any](name string) sdkmcp.ToolHandlerFor[In, any] {
	return func(_ context.Context, _ *sdkmcp.CallToolRequest, in In) (*sdkmcp.CallToolResult, any, error) {
		return nil, map[string]any{"tool": name, "args": in}, nil
	}
}

// bearerTransport stamps the caller's bearer on every request, as a client does.
type bearerTransport struct{}

func (bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", metaToolBearer)
	return http.DefaultTransport.RoundTrip(req)
}

// metaToolServer serves a meta-tool server over stateless streamable HTTP, so
// every request carries Extra.Header as in production. Middleware, inner to
// outer: probe, DecodeArguments, Logging, MetaTools.
func metaToolServer(t *testing.T, opts metaToolOpts) *metaToolFixture {
	t.Helper()

	behaviours := opts.behaviours
	if behaviours == nil {
		behaviours = map[string]tool.Behaviour{
			"read_tool":   tool.ReadOnly,
			"hidden_read": tool.ReadOnly,
			"write_tool":  tool.Destructive,
			"add_tool":    tool.Additive,
		}
	}
	behaviourOf := func(name string) (tool.Behaviour, bool) {
		b, ok := behaviours[name]
		return b, ok
	}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "metatool-probe", Version: "0"},
		&sdkmcp.ServerOptions{PageSize: opts.PageSize})
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "read_tool", Description: "Read things."}, echo[readInput]("read_tool"))
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "hidden_read", Description: "Read hidden things."}, echo[hiddenInput]("hidden_read"))
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "write_tool", Description: "Write things."}, echo[writeInput]("write_tool"))
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "add_tool", Description: "Add a thing."}, echo[addInput]("add_tool"))
	metatool.Register(server, "Covers test tools.")

	p := &probe{}
	lc := &recordingLogger{}
	// One call per middleware, each wrapping the last: MetaTools ends outermost.
	server.AddReceivingMiddleware(p.middleware)
	server.AddReceivingMiddleware(DecodeArguments(lc))
	server.AddReceivingMiddleware(Logging(lc))
	server.AddReceivingMiddleware(MetaTools(lc, []string{"read_tool"}, behaviourOf))

	httpServer := httptest.NewServer(sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return server },
		&sdkmcp.StreamableHTTPOptions{JSONResponse: true, Stateless: true}))
	t.Cleanup(httpServer.Close)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "metatool-probe-client", Version: "0"}, nil)
	cs, err := client.Connect(context.Background(), &sdkmcp.StreamableClientTransport{
		Endpoint:   httpServer.URL,
		HTTPClient: &http.Client{Transport: bearerTransport{}},
	}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cs.Close() })

	return &metaToolFixture{cs: cs, probe: p, lc: lc}
}

func (f *metaToolFixture) call(t *testing.T, name string, args any) *sdkmcp.CallToolResult {
	t.Helper()
	res, err := f.callErr(name, args)
	require.NoError(t, err)
	return res
}

func (f *metaToolFixture) callErr(name string, args any) (*sdkmcp.CallToolResult, error) {
	return f.cs.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: name, Arguments: args})
}

// texts is every TextContent of res, in order.
func texts(res *sdkmcp.CallToolResult) []string {
	var out []string
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			out = append(out, tc.Text)
		}
	}
	return out
}

// structured decodes res.StructuredContent into a generic map.
func structured(t *testing.T, res *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	b, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

func requireToolError(t *testing.T, res *sdkmcp.CallToolResult, text string) {
	t.Helper()
	require.True(t, res.IsError, "want isError, got %v", texts(res))
	assert.Equal(t, []string{text}, texts(res))
}

func (f *metaToolFixture) toolCalls() []string {
	var out []string
	for _, s := range f.probe.requests() {
		if s.method == methodToolsCall {
			out = append(out, s.tool)
		}
	}
	return out
}

func TestMetaTools_ListKeepsMetaToolsAndPinned(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	res, err := f.cs.ListTools(context.Background(), nil)
	require.NoError(t, err)

	var got []string
	for _, tl := range res.Tools {
		got = append(got, tl.Name)
	}
	assert.Equal(t, []string{"call_read_tool", "call_write_tool", "read_tool", "search_tools"}, got)
}

func TestMetaTools_RewriteReachesInnerTool(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	res := f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool", "arguments": map[string]any{"limit": 2}})

	require.False(t, res.IsError, texts(res))
	assert.Equal(t, map[string]any{"tool": "read_tool", "args": map[string]any{"limit": float64(2)}}, structured(t, res))
	assert.Equal(t, []string{"read_tool"}, f.toolCalls())
}

func TestMetaTools_KindChecks(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	requireToolError(t, f.call(t, metatool.CallReadTool, map[string]any{"name": "write_tool"}),
		`"write_tool" changes state; call it with call_write_tool`)
	requireToolError(t, f.call(t, metatool.CallWriteTool, map[string]any{"name": "read_tool"}),
		`"read_tool" only reads; call it with call_read_tool`)
	assert.Empty(t, f.toolCalls(), "a rejected call must not reach the inner chain")

	res := f.call(t, metatool.CallWriteTool, map[string]any{"name": "add_tool", "arguments": map[string]any{"name": "a"}})
	require.False(t, res.IsError, texts(res))
	assert.Equal(t, map[string]any{"tool": "add_tool", "args": map[string]any{"name": "a"}}, structured(t, res))
}

func TestMetaTools_UnknownAndMetaToolNames(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	requireToolError(t, f.call(t, metatool.CallReadTool, map[string]any{"name": "nope"}),
		`unknown tool "nope"; call search_tools to see the tools you can use`)
	requireToolError(t, f.call(t, metatool.CallReadTool, map[string]any{"name": "search_tools"}),
		`"search_tools" is a meta-tool; call it directly`)
}

func TestMetaTools_BadInput(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	requireToolError(t, f.call(t, metatool.CallReadTool, map[string]any{}),
		"call_read_tool needs {name: string, arguments?: object}")
	requireToolError(t, f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool", "arguments": 5}),
		"call_read_tool needs {name: string, arguments?: object}")
	requireToolError(t, f.call(t, metatool.SearchTools, map[string]any{"query": 5}),
		"search_tools needs {query?: string}")
}

func TestMetaTools_StringifiedAndNullArguments(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	stringified := f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool", "arguments": `{"limit":1}`})
	object := f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool", "arguments": map[string]any{"limit": 1}})
	require.False(t, stringified.IsError, texts(stringified))
	assert.Equal(t, structured(t, object), structured(t, stringified))

	null := f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool", "arguments": nil})
	require.False(t, null.IsError, texts(null))
	assert.Equal(t, map[string]any{"tool": "read_tool", "args": map[string]any{}}, structured(t, null))

	all := f.call(t, metatool.SearchTools, json.RawMessage(`null`))
	require.False(t, all.IsError, texts(all))
	assert.Equal(t, float64(4), structured(t, all)["total"])

	requireToolError(t, f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool", "arguments": "null"}),
		"call_read_tool needs {name: string, arguments?: object}")
}

func TestMetaTools_SyntheticListCarriesExtra(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})
	f.probe.reset()

	res := f.call(t, metatool.SearchTools, map[string]any{"query": "read"})
	require.False(t, res.IsError, texts(res))

	var lists []probeSeen
	for _, s := range f.probe.requests() {
		if s.method == methodToolsList {
			lists = append(lists, s)
		}
	}
	require.NotEmpty(t, lists, "search_tools must read the catalogue through the chain")
	for _, s := range lists {
		assert.Equal(t, metaToolBearer, s.auth)
	}
}

func TestMetaTools_ErrorsPassThrough(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	f.probe.set(nil, &jsonrpc.Error{Code: -32001, Message: "x"})
	_, err := f.callErr(metatool.CallReadTool, map[string]any{"name": "read_tool"})
	requireWireError(t, err, -32001, "x")

	f.probe.set(&jsonrpc.Error{Code: -32001, Message: "y"}, nil)
	_, err = f.callErr(metatool.SearchTools, map[string]any{})
	requireWireError(t, err, -32001, "y")
}

func requireWireError(t *testing.T, err error, code int64, msg string) {
	t.Helper()
	require.Error(t, err, "want a JSON-RPC error, not a tool result")
	var wire *jsonrpc.Error
	require.True(t, errors.As(err, &wire), "want *jsonrpc.Error, got %T: %v", err, err)
	assert.Equal(t, code, wire.Code)
	assert.Equal(t, msg, wire.Message)
}

func TestMetaTools_ValidationErrorCarriesSchema(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})
	bad := map[string]any{"name": "read_tool", "arguments": map[string]any{"limit": "x"}}

	res := f.call(t, metatool.CallReadTool, bad)
	require.True(t, res.IsError)
	got := texts(res)
	require.Len(t, got, 2, got)
	assert.True(t, strings.HasPrefix(got[0], `validating "arguments"`), got[0])
	assert.True(t, strings.HasPrefix(got[1], "inputSchema of read_tool:"), got[1])

	f.probe.set(&jsonrpc.Error{Code: -32001, Message: "list down"}, nil)
	res = f.call(t, metatool.CallReadTool, bad)
	require.True(t, res.IsError)
	got = texts(res)
	require.Len(t, got, 1, got)
	assert.True(t, strings.HasPrefix(got[0], `validating "arguments"`), got[0])
}

func TestMetaTools_OutsideLoggingAndDecode(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	res := f.call(t, metatool.CallWriteTool, map[string]any{
		"name": "write_tool", "arguments": map[string]any{"name": "w", "items": `["a"]`},
	})

	require.False(t, res.IsError, texts(res))
	assert.Contains(t, f.lc.logged(), "mcp method=tools/call tool=write_tool")
	assert.Equal(t, map[string]any{"tool": "write_tool", "args": map[string]any{"name": "w", "items": []any{"a"}}},
		structured(t, res))
}

func TestMetaTools_DebugLines(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	f.call(t, metatool.CallReadTool, map[string]any{"name": "read_tool"})
	f.call(t, metatool.SearchTools, map[string]any{})
	f.call(t, metatool.CallReadTool, map[string]any{"name": "write_tool"})

	logged := f.lc.logged()
	assert.Contains(t, logged, "DEBUG mcp metatool=call_read_tool tool=read_tool outcome=forwarded\n")
	assert.Contains(t, logged, "DEBUG mcp metatool=search_tools tool=- outcome=answered\n")
	assert.Contains(t, logged, "DEBUG mcp metatool=call_read_tool tool=write_tool outcome=rejected\n")
}

func TestMetaTools_PagedCatalogue(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{PageSize: 1})
	f.probe.reset()

	all := f.call(t, metatool.SearchTools, map[string]any{})
	require.False(t, all.IsError, texts(all))
	assert.Equal(t, float64(4), structured(t, all)["total"])

	// Every page, not only the first, must carry the caller's bearer.
	var pages int
	for _, s := range f.probe.requests() {
		if s.method == methodToolsList {
			pages++
			assert.Equal(t, metaToolBearer, s.auth, "page %d", pages)
		}
	}
	assert.Greater(t, pages, 1, "PageSize 1 must make the read follow the cursor")

	bad := map[string]any{"name": "hidden_read", "arguments": map[string]any{"q": 5}}
	res := f.call(t, metatool.CallReadTool, bad)
	require.True(t, res.IsError)
	got := texts(res)
	require.Len(t, got, 2, got)
	assert.True(t, strings.HasPrefix(got[1], "inputSchema of hidden_read:"), got[1])

	// A failure on a later page fails the whole read, not just that page.
	f.probe.setLaterPage(&jsonrpc.Error{Code: -32001, Message: "page 2"})
	_, err := f.callErr(metatool.SearchTools, map[string]any{})
	requireWireError(t, err, -32001, "page 2")

	res = f.call(t, metatool.CallReadTool, bad)
	require.True(t, res.IsError)
	got = texts(res)
	require.Len(t, got, 1, got)
	assert.True(t, strings.HasPrefix(got[0], `validating "arguments"`), got[0])
}

func TestMetaTools_UnknownKindIsWrite(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{behaviours: map[string]tool.Behaviour{
		"read_tool":  tool.ReadOnly,
		"write_tool": tool.Destructive,
		"add_tool":   tool.Additive,
	}})

	res := f.call(t, metatool.SearchTools, map[string]any{"query": "hidden_read"})

	require.False(t, res.IsError, texts(res))
	entries, _ := structured(t, res)["tools"].([]any)
	require.Len(t, entries, 1)
	assert.Equal(t, "hidden_read", entries[0].(map[string]any)["name"])
	assert.Equal(t, "write", entries[0].(map[string]any)["kind"])
}

func TestMetaTools_HiddenToolPassesThrough(t *testing.T) {
	f := metaToolServer(t, metaToolOpts{})

	res := f.call(t, "hidden_read", map[string]any{"q": "z"})

	require.False(t, res.IsError, texts(res))
	assert.Equal(t, map[string]any{"tool": "hidden_read", "args": map[string]any{"q": "z"}}, structured(t, res))
}

// A typed nil request must reach next untouched, as in the other middleware of
// this package, not panic on call.Params.
func TestMetaTools_NilCallPassesThrough(t *testing.T) {
	behaviourOf := func(string) (tool.Behaviour, bool) { return 0, false }
	var got sdkmcp.Request
	next := sdkmcp.MethodHandler(func(_ context.Context, _ string, req sdkmcp.Request) (sdkmcp.Result, error) {
		got = req
		return nil, nil
	})
	var nilCall *sdkmcp.CallToolRequest

	assert.NotPanics(t, func() {
		_, _ = MetaTools(&recordingLogger{}, nil, behaviourOf)(next)(context.Background(), methodToolsCall, nilCall)
	})
	assert.Equal(t, sdkmcp.Request(nilCall), got)
}
