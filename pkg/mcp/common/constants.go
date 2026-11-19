//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package common holds the constants shared by the MCP resource-server,
// middleware and authorization packages under pkg/mcp.
package common

// MCPPath is the HTTP path the MCP Streamable transport listens on.
const MCPPath = "/mcp"

// AuthorizationHeader is the standard HTTP Authorization header, carrying the
// end user's bearer token on inbound MCP requests and on forwarded proxy-auth
// calls.
const AuthorizationHeader = "Authorization"

// ForwardedUriHeader and ForwardedMethodHeader carry the route under
// authorization to security-proxy-auth, matching its gateway-style contract.
const (
	ForwardedUriHeader    = "X-Forwarded-Uri"
	ForwardedMethodHeader = "X-Forwarded-Method"
)

// ForwardedResourceHeader carries the MCP service's resource identifier so
// proxy-auth can confine a token to its bound audience (RFC 8707 aud).
const ForwardedResourceHeader = "X-Forwarded-Resource"

// AuthReasonInsufficientScope is the stable RFC 6750 reason token put in
// authorization-refusal messages so clients can match on it.
const AuthReasonInsufficientScope = "insufficient_scope"

// DefaultApiVersion is security-proxy-auth's API version, which the clients of an authorization server use when they
// are given none.
const DefaultApiVersion = "v3"

// OAuthRoute returns the path of the authorization server's OAuth endpoint (e.g. "validate") under apiVersion; an
// empty apiVersion means DefaultApiVersion. An authorization server other than proxy-auth, such as connect-ui, serves
// the same endpoints under its own API version.
func OAuthRoute(apiVersion, endpoint string) string {
	if apiVersion == "" {
		apiVersion = DefaultApiVersion
	}
	return "/api/" + apiVersion + "/oauth/" + endpoint
}
