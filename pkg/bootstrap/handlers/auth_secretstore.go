//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"net/http"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"

	"github.com/labstack/echo/v4"
)

// SecretStoreAuthenticationHandlerFunc verifies the JWT with a OpenBao-based JWT authentication check
func SecretStoreAuthenticationHandlerFunc(secretProvider interfaces.SecretProvider, lc log.Logger, token string, c echo.Context) error {
	r := c.Request()

	validToken, err := secretProvider.IsJWTValid(token)
	if err != nil {
		lc.Errorf("Error checking JWT validity by the secret provider: %v ", err)
		return echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
	} else if !validToken {
		lc.Warnf("Request to '%s' UNAUTHORIZED", r.URL.Path)
		return echo.NewHTTPError(http.StatusUnauthorized, http.StatusText(http.StatusUnauthorized))
	}
	lc.Debugf("Request to '%s' authorized", r.URL.Path)
	return nil
}
