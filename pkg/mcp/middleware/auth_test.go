//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	goErrors "errors"
	"net/http"
	"net/url"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/authz"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
)

// callReq builds a *sdkmcp.CallToolRequest carrying an Authorization header in
// Extra (mirrors how the streamable HTTP handler populates incoming requests).
func callReq(toolName, bearer string) *sdkmcp.CallToolRequest {
	header := http.Header{}
	if bearer != "" {
		header.Set(mcpCommon.AuthorizationHeader, bearer)
	}
	return &sdkmcp.CallToolRequest{
		Params: &sdkmcp.CallToolParamsRaw{Name: toolName},
		Extra:  &sdkmcp.RequestExtra{Header: header},
	}
}

func newAuthMW() sdkmcp.Middleware { return Auth(log.NewNopeLogger()) }

func TestAuth_PassesThroughNonToolsCall(t *testing.T) {
	called := false
	inner := sdkmcp.MethodHandler(func(ctx context.Context, method string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		called = true
		assert.Equal(t, "ping", method)
		assert.Empty(t, authz.CallerFrom(ctx), "only tools/call carries a caller")
		return nil, nil
	})

	_, err := newAuthMW()(inner)(context.Background(), "ping", nil)

	require.NoError(t, err)
	assert.True(t, called, "non tools/call methods must pass through to next")
}

// The middleware's whole job on the way in: make the caller reachable by the
// transport that will authorize the upstream request.
func TestAuth_PutsCallerBearerOnContext(t *testing.T) {
	var got string
	inner := sdkmcp.MethodHandler(func(ctx context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		got = authz.CallerFrom(ctx)
		return &sdkmcp.CallToolResult{}, nil
	})

	_, err := newAuthMW()(inner)(context.Background(), methodToolsCall, callReq("query_devices", "Bearer abc"))

	require.NoError(t, err)
	assert.Equal(t, "Bearer abc", got,
		"without the caller on the context every upstream call would be authorized as nobody")
}

func TestAuth_RejectsMissingAuthorizationHeader(t *testing.T) {
	_, err := newAuthMW()(failHandler(t))(context.Background(), methodToolsCall, callReq("query_devices", ""))

	require.Error(t, err)
	assert.Equal(t, errors.KindUnauthorized, errors.Kind(err))
}

func TestAuth_RejectsMissingHeaders(t *testing.T) {
	req := &sdkmcp.CallToolRequest{Params: &sdkmcp.CallToolParamsRaw{Name: "query_devices"}}

	_, err := newAuthMW()(failHandler(t))(context.Background(), methodToolsCall, req)

	require.Error(t, err)
	assert.Equal(t, errors.KindUnauthorized, errors.Kind(err))
}

func TestAuth_RejectsMalformedRequest(t *testing.T) {
	_, err := newAuthMW()(failHandler(t))(context.Background(), methodToolsCall, nil)

	require.Error(t, err)
	assert.Equal(t, errors.KindContractInvalid, errors.Kind(err))
}

// A local tool reaches its handler like any other; there is no upstream call to
// authorize, and no exemption to get wrong.
func TestAuth_LocalToolReachesHandler(t *testing.T) {
	called := false
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		called = true
		return &sdkmcp.CallToolResult{}, nil
	})

	_, err := newAuthMW()(inner)(context.Background(), methodToolsCall,
		callReq("search_guidance", "Bearer abc"))

	require.NoError(t, err)
	assert.True(t, called)
}

// deniedResult is the shape a refusal must take: a result the model can read,
// not a protocol error.
func TestAuth_DeniedBecomesIsErrorResult(t *testing.T) {
	denial := &authz.DeniedError{Route: "/core-metadata/api/v3/device/all", Method: http.MethodGet}
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		// Wrapped, as it arrives in production: the upstream client layer and
		// net/http both wrap the sentinel before it gets back here.
		return nil, errors.NewBaseError(errors.KindServiceUnavailable, "failed to send a http request",
			&url.Error{Op: "Get", URL: "http://core-metadata/api/v3/device/all", Err: denial})
	})

	res, err := newAuthMW()(inner)(context.Background(), methodToolsCall, callReq("query_devices", "Bearer abc"))

	require.NoError(t, err, "a denial is an answer, not a protocol failure")
	call, ok := res.(*sdkmcp.CallToolResult)
	require.True(t, ok)
	assert.True(t, call.IsError)
	require.Len(t, call.Content, 1)
	text, ok := call.Content[0].(*sdkmcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, text.Text, mcpCommon.AuthReasonInsufficientScope)
}

// The negative control for the test above: an outage wrapped identically — same
// error kind, same url.Error nesting — must NOT become a permission result.
// Only the sentinel type separates them.
func TestAuth_OutagePropagatesAsProtocolError(t *testing.T) {
	outage := &authz.OutageError{
		Route: "/core-metadata/api/v3/device/all", Method: http.MethodGet,
		Err: goErrors.New("connection refused"),
	}
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		return nil, errors.NewBaseError(errors.KindServiceUnavailable, "failed to send a http request",
			&url.Error{Op: "Get", URL: "http://core-metadata/api/v3/device/all", Err: outage})
	})

	res, err := newAuthMW()(inner)(context.Background(), methodToolsCall, callReq("query_devices", "Bearer abc"))

	require.Error(t, err)
	assert.Nil(t, res)
	assert.Equal(t, errors.KindServerError, errors.Kind(err))
	assert.NotContains(t, err.Error(), "connection refused", "internal failure detail must not leak")
}

// An unrelated upstream failure must reach the caller untouched — the mapping
// only claims authorization outcomes.
func TestAuth_UnrelatedErrorPassesThrough(t *testing.T) {
	sentinel := errors.NewBaseError(errors.KindEntityDoesNotExist, "device not found", nil)
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		return nil, sentinel
	})

	_, err := newAuthMW()(inner)(context.Background(), methodToolsCall, callReq("get_device", "Bearer abc"))

	require.Error(t, err)
	assert.Equal(t, sentinel, err)
}

// failHandler returns a MethodHandler that fails the test if invoked. Used by
// the "rejection" tests to assert the wrapped handler is never reached.
func failHandler(t *testing.T) sdkmcp.MethodHandler {
	t.Helper()
	return func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		t.Fatal("next handler must not be invoked when the middleware rejects the call")
		return nil, nil
	}
}

// The tools/call twin of TestVisibility_UnauthenticatedIsSanitisedButLogged.
//
// Both paths replace proxy-auth's answer with the same fixed sentence, so on both
// the log is the only record of WHY the token was refused — expired, wrong
// audience, bad signature, revoked. tools/list logs the cause; without this,
// tools/call logged only which route was attempted, so the same stale token was
// diagnosable through one entry point and not the other.
func TestAuth_UnauthenticatedLogsTheCause(t *testing.T) {
	cause := errors.NewBaseError(errors.KindUnauthorized,
		"token audience does not match resource", nil)
	inner := sdkmcp.MethodHandler(func(_ context.Context, _ string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		return nil, errors.NewBaseError(errors.KindServiceUnavailable, "failed to send a http request",
			&url.Error{Op: "Get", URL: "http://core-metadata/api/v3/device/all",
				Err: &authz.UnauthenticatedError{
					Route: "/core-metadata/api/v3/device/all", Method: http.MethodGet, Err: cause,
				}})
	})
	lc := &recordingLogger{}

	res, err := Auth(lc)(inner)(context.Background(), methodToolsCall, callReq("query_devices", "Bearer stale"))

	require.Error(t, err)
	assert.Equal(t, errors.KindUnauthorized, errors.Kind(err))
	assert.Nil(t, res)
	assert.NotContains(t, err.Error(), "audience", "the internal reason must not reach the caller")
	assert.Contains(t, lc.out.String(), "token audience does not match resource",
		"the sanitised client error drops the cause, so the log is its only trace")
}
