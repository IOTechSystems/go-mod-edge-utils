//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"
	restinterfaces "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"
)

// OAuthRouteAuthPath is proxy-auth's OAuth route-authz endpoint (introspection +
// Casbin enforcement). Distinct from first-party /api/v3/auth, which rejects
// OAuth tokens.
const OAuthRouteAuthPath = "/api/v3/oauth/auth"

// ProxyAuth asks security-proxy-auth whether one caller may take one route.
type ProxyAuth struct {
	// BaseURL is proxy-auth's base URL.
	BaseURL string
	// Injector decorates the authorization request itself. It must NOT add the
	// MCP service's own JWT: proxy-auth reads the end user's bearer from the
	// Authorization header, and a second Authorization header would be sent.
	Injector restinterfaces.AuthenticationInjector
	// Resource is forwarded as X-Forwarded-Resource so proxy-auth confines the
	// token to its bound audience.
	Resource string
}

// Allow reports whether the caller may take route with method. A 204 allows and
// a 403 denies; anything else — including a 401 for an invalid token — is
// returned as an error, so no call is ever allowed without an explicit 204.
func (p ProxyAuth) Allow(ctx context.Context, bearer, route, method string) (bool, error) {
	headers := map[string]string{
		mcpCommon.AuthorizationHeader:     bearer,
		mcpCommon.ForwardedUriHeader:      route,
		mcpCommon.ForwardedMethodHeader:   method,
		mcpCommon.ForwardedResourceHeader: p.Resource,
	}

	var res models.BaseResponse
	err := rest.PostRequestWithRawDataAndHeaders(ctx, &res, p.BaseURL, OAuthRouteAuthPath, nil, nil, p.Injector, headers)
	if err == nil {
		return true, nil
	}
	if errors.Kind(err) == errors.KindForbidden {
		return false, nil
	}
	// A rejected token is a third outcome, not an outage: it is an answer about
	// the caller, and the client can act on it by re-authenticating. Reported as
	// a sentinel so nothing downstream has to re-derive it from an error kind.
	if errors.Kind(err) == errors.KindUnauthorized {
		return false, &UnauthenticatedError{Route: route, Method: method, Err: err}
	}
	return false, err
}

var _ Authorizer = ProxyAuth{}
