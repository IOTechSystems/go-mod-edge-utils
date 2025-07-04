//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package interfaces

import (
	"net/http"
)

// AuthenticationInjector defines an interface to obtain a JWT and secure transport for remote service calls
type AuthenticationInjector interface {
	// AddAuthenticationData mutates an HTTP request to add authentication data
	// (suth as an Authorization: header) to an outbound HTTP request
	AddAuthenticationData(_ *http.Request) error
}
