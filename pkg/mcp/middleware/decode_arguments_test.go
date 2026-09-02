//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// EDX-7319 — unit tests for DecodeArguments.
//
// A bare mcp.Server with one synthetic tool, so the only things under test are
// DecodeArguments and the SDK's dispatch. recordingLogger is shared with the
// other middleware tests (logging_test.go).
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const probeToolName = "probe_tool"

type probeItem struct {
	Name string `json:"name"`
}

// probeInput mirrors the shape that matters on ManageDeviceProfileInput:
// a declared array (Profiles) next to a declared string (Note).
type probeInput struct {
	Action   string      `json:"action"`
	Profiles []probeItem `json:"profiles,omitempty"`
	Note     string      `json:"note,omitempty"`
}

// probeServer wires one synthetic tool onto a bare server over an in-memory
// transport. withDecoding installs the real DecodeArguments, which reads the
// schema back off this server, so readSchemas is under test too.
func probeServer(t *testing.T, withDecoding bool) (*sdkmcp.ClientSession, *probeInput, *bool, *recordingLogger) {
	t.Helper()

	schema, err := jsonschema.For[probeInput](&jsonschema.ForOptions{})
	if err != nil {
		t.Fatalf("building probe schema: %v", err)
	}

	// Fixture honesty check: these tests are only about a stringified array,
	// so the fixture must actually advertise an array union. If jsonschema-go
	// ever stops emitting ["null","array"] for a slice, every assertion below
	// becomes meaningless, and it must fail here rather than pass quietly.
	profiles := schema.Properties["profiles"]
	if profiles == nil {
		t.Fatal(`fixture drift: profiles has no schema at all, want Types=["null","array"]`)
	}
	if strings.Join(profiles.Types, ",") != "null,array" {
		t.Fatalf(`fixture drift: profiles advertises Type=%q Types=%v, want Types=["null","array"]`,
			profiles.Type, profiles.Types)
	}
	if note := schema.Properties["note"]; note == nil || note.Type != "string" {
		t.Fatalf("fixture drift: note is not a plain string schema")
	}

	got := new(probeInput)
	ran := new(bool)
	lc := &recordingLogger{}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "edx7319-probe", Version: "0"}, nil)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        probeToolName,
		Description: "EDX-7319 probe tool",
		InputSchema: schema,
	}, func(_ context.Context, _ *sdkmcp.CallToolRequest, in probeInput) (*sdkmcp.CallToolResult, any, error) {
		*got = in
		*ran = true
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "ok"}},
		}, nil, nil
	})

	if withDecoding {
		// Mirrors Install — one call per middleware.
		server.AddReceivingMiddleware(DecodeArguments(lc))
	}

	ctx := context.Background()
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "edx7319-probe-client", Version: "0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	return cs, got, ran, lc
}

// stringifiedCall is the Claude Desktop wire shape: the declared array
// arrives as a JSON string, the declared string arrives as itself.
func stringifiedCall() *sdkmcp.CallToolParams {
	return &sdkmcp.CallToolParams{
		Name: probeToolName,
		Arguments: map[string]any{
			"action":   "add",
			"profiles": `[{"name":"vav-core-bottom"},{"name":"vav-core-top"}]`,
			"note":     "[1,2]",
		},
	}
}

func resultText(res *sdkmcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// TestDecodeArguments_GateRejectsWithoutTheMiddleware is the control for the test below
// it, in the same harness with the same call: it establishes that this fixture
// reproduces the EDX-7319 rejection at all. Without it, a green next test
// cannot tell "the rewrite works" from "the gate never fired".
func TestDecodeArguments_GateRejectsWithoutTheMiddleware(t *testing.T) {
	cs, _, ran, _ := probeServer(t, false)

	res, err := cs.CallTool(context.Background(), stringifiedCall())
	if err != nil {
		t.Fatalf("CallTool returned a transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("gate did not reject a stringified array; result = %q", resultText(res))
	}
	if text := resultText(res); !strings.Contains(text, `validating "arguments"`) {
		t.Fatalf("rejected, but not by the schema gate: %q", text)
	}
	if *ran {
		t.Fatal("the tool handler ran even though the gate rejected the call")
	}
}

// TestDecodeArguments_RewriteReachesGate is the claim the whole design rests on: a
// receiving middleware's in-place rewrite of req.Params.Arguments is what the
// gate at mcp/server.go:360 validates.
func TestDecodeArguments_RewriteReachesGate(t *testing.T) {
	cs, got, ran, lc := probeServer(t, true)

	res, err := cs.CallTool(context.Background(), stringifiedCall())
	if err != nil {
		t.Fatalf("CallTool returned a transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("gate still rejected after the middleware rewrite: %q", resultText(res))
	}
	if !*ran {
		t.Fatal("no error, but the handler never ran")
	}
	if len(got.Profiles) != 2 {
		t.Fatalf("handler received %d profiles, want 2 (%+v)", len(got.Profiles), got.Profiles)
	}
	if got.Profiles[0].Name != "vav-core-bottom" || got.Profiles[1].Name != "vav-core-top" {
		t.Fatalf("handler received the wrong profiles: %+v", got.Profiles)
	}
	if got.Action != "add" {
		t.Fatalf("scalar argument was damaged: action = %q", got.Action)
	}
	if logged := lc.logged(); !strings.Contains(logged, "properties=[profiles]") {
		t.Fatalf("the coercion was not reported as touching profiles: %q", logged)
	}
}

// TestDecodeArguments_GenuineStringSurvives is the false-positive guard the `note`
// column exists for, and the hole python-sdk#3055 fell into: a declared string
// whose value happens to parse as JSON must arrive as the five-character
// string, never as a two-element array.
func TestDecodeArguments_GenuineStringSurvives(t *testing.T) {
	cs, got, _, lc := probeServer(t, true)

	res, err := cs.CallTool(context.Background(), stringifiedCall())
	if err != nil {
		t.Fatalf("CallTool returned a transport error: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected rejection: %q", resultText(res))
	}
	if got.Note != "[1,2]" {
		t.Fatalf("note was decoded: got %q, want %q", got.Note, "[1,2]")
	}
	if logged := lc.logged(); strings.Contains(logged, "note") {
		t.Fatalf("middleware reported touching a declared string argument: %q", logged)
	}
}

// TestDecodeArguments_UnparseableArrayStringLeftToTheGate: a array- or object-typed value
// that is a string but is not JSON is not ours to fix. It must be left
// untouched so the gate produces its original error.
func TestDecodeArguments_UnparseableArrayStringLeftToTheGate(t *testing.T) {
	cs, _, ran, lc := probeServer(t, true)

	params := stringifiedCall()
	params.Arguments.(map[string]any)["profiles"] = "[not json"

	res, err := cs.CallTool(context.Background(), params)
	if err != nil {
		t.Fatalf("CallTool returned a transport error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("an unparseable array-typed string was accepted; result = %q", resultText(res))
	}
	if text := resultText(res); !strings.Contains(text, `validating "arguments"`) {
		t.Fatalf("rejected, but not by the schema gate: %q", text)
	}
	if *ran {
		t.Fatal("the tool handler ran on an argument the gate should have rejected")
	}
	if logged := lc.logged(); strings.Contains(logged, "decoded") {
		t.Fatalf("middleware claimed a coercion it did not make: %q", logged)
	}
}

// TestDecodeArguments_DebugLineCarriesNamesNotValues pins the decision recorded in
// The line names the tool and the properties touched, and never the
// values, because arguments carry customer data. The negative half is the point
// — a green on the positive half alone would not notice a %v of the arguments.
func TestDecodeArguments_DebugLineCarriesNamesNotValues(t *testing.T) {
	cs, _, _, lc := probeServer(t, true)

	if _, err := cs.CallTool(context.Background(), stringifiedCall()); err != nil {
		t.Fatalf("CallTool returned a transport error: %v", err)
	}

	logged := lc.logged()
	if !strings.Contains(logged, "DEBUG") {
		t.Fatalf("the coercion was not logged at Debug: %q", logged)
	}
	if !strings.Contains(logged, "tool="+probeToolName) {
		t.Fatalf("log line does not name the tool: %q", logged)
	}
	if !strings.Contains(logged, "properties=[profiles]") {
		t.Fatalf("log line does not name the property touched: %q", logged)
	}
	for _, value := range []string{"vav-core-bottom", "vav-core-top", "[1,2]"} {
		if strings.Contains(logged, value) {
			t.Fatalf("log line leaked an argument value %q: %q", value, logged)
		}
	}
}

// TestDecodeArguments_FixtureMatchesSDKInference keeps probeServer representative. It
// supplies an explicit InputSchema, but 23 of the 30 real tools set none and
// rely on SDK inference; if the two ever diverged, this fixture would stop
// standing in for them.
func TestDecodeArguments_FixtureMatchesSDKInference(t *testing.T) {
	ctx := context.Background()

	serve := func(explicit bool) json.RawMessage {
		t.Helper()
		tool := &sdkmcp.Tool{Name: probeToolName, Description: "EDX-7319 probe tool"}
		if explicit {
			s, err := jsonschema.For[probeInput](&jsonschema.ForOptions{})
			if err != nil {
				t.Fatalf("inferring schema: %v", err)
			}
			tool.InputSchema = s
		}

		server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "edx7319-probe", Version: "0"}, nil)
		sdkmcp.AddTool(server, tool,
			func(_ context.Context, _ *sdkmcp.CallToolRequest, _ probeInput) (*sdkmcp.CallToolResult, any, error) {
				return &sdkmcp.CallToolResult{}, nil, nil
			})

		clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
		ss, err := server.Connect(ctx, serverTransport, nil)
		if err != nil {
			t.Fatalf("server connect: %v", err)
		}
		t.Cleanup(func() { _ = ss.Close() })

		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "edx7319-probe-client", Version: "0"}, nil)
		cs, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatalf("client connect: %v", err)
		}
		t.Cleanup(func() { _ = cs.Close() })

		list, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("tools/list: %v", err)
		}
		if len(list.Tools) != 1 {
			t.Fatalf("served %d tools, want 1", len(list.Tools))
		}
		raw, err := json.Marshal(list.Tools[0].InputSchema)
		if err != nil {
			t.Fatalf("remarshalling served schema: %v", err)
		}
		return raw
	}

	inferred := serve(false)
	explicit := serve(true)

	if string(inferred) != string(explicit) {
		t.Fatalf("supplying the schema changes what is advertised:\n  SDK-inferred: %s\n  ours:         %s",
			inferred, explicit)
	}
	if !strings.Contains(string(inferred), `"null"`) {
		t.Fatalf("advertised schema lost the array union, so this comparison proves nothing: %s", inferred)
	}
}

// TestDecodeArguments_StringifiedScalarIsNotDecoded: a stringified scalar must
// reach the gate untouched. "null" is the case that changed acceptance — for a
// ["null","array"] property it would become JSON null and pass. The positive
// control is TestDecodeArguments_RewriteReachesGate.
func TestDecodeArguments_StringifiedScalarIsNotDecoded(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "null", value: "null"},
		{name: "number", value: "5"},
		{name: "bool", value: "true"},
		{name: "quoted string", value: `"abc"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cs, got, ran, lc := probeServer(t, true)

			res, err := cs.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name:      probeToolName,
				Arguments: map[string]any{"action": "add", "profiles": tc.value},
			})
			if err != nil {
				t.Fatalf("CallTool returned a transport error: %v", err)
			}
			if !res.IsError {
				t.Fatalf("a stringified scalar %q was accepted; handler got %+v", tc.value, got)
			}
			if text := resultText(res); !strings.Contains(text, `validating "arguments"`) {
				t.Fatalf("rejected, but not by the schema gate: %q", text)
			}
			if *ran {
				t.Fatal("the tool handler ran on a stringified scalar")
			}
			// The debug line must not claim a rewrite that did not happen.
			if logged := lc.logged(); strings.Contains(logged, "profiles") {
				t.Fatalf("debug line claims it decoded profiles: %q", logged)
			}
		})
	}
}

// TestReadSchemas_OnlyCapturesJSONSchemaTools covers the server-side
// .(*jsonschema.Schema) assertion. It must be driven in process: over the wire
// Tool.InputSchema is `any` holding a map, so asserting non-nil on it always
// holds. mcp.Tool.InputSchema being `any` is what permits the map_tool case.
func TestReadSchemas_OnlyCapturesJSONSchemaTools(t *testing.T) {
	ctx := context.Background()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "edx7319-readback", Version: "0"}, nil)

	typed, err := jsonschema.For[probeInput](&jsonschema.ForOptions{})
	if err != nil {
		t.Fatalf("building probe schema: %v", err)
	}
	handler := func(_ context.Context, _ *sdkmcp.CallToolRequest, _ probeInput) (*sdkmcp.CallToolResult, any, error) {
		return &sdkmcp.CallToolResult{}, nil, nil
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: "typed_tool", Description: "x", InputSchema: typed}, handler)
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "map_tool",
		Description: "x",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, handler)

	// Drive loadSchemas through a real chain so `next` is the server's own
	// dispatcher, the same way DecodeArguments calls it.
	var schemas map[string]*jsonschema.Schema
	probe := func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if call, ok := req.(*sdkmcp.CallToolRequest); ok && method == methodToolsCall {
				schemas = readSchemas(ctx, next, call, &recordingLogger{})
			}
			return next(ctx, method, req)
		}
	}
	server.AddReceivingMiddleware(probe)

	ct, st := sdkmcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "c", Version: "0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	if _, err := cs.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      "typed_tool",
		Arguments: map[string]any{"action": "add"},
	}); err != nil {
		t.Fatalf("CallTool returned a transport error: %v", err)
	}

	if schemas == nil {
		t.Fatal("loadSchemas never ran, so the assertions below would be vacuous")
	}
	if got := schemas["typed_tool"]; got == nil {
		t.Fatalf("a tool declaring *jsonschema.Schema was not captured; map = %v", keysOf(schemas))
	} else if got.Properties["profiles"] == nil {
		t.Fatal("captured schema is not the declared one: no profiles property")
	}
	if _, ok := schemas["map_tool"]; ok {
		t.Fatal("a tool declaring map[string]any was captured; the type assertion no longer holds")
	}
}

func keysOf(m map[string]*jsonschema.Schema) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// readbackServerWithGate serves one probe tool with `intercept` on every internal
// tools/list, and `gate` on every tools/call from a middleware installed OUTSIDE
// DecodeArguments — arriving there means the request is inside the chain but has
// not yet reached the schema cache, which is what lets a caller hold several
// requests at that point and release them together.
func readbackServerWithGate(
	t *testing.T,
	intercept func(ctx context.Context, callNo int) (sdkmcp.Result, error),
	gate func(),
	rewriteCtx func(context.Context) context.Context,
) *sdkmcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "edx7319-readback", Version: "0"}, nil)

	schema, err := jsonschema.For[probeInput](&jsonschema.ForOptions{})
	if err != nil {
		t.Fatalf("building probe schema: %v", err)
	}
	sdkmcp.AddTool(server, &sdkmcp.Tool{Name: probeToolName, Description: "x", InputSchema: schema},
		func(_ context.Context, _ *sdkmcp.CallToolRequest, _ probeInput) (*sdkmcp.CallToolResult, any, error) {
			return &sdkmcp.CallToolResult{}, nil, nil
		})

	lc := &recordingLogger{}
	// Guarded, not incidentally safe. Every internal tools/list happens under
	// schemaCache's write lock today, which serialises these increments — but
	// this middleware sees EVERY tools/list, including one a client dispatches
	// directly, which is not under that lock. The fixture must not depend on the
	// thing it exists to test.
	var callMu sync.Mutex
	calls := 0
	server.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method == methodToolsList {
				callMu.Lock()
				calls++
				n := calls
				callMu.Unlock()
				if res, err := intercept(ctx, n); res != nil || err != nil {
					return res, err
				}
			}
			return next(ctx, method, req)
		}
	})

	server.AddReceivingMiddleware(DecodeArguments(lc))

	// Installed last, so it is the OUTERMOST layer and runs before the request
	// reaches DecodeArguments.
	if gate != nil || rewriteCtx != nil {
		server.AddReceivingMiddleware(func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
			return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
				if method == methodToolsCall {
					if gate != nil {
						gate()
					}
					if rewriteCtx != nil {
						ctx = rewriteCtx(ctx)
					}
				}
				return next(ctx, method, req)
			}
		})
	}

	ct, st := sdkmcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server.Connect: %v", err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestDecodeArguments_ConcurrentFirstCalls holds all 8 requests inside the chain
// before releasing them, so the load really is entered concurrently. Counting
// goroutines around cs.CallTool would not: that rises even when calls are
// serialised. Run under -race, and with -count.
func TestDecodeArguments_ConcurrentFirstCalls(t *testing.T) {
	const n = 8

	var mu sync.Mutex
	readbacks := 0

	// Released once n requests are simultaneously inside the chain. Installed
	// OUTSIDE DecodeArguments by the fixture, so arriving here means the
	// request has not yet reached the cache.
	arrived := make(chan struct{}, n)
	release := make(chan struct{})

	cs := readbackServerWithGate(t,
		func(context.Context, int) (sdkmcp.Result, error) {
			mu.Lock()
			readbacks++
			mu.Unlock()
			return nil, nil
		},
		func() {
			arrived <- struct{}{}
			<-release
		},
		nil)

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := cs.CallTool(context.Background(), stringifiedCall())
			switch {
			case err != nil:
				errs <- fmt.Errorf("transport error: %w", err)
			case res.IsError:
				errs <- fmt.Errorf("rejected: %s", resultText(res))
			}
		}()
	}

	// Wait for all n to be inside the chain before letting any proceed. If the
	// SDK ever serialises request handling per session this times out, and that
	// is the correct outcome: the test cannot measure what it claims.
	for i := 0; i < n; i++ {
		select {
		case <-arrived:
		case <-time.After(10 * time.Second):
			close(release)
			t.Fatalf("only %d of %d requests reached the chain: they are not handled concurrently, so this test cannot measure overlap", i, n)
		}
	}
	close(release)

	wg.Wait()
	close(errs)

	failed := 0
	for err := range errs {
		if failed == 0 {
			t.Errorf("concurrent first call failed: %v", err)
		}
		failed++
	}
	if failed > 0 {
		t.Fatalf("%d of %d concurrent first calls failed", failed, n)
	}

	mu.Lock()
	defer mu.Unlock()
	// The point of the cache: one readback, however many callers race in.
	if readbacks != 1 {
		t.Fatalf("the schema readback ran %d times across %d overlapping first calls, want 1", readbacks, n)
	}
}

// TestDecodeArguments_CancelledCallerDoesNotDisableDecoding pins the readback
// against a caller that has already gone away. It deliberately does NOT rely on
// go-sdk's listTools discarding its ctx (v1.7.0 mcp/server.go:929): the
// interceptor sits INSIDE DecodeArguments and refuses a cancelled ctx, i.e. it
// stands in for an SDK that honours one. Without WithoutCancel the readback
// fails, the nil map is cached for good, and the gate rejects — which is what
// the sub-test below measures, so this one cannot pass for the wrong reason.
func TestDecodeArguments_CancelledCallerDoesNotDisableDecoding(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failReadback bool
		wantDecoded  bool
	}{
		{"readback survives a cancelled caller", false, true},
		{"control: a failed readback disables decoding", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sawCancelled bool
			cs := readbackServerWithGate(t,
				func(ctx context.Context, _ int) (sdkmcp.Result, error) {
					if tc.failReadback {
						return nil, errors.New("readback refused")
					}
					// Stands in for an SDK that honours the ctx it is handed.
					if err := ctx.Err(); err != nil {
						sawCancelled = true
						return nil, err
					}
					return nil, nil
				},
				nil,
				func(ctx context.Context) context.Context {
					dead, cancel := context.WithCancel(ctx)
					cancel()
					return dead
				})

			res, err := cs.CallTool(context.Background(), stringifiedCall())
			if err != nil {
				t.Fatalf("CallTool returned a transport error: %v", err)
			}
			if sawCancelled {
				t.Fatal("the readback carried the caller's cancellation into next, so WithoutCancel is not in effect")
			}
			if got := !res.IsError; got != tc.wantDecoded {
				t.Fatalf("decoded = %v, want %v; result = %s", got, tc.wantDecoded, resultText(res))
			}
		})
	}
}

// TestDecode_UnwrapsOnlyTheDeclaredContainerKind is the only pin on three
// guarantees: the declared kind must match, numbers outside float64 range must
// survive, and the original bytes must be assigned rather than a re-marshal.
func TestDecode_UnwrapsOnlyTheDeclaredContainerKind(t *testing.T) {
	arrayProp := &jsonschema.Schema{Types: []string{"null", "array"}}
	objectProp := &jsonschema.Schema{Types: []string{"null", "object"}}
	stringProp := &jsonschema.Schema{Type: "string"}
	eitherProp := &jsonschema.Schema{Types: []string{"array", "object"}}

	cases := []struct {
		name    string
		prop    *jsonschema.Schema
		in      string // the raw JSON value of the "v" argument
		touched bool
		wantArg string // exact bytes expected for "v" afterwards
	}{
		{
			name: "stringified array into a declared array", prop: arrayProp,
			in: `"[{\"name\":\"a\"}]"`, touched: true, wantArg: `[{"name":"a"}]`,
		},
		{
			name: "stringified object into a declared object", prop: objectProp,
			in: `"{\"name\":\"a\"}"`, touched: true, wantArg: `{"name":"a"}`,
		},
		{
			name: "stringified object into a declared ARRAY is left alone", prop: arrayProp,
			in: `"{\"name\":\"a\"}"`, touched: false, wantArg: `"{\"name\":\"a\"}"`,
		},
		{
			name: "stringified array into a declared OBJECT is left alone", prop: objectProp,
			in: `"[1,2]"`, touched: false, wantArg: `"[1,2]"`,
		},
		{
			name: "number beyond float64 range is still decoded", prop: arrayProp,
			in: `"[{\"threshold\":1e400}]"`, touched: true, wantArg: `[{"threshold":1e400}]`,
		},
		{
			name: "large integer keeps every digit", prop: arrayProp,
			in: `"[10000000000000000001]"`, touched: true, wantArg: `[10000000000000000001]`,
		},
		{
			// Two things at once. Whitespace BEFORE the container is not a case
			// this table can reach — json.Unmarshal strips it before
			// unwrappable sees the value, so that lives in TestUnwrappable.
			// Whitespace inside is reachable, and the final json.Marshal compacts
			// it away: worth pinning, because the same compaction must NOT
			// rewrite number literals (see the two numeric cases below).
			name: "whitespace inside the container is compacted", prop: arrayProp,
			in: `"[ 1 , 2 ]"`, touched: true, wantArg: `[1,2]`,
		},
		{
			name: "empty array", prop: arrayProp,
			in: `"[]"`, touched: true, wantArg: `[]`,
		},
		{
			name: "empty object", prop: objectProp,
			in: `"{}"`, touched: true, wantArg: `{}`,
		},
		{
			// The only shape where declares admits both kinds.
			name: "array into a property allowing either", prop: eitherProp,
			in: `"[1,2]"`, touched: true, wantArg: `[1,2]`,
		},
		{
			name: "object into a property allowing either", prop: eitherProp,
			in: `"{\"a\":1}"`, touched: true, wantArg: `{"a":1}`,
		},
		{name: "literal null", prop: arrayProp, in: `"null"`, touched: false, wantArg: `"null"`},
		{name: "number", prop: arrayProp, in: `"5"`, touched: false, wantArg: `"5"`},
		{name: "bool", prop: arrayProp, in: `"true"`, touched: false, wantArg: `"true"`},
		{name: "quoted string", prop: arrayProp, in: `"\"abc\""`, touched: false, wantArg: `"\"abc\""`},
		{name: "not JSON at all", prop: arrayProp, in: `"[not json"`, touched: false, wantArg: `"[not json"`},
		{
			name: "trailing garbage after a container", prop: arrayProp,
			in: `"[1,2] oops"`, touched: false, wantArg: `"[1,2] oops"`,
		},
		{
			name: "a declared string is never touched", prop: stringProp,
			in: `"[1,2]"`, touched: false, wantArg: `"[1,2]"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := &jsonschema.Schema{
				Type:       "object",
				Properties: map[string]*jsonschema.Schema{"v": tc.prop},
			}
			params := &sdkmcp.CallToolParamsRaw{
				Arguments: json.RawMessage(`{"v":` + tc.in + `}`),
			}

			touched := decode(schema, params)

			if got := len(touched) > 0; got != tc.touched {
				t.Fatalf("decode reported touched=%v (%v), want %v", got, touched, tc.touched)
			}
			var args map[string]json.RawMessage
			if err := json.Unmarshal(params.Arguments, &args); err != nil {
				t.Fatalf("arguments are no longer valid JSON: %v (%s)", err, params.Arguments)
			}
			if got := string(args["v"]); got != tc.wantArg {
				t.Fatalf("argument is %s, want %s", got, tc.wantArg)
			}
		})
	}
}

// TestUnwrappable pins the predicate directly, both halves of it: the declared
// type set and the actual kind of the decoded value. It replaces two narrower
// tables that each covered one half.
func TestUnwrappable(t *testing.T) {
	anyUnion := []string{"null", "boolean", "object", "array", "number", "integer", "string"}
	for _, tc := range []struct {
		name  string
		typ   string
		types []string
		inner string
		want  bool
	}{
		// The declared shapes this repo actually advertises.
		{name: "slice as advertised", types: []string{"null", "array"}, inner: `[1,2]`, want: true},
		{name: "bare array", typ: "array", inner: `[]`, want: true},
		{name: "bare object", typ: "object", inner: `{}`, want: true},
		{name: "either kind, array given", types: []string{"array", "object"}, inner: `[1]`, want: true},
		{name: "either kind, object given", types: []string{"array", "object"}, inner: `{"a":1}`, want: true},

		// A declared string is never rewritten, even when its value is JSON.
		{name: "bare string", typ: "string", inner: `[1,2]`, want: false},
		{name: "nullable string", types: []string{"null", "string"}, inner: `[1,2]`, want: false},
		{name: "array or string", types: []string{"array", "string"}, inner: `[1,2]`, want: false},
		{name: "go any", types: anyUnion, inner: `[1,2]`, want: false},

		// The kind must match what was declared.
		{name: "object into array-only", types: []string{"null", "array"}, inner: `{"a":1}`, want: false},
		{name: "array into object-only", types: []string{"null", "object"}, inner: `[1,2]`, want: false},

		// Scalars are not containers, whatever the declaration permits.
		{name: "literal null", types: []string{"null", "array"}, inner: `null`, want: false},
		{name: "number", types: []string{"null", "array"}, inner: `5`, want: false},
		{name: "bool", types: []string{"null", "array"}, inner: `true`, want: false},
		{name: "quoted string", types: []string{"null", "array"}, inner: `"abc"`, want: false},

		{name: "no type at all", inner: `[1,2]`, want: false},
		{name: "empty value", typ: "array", inner: ``, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prop := &jsonschema.Schema{Type: tc.typ, Types: tc.types}
			if got := unwrappable(prop, json.RawMessage(tc.inner)); got != tc.want {
				t.Fatalf("unwrappable(Type=%q Types=%v, %q) = %v, want %v",
					tc.typ, tc.types, tc.inner, got, tc.want)
			}
		})
	}
	if unwrappable(nil, json.RawMessage(`[1,2]`)) {
		t.Fatal("a property with no schema was unwrapped")
	}
}

// TestReadSchemas_SurfaceOutgrowingOnePageDisablesDecoding is the one failure
// path that is not merely defensive: DefaultPageSize is 1000 against ~30 tools
// today, but a surface that grows past a page must fail loudly rather than leave
// a partial map that decodes some tools and not others.
func TestReadSchemas_SurfaceOutgrowingOnePageDisablesDecoding(t *testing.T) {
	call := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: probeToolName}}
	lc := &recordingLogger{}
	truncated := func(context.Context, string, sdkmcp.Request) (sdkmcp.Result, error) {
		return &sdkmcp.ListToolsResult{
			Tools:      []*sdkmcp.Tool{{Name: probeToolName, Description: "x"}},
			NextCursor: "more",
		}, nil
	}

	if got := readSchemas(context.Background(), truncated, call, lc); got != nil {
		t.Fatalf("a truncated readback produced a map of %d tools; decoding must be disabled", len(got))
	}
	if logged := lc.logged(); !strings.Contains(logged, "outgrew one tools/list page") {
		t.Fatalf("the truncation was not reported: %q", logged)
	}
	if !strings.Contains(lc.logged(), "ERROR") {
		t.Fatalf("the truncation was not reported at Error: %q", lc.logged())
	}
}
