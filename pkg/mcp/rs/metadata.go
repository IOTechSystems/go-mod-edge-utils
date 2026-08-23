//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package rs is an MCP service's OAuth 2.1 Resource Server HTTP surface: the
// RFC 9728 discovery endpoint and the bearer-authn middleware on /mcp. Tokens
// are authenticated by proxy-auth introspection (no local verification);
// per-tool authorization lives in pkg/mcp/middleware/auth.go.
package rs

import (
	"fmt"
	"net/http"
	neturl "net/url"

	"github.com/labstack/echo/v4"

	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
	mcpConfig "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/config"
)

// MetadataPath is the RFC 9728 well-known path for protected-resource metadata.
const MetadataPath = "/.well-known/oauth-protected-resource"

// ValidateConfig rejects incomplete OAuth settings so startup fails rather than
// serving empty metadata / forwarding an empty aud. Scope stays optional.
func ValidateConfig(oauth mcpConfig.OAuthInfo) error {
	if err := requireAbsoluteURL("OAuth.Resource", oauth.Resource); err != nil {
		return err
	}
	return requireAbsoluteURL("OAuth.AuthorizationServer", oauth.AuthorizationServer)
}

// requireAbsoluteURL fails when raw is not an absolute URL (scheme + host, no
// fragment, no query). A query is rejected because metadataLocation derives the
// metadata path from the path alone and would silently drop it (RFC 8707 §2).
func requireAbsoluteURL(name, raw string) error {
	if raw == "" {
		return fmt.Errorf("%s must be set (add the OAuth config section)", name)
	}
	u, err := neturl.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Fragment != "" || u.RawQuery != "" {
		return fmt.Errorf("%s must be an absolute URL with scheme and host and no fragment or query, got %q", name, raw)
	}
	return nil
}

// protectedResourceMetadataDTO is the RFC 9728 §2 protected-resource metadata
// document. Only the two fields the RS needs are populated.
type protectedResourceMetadataDTO struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
}

// protectedResourceMetadata returns an echo handler that serves the RFC 9728
// protected-resource metadata. It requires no authentication.
func protectedResourceMetadata(resource, authorizationServer string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, protectedResourceMetadataDTO{
			Resource:             resource,
			AuthorizationServers: []string{authorizationServer},
		})
	}
}

// Register mounts the RS surface: the RFC 9728 metadata endpoint and /mcp behind
// the bearer-authn middleware. Fails fast when the OAuth config is invalid or
// validator is nil.
func Register(router *echo.Echo, mcpHandler http.Handler, oauth mcpConfig.OAuthInfo, validator TokenValidator) error {
	// Self-protect even though the server bootstrap also validates up front.
	if err := ValidateConfig(oauth); err != nil {
		return err
	}

	if validator == nil {
		return fmt.Errorf("rs.Register: validator must be non-nil")
	}

	metadataPath, metadataURL := metadataLocation(oauth.Resource)
	router.GET(metadataPath, protectedResourceMetadata(oauth.Resource, oauth.AuthorizationServer))
	router.Any(mcpCommon.MCPPath, echo.WrapHandler(mcpHandler), bearerAuthn(validator, oauth.Resource, metadataURL, oauth.Scope))
	return nil
}

// metadataLocation derives, per RFC 9728 §3.1, the metadata path and the
// absolute URL to advertise. The well-known segment is inserted between the
// resource's host and its (verbatim) path: "http://host/mcp" yields
// "/.well-known/oauth-protected-resource/mcp"; an empty or "/"-only path
// collapses to the bare well-known path. An unparseable resource (only
// reachable if config validation is bypassed) falls back to the bare path.
func metadataLocation(resource string) (path, url string) {
	u, err := neturl.Parse(resource)
	if err != nil || u.Host == "" {
		return MetadataPath, MetadataPath
	}
	resourcePath := u.EscapedPath() // preserve %-encoding; only ""/"/" mean "no path"
	if resourcePath == "/" {
		resourcePath = ""
	}
	path = MetadataPath + resourcePath
	url = u.Scheme + "://" + u.Host + path
	return path, url
}
