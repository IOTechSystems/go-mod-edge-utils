//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// caller.go — the only channel by which the end user's bearer reaches the
// transport that authorizes their upstream call. In the context because every
// layer between is library code with no parameter for it, and read at RoundTrip
// time, so the token always belongs to the request being sent.

package authz

import (
	"context"
	"net/http"

	restinterfaces "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"
)

// callerKey types the context value holding the end user's bearer token.
type callerKey struct{}

// WithCaller returns ctx carrying the end user's bearer token. Set once per
// tools/call (middleware Auth), read on every upstream request it makes.
func WithCaller(ctx context.Context, bearer string) context.Context {
	return context.WithValue(ctx, callerKey{}, bearer)
}

// CallerFrom returns the bearer WithCaller stored, or "". An empty one is passed
// to proxy-auth, which refuses it: failing closed at the authorizer keeps "every
// upstream call is authorized" from depending on a check here.
func CallerFrom(ctx context.Context) string {
	bearer, _ := ctx.Value(callerKey{}).(string)
	return bearer
}

// Injector is the AuthenticationInjector an upstream client is built with.
// Through NewInjector, never as a literal: the transport is assembled once at
// construction, so a field set afterwards would be ignored.
//
// Both halves are shared: one instance per upstream service, serving every
// caller. That is safe for exactly one reason — nothing about the caller is
// stored on either. Anything per-caller rides the request's context.
type Injector struct {
	// delegate supplies the service-JWT decoration and the base transport
	// (TLS/mTLS in secure mode).
	delegate  restinterfaces.AuthenticationInjector
	transport *Transport
}

// NewInjector builds one service's injector. A nil delegate is usable; a nil
// authorizer is not — it panics on the first RoundTrip. The fail-closed answer is
// an authorizer that refuses every route, which is what the production caller
// supplies.
func NewInjector(delegate restinterfaces.AuthenticationInjector, authorizer Authorizer, servicePrefix string) *Injector {
	return &Injector{
		delegate: delegate,
		transport: &Transport{
			Next:          transportOf(delegate),
			Authorizer:    authorizer,
			ServicePrefix: servicePrefix,
		},
	}
}

func (i *Injector) AddAuthenticationData(req *http.Request) error {
	if i.delegate == nil {
		return nil
	}
	return i.delegate.AddAuthenticationData(req)
}

// RoundTripper returns the authorizing transport — never the base one unwrapped,
// which would send the request unauthorized.
func (i *Injector) RoundTripper() http.RoundTripper { return i.transport }

var (
	_ restinterfaces.AuthenticationInjector  = (*Injector)(nil)
	_ restinterfaces.SecureTransportProvider = (*Injector)(nil)
)

// transportOf is the base transport a delegate supplies, or http.DefaultTransport
// when it supplies none. The edge-utils injector split keeps RoundTripper on the
// optional SecureTransportProvider, so a delegate that only decorates the request
// contributes no transport and the default stands in.
//
// ⚠ Never nil: a SecureProvider with no transport configured would panic a direct
// RoundTrip caller, and returning the base transport unwrapped would send the
// request unauthorized. Resolved once here, at construction — RoundTripper() is on
// the request path.
func transportOf(delegate restinterfaces.AuthenticationInjector) http.RoundTripper {
	if stp, ok := delegate.(restinterfaces.SecureTransportProvider); ok {
		if rt := stp.RoundTripper(); rt != nil {
			return rt
		}
	}
	return http.DefaultTransport
}
