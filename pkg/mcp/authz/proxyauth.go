//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"fmt"
	"net/http"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
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

// Allow reports whether the caller may take route with method. Only an explicit
// 204 allows; a 403 denies; a 401 for a rejected token is surfaced as a sentinel.
// Anything else — an unexpected 2xx, a 5xx, or a transport failure — is returned
// as an error, so no call is ever allowed without an explicit 204 (fail closed).
func (p ProxyAuth) Allow(ctx context.Context, bearer, route, method string) (bool, error) {
	headers := map[string]string{
		mcpCommon.AuthorizationHeader:     bearer,
		mcpCommon.ForwardedUriHeader:      route,
		mcpCommon.ForwardedMethodHeader:   method,
		mcpCommon.ForwardedResourceHeader: p.Resource,
	}

	req, err := rest.CreateRequestWithRawDataAndHeaders(ctx, http.MethodPost, p.BaseURL, OAuthRouteAuthPath, nil, nil, headers)
	if err != nil {
		return false, err
	}
	statusCode, _, err := rest.SendRequestReturningStatus(ctx, req, p.Injector)
	switch statusCode {
	case http.StatusNoContent: // 204: the only explicit allow.
		return true, nil
	case http.StatusForbidden: // 403: an explicit deny.
		return false, nil
	case http.StatusUnauthorized:
		// A rejected token is a third outcome, not an outage: it is an answer about
		// the caller, and the client can act on it by re-authenticating. Reported as
		// a sentinel so nothing downstream has to re-derive it from an error kind.
		return false, &UnauthenticatedError{Route: route, Method: method, Err: err}
	}

	if err == nil {
		err = errors.NewBaseError(errors.KindServerError, fmt.Sprintf("proxy-auth returned unexpected status %d", statusCode), nil)
	}
	return false, err
}

var _ Authorizer = ProxyAuth{}
