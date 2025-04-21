//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package openbao

const (
	// NamespaceHeader specifies the header name to use when including Namespace information in a request.
	NamespaceHeader = "X-Vault-Namespace"
	AuthTypeHeader  = "X-Vault-Token"

	oidcGetTokenAPI        = "/v1/identity/oidc/token"      // nolint: gosec
	oidcTokenIntrospectAPI = "/v1/identity/oidc/introspect" // nolint: gosec

	lookupSelfTokenAPI = "/v1/auth/token/lookup-self" // nolint: gosec
	renewSelfTokenAPI  = "/v1/auth/token/renew-self"  // nolint: gosec
)
