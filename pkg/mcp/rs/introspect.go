//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package rs

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"
	restinterfaces "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"
)

// ValidateRoute is proxy-auth's OAuth token-introspection endpoint (authn + aud, no
// route authorization). It lives under the isolated /api/v3/oauth/* namespace; the
// first-party /api/v3/auth gateway path rejects OAuth-class tokens.
const ValidateRoute = "/api/v3/oauth/validate"

// validationTimeout bounds the proxy-auth introspection call. The MCP server
// omits the request-timeout middleware and the rest package's http.Client has no
// Timeout, so without this a proxy-auth that accepts the connection and never
// answers would pin the calling goroutine for as long as the caller stays
// connected.
const validationTimeout = 30 * time.Second

// TokenValidator authenticates a bearer against proxy-auth introspection.
type TokenValidator interface {
	// Validate returns nil when valid; an error of KindUnauthorized when
	// invalid/expired/wrong-aud; other kinds for service failures. authz is
	// "Bearer <token>".
	Validate(ctx context.Context, authz, resource string) error
}

type httpValidator struct {
	baseURL  string
	injector restinterfaces.AuthenticationInjector
}

// NewHTTPValidator builds a TokenValidator that POSTs to proxy-auth's ValidateRoute.
func NewHTTPValidator(baseURL string, injector restinterfaces.AuthenticationInjector) TokenValidator {
	return &httpValidator{baseURL: baseURL, injector: injector}
}

func (v *httpValidator) Validate(ctx context.Context, authzHeader, resource string) error {
	ctx, cancel := context.WithTimeout(ctx, validationTimeout)
	defer cancel()

	headers := map[string]string{
		mcpCommon.AuthorizationHeader:     authzHeader,
		mcpCommon.ForwardedResourceHeader: resource,
	}
	req, err := rest.CreateRequestWithRawDataAndHeaders(ctx, http.MethodPost, v.baseURL, ValidateRoute, nil, nil, headers)
	if err != nil {
		return err
	}
	status, _, err := rest.SendRequestReturningStatus(ctx, req, v.injector)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent {
		return errors.NewBaseError(errors.KindServerError, fmt.Sprintf("token validation returned unexpected status %d", status), nil)
	}
	return nil
}
