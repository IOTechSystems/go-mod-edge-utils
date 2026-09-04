//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package rs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/callstate"
)

// bearerScheme is the OAuth 2.1 bearer auth scheme, matched case-insensitively.
const bearerScheme = "bearer"

// bearerAuthn gates /mcp: OPTIONS passes (CORS preflight); no bearer → 401 +
// WWW-Authenticate (RFC 9728 discovery); otherwise the token is authenticated via
// proxy-auth introspection — valid passes, KindUnauthorized → 401 challenge, any
// other failure (outage) → 503 so a client can tell an outage from a denial. A
// client that hangs up mid-introspection is not an outage and is excluded.
func bearerAuthn(validator TokenValidator, resource, metadataURL, scope string) echo.MiddlewareFunc {
	challenge := fmt.Sprintf("Bearer resource_metadata=%q", metadataURL)
	if scope != "" {
		challenge += fmt.Sprintf(", scope=%q", scope)
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.Request().Method == http.MethodOptions {
				return next(c)
			}
			authz := c.Request().Header.Get(echo.HeaderAuthorization)
			if !hasBearerToken(authz) {
				c.Response().Header().Set("WWW-Authenticate", challenge)
				return c.NoContent(http.StatusUnauthorized)
			}
			if err := validator.Validate(c.Request().Context(), authz, resource); err != nil {
				if errors.Kind(err) == errors.KindUnauthorized {
					c.Response().Header().Set("WWW-Authenticate", challenge)
					return c.NoContent(http.StatusUnauthorized)
				}
				// Nothing is delivered either way — the socket is gone — but the 503
				// claim would be wrong, and it is what an operator would be paged on.
				//
				// ⚠ This only distinguishes a hang-up because nothing on the inbound
				// path carries a deadline (the server omits a request timeout). It does
				// NOT survive that changing: http.TimeoutHandler cancels with a plain
				// context.WithTimeout and stamps no cause, so an inbound ceiling would
				// read as Abandoned here and suppress the 503. Whoever enables one must
				// stamp it, the way middleware/deadline.go does.
				if callstate.Abandoned(c.Request().Context()) {
					return context.Cause(c.Request().Context())
				}
				return c.NoContent(http.StatusServiceUnavailable)
			}
			return next(c)
		}
	}
}

// hasBearerToken reports whether authz carries a non-empty Bearer credential.
// The scheme is matched case-insensitively (RFC 6750 / RFC 7235).
func hasBearerToken(authz string) bool {
	scheme, token, found := strings.Cut(strings.TrimSpace(authz), " ")
	return found && strings.EqualFold(scheme, bearerScheme) && strings.TrimSpace(token) != ""
}
