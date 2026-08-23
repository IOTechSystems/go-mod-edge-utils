//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"

	"net/http"

	restinterfaces "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"
)

// callerKey types the context value holding the end user's bearer token.
type callerKey struct{}

// WithCaller returns ctx carrying the end user's bearer token, so the upstream
// client built further down the call can bind its authorization to this caller.
// Set once per tools/call, from the inbound request's Authorization header.
func WithCaller(ctx context.Context, bearer string) context.Context {
	return context.WithValue(ctx, callerKey{}, bearer)
}

// CallerFrom returns the bearer token WithCaller stored, or "" when the context
// carries none. An empty bearer is not treated as an error here: it is passed to
// proxy-auth, which refuses it. Failing closed at the authorizer keeps the
// "every upstream call is authorized" property from depending on a check here.
func CallerFrom(ctx context.Context) string {
	bearer, _ := ctx.Value(callerKey{}).(string)
	return bearer
}

// Injector is the AuthenticationInjector an upstream client is built with. It
// keeps the delegate's request decoration — the service's own JWT, which the
// upstream service still requires — and replaces the transport with one that
// authorizes the end user against every request before it is sent.
//
// One Injector serves one caller and one upstream service. Sharing an instance
// between callers would let one caller's request be authorized with another's
// token, which no single-threaded test would show.
type Injector struct {
	// Delegate supplies the service-JWT decoration and the base transport
	// (TLS/mTLS in secure mode).
	Delegate restinterfaces.AuthenticationInjector
	// Authorizer, Bearer, ServicePrefix and Ctx are handed to the Transport.
	Authorizer    Authorizer
	Bearer        string
	ServicePrefix string
	// Ctx is the caller's context. It travels with the bearer because the two
	// answer the same question — which call is this? — and neither can reach the
	// Transport any other way.
	Ctx context.Context
}

func (i *Injector) AddAuthenticationData(req *http.Request) error {
	if i.Delegate == nil {
		return nil
	}
	return i.Delegate.AddAuthenticationData(req)
}

// RoundTripper returns the authorizing transport. It never returns nil: a
// SecureProvider with no transport configured would panic a direct RoundTrip
// caller, and returning the base transport unwrapped would send the request
// unauthorized.
func (i *Injector) RoundTripper() http.RoundTripper {
	next := http.DefaultTransport
	if i.Delegate != nil {
		if rt := i.Delegate.RoundTripper(); rt != nil {
			next = rt
		}
	}
	return &Transport{
		Next:          next,
		Authorizer:    i.Authorizer,
		Bearer:        i.Bearer,
		ServicePrefix: i.ServicePrefix,
		Ctx:           i.Ctx,
	}
}

var _ restinterfaces.AuthenticationInjector = (*Injector)(nil)
