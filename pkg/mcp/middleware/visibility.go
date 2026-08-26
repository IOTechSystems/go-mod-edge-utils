//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"slices"
	"time"

	restinterfaces "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/tool"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/validator"
)

// methodToolsList mirrors the MCP SDK's internal `methodListTools` constant,
// which is unexported in github.com/modelcontextprotocol/go-sdk/mcp.
const methodToolsList = "tools/list"

// OAuthAuthRoutesPath is proxy-auth's batch OAuth route-authz endpoint.
const OAuthAuthRoutesPath = "/api/v3/oauth/auth-routes"

// proxyAuthApiVersion is the apiVersion proxy-auth's DTO validation requires on
// the wire.
const proxyAuthApiVersion = "v3"

// cacheScopePrivate restricts a result to the client that asked for it. MCP spec
// 2026-07-28 makes every list result carry a cache declaration and the SDK
// defaults it to "public", which is wrong for a tools/list filtered per caller.
const cacheScopePrivate = "private"

// authRoutesTimeout bounds the proxy-auth batch-authz round-trip. MCP server
// requests carry no deadline and the shared rest client has no timeout, so
// without this a proxy-auth connection that stalls before returning headers
// would block the tools/list goroutine indefinitely.
const authRoutesTimeout = 30 * time.Second

type AuthRoute struct {
	Path string `json:"path" validate:"required,dto-none-empty-string"`
	// The Method oneof constraint must stay in sync with tool.validMethods, which
	// Register enforces at startup (see pkg/mcp/tool/registry.go).
	Method string `json:"method" validate:"required,oneof=GET HEAD POST PUT DELETE CONNECT OPTIONS TRACE PATCH QUERY MUTATION SUBSCRIPTION"`
}

// AuthRouteResult defines the content for auth route result
type AuthRouteResult struct {
	AuthRoute  `json:",inline"`
	AuthResult bool `json:"authResult"`
}

// authRouteRequest defines the Request Content for POST AuthRoute DTO
type authRouteRequest struct {
	ApiVersion string    `json:"apiVersion" validate:"required"`
	RequestId  string    `json:"requestId" validate:"len=0|uuid"`
	AuthRoute  AuthRoute `json:"authRoute"`
}

// authRouteResponse defines the Response Content for POST AuthRoute DTO
type authRouteResponse struct {
	models.BaseResponse `json:",inline"`
	AuthResponses       []AuthRouteResult `json:"authResponses"`
}

// RouteAuthorizer is the proxy-auth subset tools/list filtering needs:
// authorize a whole route set for the bearer's user in one round-trip.
type RouteAuthorizer interface {
	AuthRoutes(ctx context.Context, headers map[string]string, routes []tool.Route) ([]AuthRouteResult, error)
}

// oauthAuthRoutesClient POSTs to /api/v3/oauth/auth-routes; hand-rolled for the
// same reason as oauthAuthClient (see README.md).
type oauthRoutesClient struct {
	baseURL  string
	injector restinterfaces.AuthenticationInjector
}

// NewAuthRoutesClient builds the proxy-auth batch route-authz client Visibility
// uses.
func NewAuthRoutesClient(baseURL string, injector restinterfaces.AuthenticationInjector) RouteAuthorizer {
	return oauthRoutesClient{baseURL: baseURL, injector: injector}
}

func (c oauthRoutesClient) AuthRoutes(ctx context.Context, headers map[string]string, routes []tool.Route) ([]AuthRouteResult, error) {
	reqs := make([]authRouteRequest, 0, len(routes))
	for _, r := range routes {
		req := authRouteRequest{
			ApiVersion: proxyAuthApiVersion,
			RequestId:  uuid.NewString(),
			AuthRoute:  AuthRoute{Path: r.URI, Method: r.Method},
		}
		if err := validator.Validate(req); err != nil {
			return nil, errors.NewBaseError(errors.KindContractInvalid,
				"visibility: invalid auth-route request", err)
		}
		reqs = append(reqs, req)
	}
	ctx, cancel := context.WithTimeout(ctx, authRoutesTimeout)
	defer cancel()

	var res authRouteResponse
	if err := rest.PostRequestWithRawDataAndHeaders(ctx, &res, c.baseURL, OAuthAuthRoutesPath, nil, reqs, c.injector, headers); err != nil {
		return nil, err
	}
	return res.AuthResponses, nil
}

// RejectToolsList fails every tools/list with err while passing other methods
// through. tools/call is left alone deliberately: it is refused further down, by
// the transport, so rejecting it here as well would be a second copy of one
// decision.
func RejectToolsList(err error) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method == methodToolsList {
				return nil, err
			}
			return next(ctx, method, req)
		}
	}
}

// Visibility filters tools/list down to the tools the calling user may actually
// use: it batch-authorizes the union of every listed tool's declared route
// universe and keeps a tool when any one route is allowed, and declares the
// result private so no intermediary may forward one caller's catalogue to the
// next. Union semantics, the local-tool exemption, the fail-closed paths and the
// pagination caveat are all explained in README.md.
func Visibility(lc log.Logger, client RouteAuthorizer, toolRoutes func() map[string][]tool.Route, isLocal func(string) bool, resource string) sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			if method != methodToolsList {
				return next(ctx, method, req)
			}

			list, ok := req.(*sdkmcp.ListToolsRequest)
			if !ok || list == nil {
				return nil, errors.NewBaseError(errors.KindContractInvalid,
					"visibility: malformed tools/list request", nil)
			}
			bearer, err := bearerFromListRequest(list)
			if err != nil {
				return nil, err
			}

			res, nextErr := next(ctx, method, req)
			if nextErr != nil {
				// Drop res: the filtering below never ran, so whatever next
				// produced is unfiltered by definition.
				return nil, nextErr
			}
			result, ok := res.(*sdkmcp.ListToolsResult)
			if !ok || result == nil {
				return nil, errors.NewBaseError(errors.KindServerError,
					"visibility: unexpected tools/list result type", nil)
			}

			universes := toolRoutes()
			routes := dedupedRoutes(result.Tools, universes, isLocal)
			allowed := map[tool.Route]bool{}
			if len(routes) > 0 {
				results, authErr := client.AuthRoutes(ctx, map[string]string{
					mcpCommon.AuthorizationHeader:     bearer,
					mcpCommon.ForwardedResourceHeader: resource,
				}, routes)
				if authErr != nil {
					if errors.Kind(authErr) == errors.KindUnauthorized {
						// Debug, not Error: a stale token is a caller condition and
						// happens routinely, so logging it louder would bury the
						// outage below. But the returned message is fixed, so this
						// is the only place proxy-auth's reason — expired, wrong
						// audience, bad signature — is recorded at all.
						lc.Debugf("tools/list rejected: caller not authenticated: %v", authErr)
						return nil, errors.NewBaseError(errors.KindUnauthorized,
							"access token invalid or expired", nil)
					}
					// Outage / 5xx / timeout — not a decision, so no list. The
					// returned message is generic, so this log is the only trace.
					lc.Errorf("tools/list authorization failed against %s: %v", OAuthAuthRoutesPath, authErr)
					return nil, errors.NewBaseError(errors.KindServerError,
						"authorization service unavailable", nil)
				}
				for _, r := range results {
					if r.AuthResult {
						allowed[tool.Route{URI: r.Path, Method: r.Method}] = true
					}
				}
			}

			visible := make([]*sdkmcp.Tool, 0, len(result.Tools))
			for _, t := range result.Tools {
				if isLocal(t.Name) || anyAllowed(universes[t.Name], allowed) {
					visible = append(visible, t)
				}
			}
			result.Tools = visible
			// The filtered list is the one that carries the declaration, so the two
			// cannot drift apart. ttlMs stays 0 -- a client discards a result with a
			// non-positive ttl whatever the scope says, so nothing is cached today;
			// this is what stops a later ttl for speed from opening a leak with it.
			result.CacheScope = cacheScopePrivate
			return result, nil
		}
	}
}

// dedupedRoutes collects each listed tool's declared route universe into one
// deduplicated, deterministically ordered batch. Local tools contribute nothing.
func dedupedRoutes(tools []*sdkmcp.Tool, universes map[string][]tool.Route, isLocal func(string) bool) []tool.Route {
	seen := map[tool.Route]bool{}
	out := make([]tool.Route, 0, len(universes))
	for _, t := range tools {
		if isLocal(t.Name) {
			continue
		}
		for _, r := range universes[t.Name] {
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
	}
	slices.SortFunc(out, func(a, b tool.Route) int {
		if a.URI != b.URI {
			return cmpString(a.URI, b.URI)
		}
		return cmpString(a.Method, b.Method)
	})
	return out
}

func cmpString(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// anyAllowed implements the union rule: one allowed route makes the tool visible.
// An empty universe is therefore fail-closed.
func anyAllowed(routes []tool.Route, allowed map[tool.Route]bool) bool {
	for _, r := range routes {
		if allowed[r] {
			return true
		}
	}
	return false
}

// bearerFromListRequest reads the Authorization header off the SDK-supplied
// RequestExtra, which the streamable HTTP transport populates from the inbound
// request.
func bearerFromListRequest(req *sdkmcp.ListToolsRequest) (string, error) {
	if req.Extra == nil || req.Extra.Header == nil {
		return "", errors.NewBaseError(errors.KindUnauthorized,
			"missing HTTP headers on tools/list request", nil)
	}
	bearer := req.Extra.Header.Get(mcpCommon.AuthorizationHeader)
	if bearer == "" {
		return "", errors.NewBaseError(errors.KindUnauthorized,
			"missing Authorization header", nil)
	}
	return bearer, nil
}
