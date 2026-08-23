//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package config holds the configuration types an MCP resource server embeds
// into its own configuration struct.
package config

// OAuthInfo holds an MCP service's OAuth 2.1 Resource Server settings. These
// only feed RFC 9728 discovery, the WWW-Authenticate challenge, and the aud
// forwarded to proxy-auth — the service never verifies tokens locally.
type OAuthInfo struct {
	// Resource is this RS's resource identifier (RFC 8707 aud / RFC 9728 resource).
	Resource string
	// AuthorizationServer is the AS issuer URL advertised in discovery.
	AuthorizationServer string
	// Scope is advertised in the WWW-Authenticate challenge (may be empty).
	Scope string
}
