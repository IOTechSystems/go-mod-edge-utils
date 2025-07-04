/*******************************************************************************
 * Copyright 2023 Intel Corporation
 * Copyright 2023-2025 IOTech Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers/headers"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/secret"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// openBaoIssuer defines the issuer if JWT was issued from OpenBao
const openBaoIssuer = "/v1/identity/oidc"

// AuthenticationHandlerFunc prefixes an existing HandlerFunc,
// performing authentication checks based on OpenBao-issued JWTs or external JWTs by checking the Authorization header. Usage:
//
// authenticationHook := handlers.NilAuthenticationHandlerFunc()
//
//	if secret.IsSecurityEnabled() {
//		    authenticationHook = handlers.AuthenticationHandlerFunc(dic)
//		}
//		For optionally-authenticated requests
//		r.HandleFunc("path", authenticationHook(handlerFunc)).Methods(http.MethodGet)
//
//		For unauthenticated requests
//		r.HandleFunc("path", handlerFunc).Methods(http.MethodGet)
//
// For typical usage, it is preferred to use AutoConfigAuthenticationFunc which
// will automatically select between a real and a fake JWT validation handler.
func AuthenticationHandlerFunc(dic *di.Container) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			lc := container.LoggerFrom(dic.Get)
			secretProvider := container.SecretProviderFrom(dic.Get)
			r := c.Request()
			w := c.Response()
			authHeader := r.Header.Get("Authorization")
			lc.Debugf("Authorizing incoming call to '%s' via JWT (Authorization len=%d)", r.URL.Path, len(authHeader))

			authParts := strings.Split(authHeader, " ")
			if len(authParts) >= 2 && strings.EqualFold(authParts[0], "Bearer") {
				token := authParts[1]

				parser := jwt.NewParser()
				parsedToken, _, jwtErr := parser.ParseUnverified(token, &jwt.MapClaims{})
				if jwtErr != nil {
					w.Committed = false
					return echo.NewHTTPError(http.StatusUnauthorized, jwtErr)
				}
				issuer, jwtErr := parsedToken.Claims.GetIssuer()
				if jwtErr != nil {
					w.Committed = false
					return echo.NewHTTPError(http.StatusUnauthorized, jwtErr)
				}

				var err error
				if issuer == openBaoIssuer {
					err = SecretStoreAuthenticationHandlerFunc(secretProvider, lc, token, c)
				} else {
					// Verify the JWT by invoking security-proxy-auth http client
					err = headers.VerifyJWT(token, issuer, parsedToken.Method.Alg(), dic, r.Context())
				}
				if err != nil {
					return echo.NewHTTPError(http.StatusUnauthorized, err)
				} else {
					return next(c)
				}
			}
			err := fmt.Errorf("unable to parse JWT for call to '%s'; unauthorized", r.URL.Path)
			lc.Errorf("%v", err)
			// set Response.Committed to true in order to rewrite the status code
			w.Committed = false
			return echo.NewHTTPError(http.StatusUnauthorized, err)
		}
	}
}

// NilAuthenticationHandlerFunc just invokes a nested handler
func NilAuthenticationHandlerFunc() echo.MiddlewareFunc {
	return func(inner echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			return inner(c)
		}
	}
}

// AutoConfigAuthenticationFunc auto-selects between a HandlerFunc
// wrapper that does authentication and a HandlerFunc wrapper that does not.
// By default, JWT validation is enabled in secure mode
// (i.e. when using a real secrets provider instead of a no-op stub)
//
// Set EDGE_DISABLE_JWT_VALIDATION to 1, t, T, TRUE, true, or True
// to disable JWT validation.  This might be wanted for an
// adopter that wanted to only validate JWT's at the proxy layer,
// or as an escape hatch for a caller that cannot authenticate.
func AutoConfigAuthenticationFunc(dic *di.Container) echo.MiddlewareFunc {
	// Golang standard library treats an error as false
	disableJWTValidation, _ := strconv.ParseBool(os.Getenv(common.EnvKeyDisableJWTValidation))
	authenticationHook := NilAuthenticationHandlerFunc()
	if secret.IsSecurityEnabled() && !disableJWTValidation {
		authenticationHook = AuthenticationHandlerFunc(dic)
	}
	return authenticationHook
}
