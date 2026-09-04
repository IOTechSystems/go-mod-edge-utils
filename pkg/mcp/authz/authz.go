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

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/callstate"
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
// the only place an upstream call can be authorized from.
//
// It holds nothing about any caller: the bearer and the caller's context both
// ride the request that RoundTrip is handed, read at the moment it is sent. That
// is what lets one Transport — and so one upstream client — serve every caller.
// ⚠ A caller-bound field would be set once for whoever built the object and then
// used for every request through it, which no single-threaded test would show.
type Transport struct {
	// Next carries the request once it is allowed.
	Next http.RoundTripper
	// Authorizer is asked about every request.
	Authorizer Authorizer
	// ServicePrefix is the "/core-metadata"-style prefix identifying the
	// upstream service. It comes from the client's configuration, never from
	// the request URL: the URL a client sends is service-relative
	// ("/api/v3/device/all"), so the service identity lives only in the host,
	// and proxy-auth's routes are prefixed by service.
	ServicePrefix string
}

// authorizationTimeout bounds the proxy-auth call. When an MCP service installs
// middleware.Deadline it must stay STRICTLY under that ToolCallCeiling: a
// sub-ceiling equal to its parent gets no timer at all, so proxy-auth could spend
// the whole call's budget and the resulting error would then name the upstream.
// Standing alone it is simply the bound that keeps a proxy-auth which accepts the
// connection and never answers from pinning the calling goroutine.
const authorizationTimeout = 5 * time.Second

// authorizationContext derives the context the authorization call runs under: the
// caller's own (req.Context()), so a disconnect cancels it, plus the sub-deadline
// above.
func authorizationContext(req *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(req.Context(), authorizationTimeout)
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

	ctx, cancel := authorizationContext(req)
	defer cancel()

	allowed, err := t.Authorizer.Allow(ctx, CallerFrom(req.Context()), route, req.Method)
	if err != nil {
		// An authentication failure is already a typed answer; wrapping it as an
		// outage would tell the caller to retry a token that will never work.
		var unauth *UnauthenticatedError
		if errors.As(err, &unauth) {
			return nil, err
		}
		// The OWNER of the deadline, not its kind. Our own sub-ceiling expiring
		// means proxy-auth did not answer while the caller still waited: an outage.
		// A caller who stopped waiting is not, and the bare cause is enough to say
		// so — the middleware maps only the three typed answers and passes anything
		// else through untouched.
		if callstate.Abandoned(req.Context()) {
			return nil, context.Cause(req.Context())
		}
		return nil, &OutageError{Route: route, Method: req.Method, Err: err}
	}
	if !allowed {
		return nil, &DeniedError{Route: route, Method: req.Method}
	}
	return t.Next.RoundTrip(req)
}

var _ http.RoundTripper = (*Transport)(nil)
