//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Visibility filters tools/list only. Local tools (those the isLocal callback
// claims) stay visible; anything else with no allowed route is dropped; any
// client failure is an error, never an unfiltered list. See README.md.

package middleware

import (
	"context"
	"net/http"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/tool"
)

const testVisibilityResource = "http://rs/mcp"

// The local tools of the test service: served in-process, no upstream route.
const (
	nameSearchGuidance = "search_guidance"
	nameGetGuidance    = "get_guidance"
)

// testIsLocal is the isLocal callback the tests hand to Visibility.
func testIsLocal(name string) bool {
	return name == nameSearchGuidance || name == nameGetGuidance
}

var (
	getDevices  = tool.Route{URI: "/core-metadata/api/v3/device/all", Method: http.MethodGet}
	postDevice  = tool.Route{URI: "/core-metadata/api/v3/device", Method: http.MethodPost}
	patchDevice = tool.Route{URI: "/core-metadata/api/v3/device", Method: http.MethodPatch}
)

// stubBatchClient answers per (path, method); anything absent from allow is denied.
type stubBatchClient struct {
	calls       int
	lastHeaders map[string]string
	lastRoutes  []tool.Route
	allow       map[tool.Route]bool
	err         error
}

func (s *stubBatchClient) AuthRoutes(_ context.Context, headers map[string]string, routes []tool.Route) ([]AuthRouteResult, error) {
	s.calls++
	s.lastHeaders = headers
	s.lastRoutes = routes
	if s.err != nil {
		return nil, s.err
	}
	out := make([]AuthRouteResult, 0, len(routes))
	for _, r := range routes {
		out = append(out, AuthRouteResult{
			AuthRoute:  AuthRoute{Path: r.URI, Method: r.Method},
			AuthResult: s.allow[r],
		})
	}
	return out, nil
}

var _ RouteAuthorizer = (*stubBatchClient)(nil)

func listReq(bearer string) *sdkmcp.ListToolsRequest {
	header := http.Header{}
	if bearer != "" {
		header.Set(mcpCommon.AuthorizationHeader, bearer)
	}
	return &sdkmcp.ListToolsRequest{
		Params: &sdkmcp.ListToolsParams{},
		Extra:  &sdkmcp.RequestExtra{Header: header},
	}
}

// listHandler returns a ListToolsResult carrying the given tool names.
func listHandler(names ...string) sdkmcp.MethodHandler {
	return func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		tools := make([]*sdkmcp.Tool, 0, len(names))
		for _, n := range names {
			tools = append(tools, &sdkmcp.Tool{Name: n})
		}
		return &sdkmcp.ListToolsResult{Tools: tools, NextCursor: "cursor-42"}, nil
	}
}

func namesOf(t *testing.T, res sdkmcp.Result) []string {
	t.Helper()
	list, ok := res.(*sdkmcp.ListToolsResult)
	require.True(t, ok, "result must be a *ListToolsResult")
	out := make([]string, 0, len(list.Tools))
	for _, tl := range list.Tools {
		out = append(out, tl.Name)
	}
	return out
}

func newVisibilityMW(c RouteAuthorizer, universes map[string][]tool.Route) sdkmcp.Middleware {
	return newVisibilityMWWithLogger(log.NewNopeLogger(), c, universes)
}

func newVisibilityMWWithLogger(lc log.Logger, c RouteAuthorizer, universes map[string][]tool.Route) sdkmcp.Middleware {
	return Visibility(lc, c, func() map[string][]tool.Route { return universes }, testIsLocal, testVisibilityResource)
}

func TestVisibility_PassesThroughNonListMethods(t *testing.T) {
	client := &stubBatchClient{}
	mw := newVisibilityMW(client, map[string][]tool.Route{})

	called := false
	inner := sdkmcp.MethodHandler(func(_ context.Context, method string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		called = true
		assert.Equal(t, "tools/call", method)
		return nil, nil
	})

	_, err := mw(inner)(context.Background(), "tools/call", nil)

	require.NoError(t, err)
	assert.True(t, called)
	assert.Zero(t, client.calls, "the batch endpoint must not be called for anything but tools/list")
}

// Union semantics: one allowed route out of several keeps the tool visible.
func TestVisibility_UnionOfAllowedRoutesKeepsToolVisible(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{patchDevice: true}}
	mw := newVisibilityMW(client, map[string][]tool.Route{
		"manage_device": {postDevice, patchDevice},
		"query_devices": {getDevices},
	})

	res, err := mw(listHandler("manage_device", "query_devices"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.NoError(t, err)
	assert.Equal(t, []string{"manage_device"}, namesOf(t, res),
		"manage_device is visible on one allowed route; query_devices has none allowed")
}

func TestVisibility_LocalToolsAlwaysVisible(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{}} // deny everything
	mw := newVisibilityMW(client, map[string][]tool.Route{"query_devices": {getDevices}})

	res, err := mw(listHandler("query_devices", nameSearchGuidance, nameGetGuidance))(
		context.Background(), "tools/list", listReq("Bearer abc"))

	require.NoError(t, err)
	assert.Equal(t, []string{nameSearchGuidance, nameGetGuidance}, namesOf(t, res),
		"local tools have no upstream route and must survive an all-deny result")
}

// An all-deny answer is a successful list with no upstream tools — not an error.
func TestVisibility_AllDeniedReturnsEmptyUpstreamSet(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{}}
	mw := newVisibilityMW(client, map[string][]tool.Route{
		"query_devices": {getDevices},
		"manage_device": {postDevice},
	})

	res, err := mw(listHandler("query_devices", "manage_device"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.NoError(t, err)
	assert.Empty(t, namesOf(t, res))
}

// A tool with no declared universe and no local exemption is fail-closed.
func TestVisibility_UnmappedToolIsHidden(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{getDevices: true}}
	mw := newVisibilityMW(client, map[string][]tool.Route{"query_devices": {getDevices}})

	res, err := mw(listHandler("query_devices", "mystery_tool"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.NoError(t, err)
	assert.Equal(t, []string{"query_devices"}, namesOf(t, res))
}

func TestVisibility_DeduplicatesRoutesAndForwardsHeaders(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{getDevices: true}}
	mw := newVisibilityMW(client, map[string][]tool.Route{
		"query_devices":  {getDevices},
		"query_devices2": {getDevices, postDevice},
	})

	_, err := mw(listHandler("query_devices", "query_devices2"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.NoError(t, err)
	require.Equal(t, 1, client.calls, "one batch round-trip per tools/list")
	assert.ElementsMatch(t, []tool.Route{getDevices, postDevice}, client.lastRoutes,
		"the shared route must be sent once")
	assert.Equal(t, "Bearer abc", client.lastHeaders[mcpCommon.AuthorizationHeader])
	assert.Equal(t, testVisibilityResource, client.lastHeaders[mcpCommon.ForwardedResourceHeader],
		"resource must be forwarded so proxy-auth can confine the OAuth token")
}

func TestVisibility_NextCursorPreserved(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{getDevices: true}}
	mw := newVisibilityMW(client, map[string][]tool.Route{"query_devices": {getDevices}})

	res, err := mw(listHandler("query_devices"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.NoError(t, err)
	list, ok := res.(*sdkmcp.ListToolsResult)
	require.True(t, ok)
	assert.Equal(t, "cursor-42", list.NextCursor, "filtering must not rewrite pagination")
}

func TestVisibility_MissingBearerIsRejected(t *testing.T) {
	client := &stubBatchClient{}
	mw := newVisibilityMW(client, map[string][]tool.Route{"query_devices": {getDevices}})

	_, err := mw(listHandler("query_devices"))(context.Background(), "tools/list", listReq(""))

	require.Error(t, err)
	assert.Zero(t, client.calls)
}

func TestVisibility_UnauthorizedPropagates(t *testing.T) {
	client := &stubBatchClient{err: errors.NewBaseError(errors.KindUnauthorized, "expired", nil)}
	mw := newVisibilityMW(client, map[string][]tool.Route{"query_devices": {getDevices}})

	res, err := mw(listHandler("query_devices"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.Error(t, err)
	assert.Equal(t, errors.KindUnauthorized, errors.Kind(err))
	assert.Nil(t, res, "no tool list may be returned when the token is not valid")
}

// The failure mode that matters: an outage must not degrade into "show everything".
func TestVisibility_OutageFailsClosedAndLeaksNoList(t *testing.T) {
	client := &stubBatchClient{err: errors.NewBaseError(errors.KindServiceUnavailable, "proxy-auth down", nil)}
	lc := &recordingLogger{}
	mw := newVisibilityMWWithLogger(lc, client, map[string][]tool.Route{
		"query_devices": {getDevices},
		"manage_device": {postDevice},
	})

	res, err := mw(listHandler("query_devices", "manage_device"))(context.Background(), "tools/list", listReq("Bearer abc"))

	require.Error(t, err)
	assert.Equal(t, errors.KindServerError, errors.Kind(err),
		"an outage is not an authorization decision")
	assert.Nil(t, res, "the unfiltered list must never reach the client")
	logged := lc.out.String()
	assert.Contains(t, logged, "ERROR", "the sanitised client error drops the cause, so the outage must be logged")
	assert.Contains(t, logged, "proxy-auth down", "the log is the only trace of the real cause")
	assert.Contains(t, logged, OAuthAuthRoutesPath, "the log must name the endpoint that failed")
}

// A failing next must not hand back the list it produced. The filtering below it
// never ran, so that result is unfiltered by definition.
func TestVisibility_NextErrorReturnsNoList(t *testing.T) {
	client := &stubBatchClient{allow: map[tool.Route]bool{getDevices: true}}
	mw := newVisibilityMW(client, map[string][]tool.Route{"query_devices": {getDevices}})

	sentinel := errors.NewBaseError(errors.KindServerError, "downstream exploded", nil)
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		return &sdkmcp.ListToolsResult{Tools: []*sdkmcp.Tool{{Name: "manage_device"}}}, sentinel
	})

	res, err := mw(inner)(context.Background(), "tools/list", listReq("Bearer abc"))

	require.Error(t, err)
	assert.Equal(t, sentinel, err, "next's error must propagate unchanged")
	assert.Nil(t, res, "an unfiltered list must not ride along with the error")
	assert.Zero(t, client.calls, "no point authorizing when the list itself failed")
}

// The Visibility filter leaves local tools in the list because endpoint-level
// bearerAuthn already ran. This fallback is built when proxy-auth is
// unconfigured, so it cannot assume that precondition and must reject the whole
// list — local tools included.
func TestRejectToolsList_RejectsLocalTools(t *testing.T) {
	sentinel := errors.NewBaseError(errors.KindServerError, "proxy-auth not configured", nil)
	mw := RejectToolsList(sentinel)

	res, err := mw(failHandler(t))(context.Background(), methodToolsList,
		listReq("Bearer abc"))

	require.Error(t, err)
	assert.Equal(t, sentinel, err, "local tools must get the fail-closed error, not a bypass")
	assert.Nil(t, res)
}

// The counterpart: the fallback is scoped to tools/list and must not break
// anything else.
func TestRejectToolsList_PassesThroughOtherMethods(t *testing.T) {
	called := false
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		called = true
		return nil, nil
	})

	_, err := RejectToolsList(errors.NewBaseError(errors.KindServerError, "unused", nil))(inner)(
		context.Background(), methodToolsCall, nil)

	require.NoError(t, err)
	assert.True(t, called)
}

// The sibling of TestVisibility_OutageFailsClosedAndLeaksNoList, for the branch
// above it. A rejected token is sanitised to a fixed sentence, so proxy-auth's
// reason — expired, wrong audience, bad signature — survives only in the log.
//
// Both halves are asserted on purpose: checking only that the client-facing
// error hides the cause would still pass with the log deleted.
func TestVisibility_UnauthenticatedIsSanitisedButLogged(t *testing.T) {
	client := &stubBatchClient{err: errors.NewBaseError(
		errors.KindUnauthorized, "token audience does not match resource", nil)}
	lc := &recordingLogger{}
	mw := newVisibilityMWWithLogger(lc, client, map[string][]tool.Route{
		"query_devices": {getDevices},
	})

	res, err := mw(listHandler("query_devices"))(context.Background(), "tools/list", listReq("Bearer stale"))

	require.Error(t, err)
	assert.Equal(t, errors.KindUnauthorized, errors.Kind(err),
		"a rejected token is an authentication failure, not a server fault")
	assert.Nil(t, res)
	assert.NotContains(t, err.Error(), "audience",
		"proxy-auth's internal reason must not reach the caller")

	logged := lc.out.String()
	assert.Contains(t, logged, "token audience does not match resource",
		"the sanitised client error drops the cause, so the log is its only trace")
}
