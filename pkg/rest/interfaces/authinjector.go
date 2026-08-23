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
	// SecureTransportProvider supplies the http.RoundTripper to send the request with
	SecureTransportProvider
}

// SecureTransportProvider defines an interface to obtain a secure http.RoundTripper to use when making http requests
type SecureTransportProvider interface {
	// RoundTripper returns the configured http.RoundTripper to use when making the request.
	// A nil return falls back to http.DefaultTransport.
	RoundTripper() http.RoundTripper
}
