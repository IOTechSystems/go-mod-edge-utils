/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2021 Intel Corp.
 * Copyright 2024-2025 IOTech Ltd
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
