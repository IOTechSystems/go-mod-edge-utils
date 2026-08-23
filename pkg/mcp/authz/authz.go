//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package authz authorizes each upstream call at the moment it is sent.
//
// The alternative — predicting from a tool's arguments which upstream endpoint
// the call will hit, and authorizing that prediction — needs the prediction and
// the dispatch to agree forever. They are two copies of one decision, and a
// disagreement means a call authorized against one route is made against
// another. Here the request being authorized IS the request being sent: the
// check happens inside the http.RoundTripper the upstream client was built
// with, where the method and full URL are facts rather than forecasts.
package authz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Authorizer decides one (route, method) for one caller. route is the
// service-prefixed path proxy-auth's RBAC is keyed by, e.g.
// "/core-metadata/api/v3/device/all".
type Authorizer interface {
	Allow(ctx context.Context, bearer, route, method string) (bool, error)
}

// DeniedError reports that proxy-auth refused this route for this caller. It
// carries the route so the refusal can be reported without re-deriving it.
//
// Denial and outage MUST be distinguishable by type, not by EdgeX error kind:
// a refused upstream call surfaces as KindServiceUnavailable, the same kind a
// proxy-auth outage produces, so kind-based handling would let a denial be
// retried as a transient fault.
type DeniedError struct {
	Route  string
	Method string
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("authz: denied %s %s", e.Method, e.Route)
}

// UnauthenticatedError reports that proxy-auth rejected the caller's token
// itself — expired, revoked, or not valid for this resource — rather than
// refusing a route to an authenticated caller.
//
// It is a third outcome, not a flavour of OutageError: a client that re-runs its
// OAuth flow on an authentication failure must be able to tell "your token is
// stale, get a new one" from "the authorization service is down, try later".
// Collapsing the two makes a bad token look like a transient fault and hides the
// one error the client can actually act on.
type UnauthenticatedError struct {
	Route  string
	Method string
	// Err is proxy-auth's own answer. The message returned to the caller is a
	// fixed sentence, so this is the only record of WHY the token was refused —
	// expired, wrong audience, bad signature, revoked — and the middleware logs
	// it. Carried for the same reason OutageError carries its cause.
	Err error
}

func (e *UnauthenticatedError) Error() string {
	return fmt.Sprintf("authz: caller not authenticated for %s %s: %v", e.Method, e.Route, e.Err)
}

func (e *UnauthenticatedError) Unwrap() error { return e.Err }

// OutageError reports that proxy-auth could not be reached or did not answer.
// Not a decision: the call is refused, but nothing is known about the caller's
// permissions.
type OutageError struct {
	Route  string
	Method string
	Err    error
}

func (e *OutageError) Error() string {
	return fmt.Sprintf("authz: cannot authorize %s %s: %v", e.Method, e.Route, e.Err)
}

func (e *OutageError) Unwrap() error { return e.Err }

// Transport authorizes every request it carries before letting it out, and is
// the only place an upstream call can be authorized from. It is built per
// caller: the bearer is a field, because the request context does not reach
// here — go-mod-core-contracts builds upstream requests with http.NewRequest,
// not NewRequestWithContext, so req.Context() is always Background.
type Transport struct {
	// Next carries the request once it is allowed.
	Next http.RoundTripper
	// Authorizer is asked about every request.
	Authorizer Authorizer
	// Bearer is the end user's token, bound at construction.
	Bearer string
	// ServicePrefix is the "/core-metadata"-style prefix identifying the
	// upstream service. It comes from the client's configuration, never from
	// the request URL: the URL a client sends is service-relative
	// ("/api/v3/device/all"), so the service identity lives only in the host,
	// and proxy-auth's routes are prefixed by service.
	ServicePrefix string
	// Ctx is the caller's context, carried here for the same reason Bearer is:
	// req.Context() is always Background (see above), so cancelling the MCP
	// request would otherwise not reach the authorization call. Storing a context
	// in a struct is normally wrong; here the library leaves no other channel,
	// and the alternative is a call that cannot be cancelled at all. Nil is
	// tolerated — authorizationContext falls back to a bare deadline.
	Ctx context.Context
}

// authorizationTimeout bounds the proxy-auth call when nothing else does. The
// http.Client go-mod-core-contracts builds has no Timeout, so without this a
// proxy-auth that accepts the connection and never answers pins the calling
// goroutine for the life of the process.
const authorizationTimeout = 30 * time.Second

// authorizationContext derives the context the authorization call runs under:
// the caller's, so a disconnect cancels it, plus a deadline, so an unanswered
// proxy-auth cannot pin the goroutine even when the caller waits forever.
func (t *Transport) authorizationContext() (context.Context, context.CancelFunc) {
	parent := t.Ctx
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, authorizationTimeout)
}

// RoundTrip authorizes req and sends it only if allowed. On refusal or on an
// authorization failure nothing is sent — the error is returned before Next is
// reached, so no data can leave on an unauthorized call.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// EscapedPath, not Path: Path is decoded, so a device named "a/b" is sent as
	// ".../name/a%2Fb" but would be authorized as ".../name/a/b" — a different
	// route, one segment longer, which proxy-auth's per-segment patterns no
	// longer match. Authorizing anything but the bytes that leave would make
	// this package's premise ("the request being authorized IS the request being
	// sent") quietly untrue exactly where EnableNameFieldEscape is in use.
	route := t.ServicePrefix + req.URL.EscapedPath()

	ctx, cancel := t.authorizationContext()
	defer cancel()

	allowed, err := t.Authorizer.Allow(ctx, t.Bearer, route, req.Method)
	if err != nil {
		// An authentication failure is already a typed answer; wrapping it as an
		// outage would tell the caller to retry a token that will never work.
		var unauth *UnauthenticatedError
		if errors.As(err, &unauth) {
			return nil, err
		}
		return nil, &OutageError{Route: route, Method: req.Method, Err: err}
	}
	if !allowed {
		return nil, &DeniedError{Route: route, Method: req.Method}
	}
	return t.Next.RoundTrip(req)
}

var _ http.RoundTripper = (*Transport)(nil)
