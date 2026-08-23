//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	goErrors "errors"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/authz"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
)

// Auth attaches the caller's bearer token to the context of every tools/call and
// reports the authorization outcome of whatever upstream calls the tool then
// makes.
//
// It does not decide anything. The decision is made once, per outgoing request,
// inside authz.Transport — which is looking at the request being sent rather
// than at a prediction of it. This middleware exists so the caller's identity
// reaches that transport, and so a refusal reads to the model as a refusal.
//
// Local tools call no upstream, so nothing authorizes them; the bearer is
// attached anyway, and endpoint-level bearerAuthn has already required a valid
// token.
func Auth(lc log.Logger) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method != methodToolsCall {
				return next(ctx, method, req)
			}

			call, ok := req.(*sdkmcp.CallToolRequest)
			if !ok || call == nil || call.Params == nil {
				return nil, errors.NewBaseError(errors.KindContractInvalid,
					"rbac: malformed tools/call request", nil)
			}

			bearer, bearerErr := bearerFromRequest(call)
			if bearerErr != nil {
				return nil, bearerErr
			}

			res, err := next(authz.WithCaller(ctx, bearer), method, req)
			return authzOutcome(lc, res, err)
		}
	}
}

// authzOutcome maps an authorization failure raised deep inside a tool's
// upstream call onto the tools/call reply.
//
// A denial is an answer the model should adapt to; an outage is the absence of
// one and must not look retryable. An error kind cannot tell them apart — all
// three can arrive as KindServiceUnavailable — hence the errors.As sentinel
// checks.
//
// The failure usually arrives in the RESULT, not the error: the SDK converts a
// tool handler's error into an isError CallToolResult before any receiving
// middleware runs, so err is nil here and only GetError still holds the sentinel.
// Reading err alone left this function dead on the real path.
func authzOutcome(lc log.Logger, res sdkmcp.Result, err error) (sdkmcp.Result, error) {
	cause := err
	if cause == nil {
		if call, ok := res.(*sdkmcp.CallToolResult); ok && call != nil {
			cause = call.GetError()
		}
	}
	if cause == nil {
		return res, nil
	}

	var denied *authz.DeniedError
	if goErrors.As(cause, &denied) {
		lc.Debugf("tools/call denied for %s %s", denied.Method, denied.Route)
		return isErrorResult("Not authorized to use this tool (" +
			mcpCommon.AuthReasonInsufficientScope + ")."), nil
	}

	// Checked before the outage branch: a rejected token is an answer about the
	// caller, and the client can act on it by re-authenticating. Reported with the
	// same kind and wording tools/list already uses for a 401, so one expired
	// token does not look like two different failures depending on which call hit
	// it first. The proxy-auth message is not passed through — it is internal.
	var unauth *authz.UnauthenticatedError
	if goErrors.As(cause, &unauth) {
		lc.Debugf("tools/call rejected for %s %s: caller not authenticated: %v", unauth.Method, unauth.Route, unauth.Err)
		return nil, errors.NewBaseError(errors.KindUnauthorized,
			"access token invalid or expired", nil)
	}

	var outage *authz.OutageError
	if goErrors.As(cause, &outage) {
		// The returned message is generic, so this log is the only trace.
		lc.Errorf("tools/call authorization failed for %s %s: %v", outage.Method, outage.Route, outage.Err)
		return nil, errors.NewBaseError(errors.KindServerError,
			"authorization service unavailable", nil)
	}

	// Unrelated failure: hand back exactly what came in. Returning `cause` would
	// promote a tool's own error result into a protocol error.
	return res, err
}

// bearerFromRequest reads the Authorization header off the SDK-supplied
// RequestExtra, which the streamable HTTP transport populates from the inbound
// request — the hook for reading transport headers inside a receiving middleware.
func bearerFromRequest(call *sdkmcp.CallToolRequest) (string, error) {
	if call.Extra == nil || call.Extra.Header == nil {
		return "", errors.NewBaseError(errors.KindUnauthorized,
			"missing HTTP headers on tools/call request", nil)
	}
	bearer := call.Extra.Header.Get(mcpCommon.AuthorizationHeader)
	if bearer == "" {
		return "", errors.NewBaseError(errors.KindUnauthorized,
			"missing Authorization header", nil)
	}
	return bearer, nil
}
