//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	goErrors "errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
)

// recordingRoundTripper counts what actually got sent, which is the only way to
// tell "refused" from "sent and then reported as refused".
type recordingRoundTripper struct {
	mu       sync.Mutex
	requests []string
}

func (r *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.requests = append(r.requests, req.Method+" "+req.URL.Path)
	r.mu.Unlock()
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: req}, nil
}

func (r *recordingRoundTripper) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.requests)
}

// stubAuthorizer records what it was asked and answers as configured.
type stubAuthorizer struct {
	mu      sync.Mutex
	allow   bool
	err     error
	asked   []string
	bearers []string
}

func (s *stubAuthorizer) Allow(_ context.Context, bearer, route, method string) (bool, error) {
	s.mu.Lock()
	s.asked = append(s.asked, method+" "+route)
	s.bearers = append(s.bearers, bearer)
	s.mu.Unlock()
	return s.allow, s.err
}

func newRequest(t *testing.T, method, rawURL string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, nil)
	require.NoError(t, err)
	return req
}

func TestTransport_AllowedRequestIsSentOnce(t *testing.T) {
	next := &recordingRoundTripper{}
	authorizer := &stubAuthorizer{allow: true}
	tr := &Transport{Next: next, Authorizer: authorizer, Bearer: "Bearer abc", ServicePrefix: "/core-metadata"}

	res, err := tr.RoundTrip(newRequest(t, http.MethodGet, "http://core-metadata:59881/api/v3/device/all"))

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, 1, next.count(), "an allowed request must be sent exactly once")
	assert.Equal(t, []string{"GET /core-metadata/api/v3/device/all"}, authorizer.asked,
		"the route asked about must be the request's own path, prefixed by the service")
}

// The property the whole design rests on: on refusal nothing leaves. Asserting
// the send count, not merely that an error came back — a transport that sent
// first and reported after would satisfy "returns an error" while having already
// leaked the data.
func TestTransport_DeniedRequestIsNeverSent(t *testing.T) {
	next := &recordingRoundTripper{}
	tr := &Transport{Next: next, Authorizer: &stubAuthorizer{allow: false}, Bearer: "Bearer abc", ServicePrefix: "/core-data"}

	res, err := tr.RoundTrip(newRequest(t, http.MethodDelete, "http://core-data:59880/api/v3/event/age/1000"))

	require.Error(t, err)
	assert.Nil(t, res)
	assert.Zero(t, next.count(), "a denied request must not be sent upstream at all")

	var denied *DeniedError
	require.True(t, goErrors.As(err, &denied))
	assert.Equal(t, "/core-data/api/v3/event/age/1000", denied.Route)
	assert.Equal(t, http.MethodDelete, denied.Method)
}

func TestTransport_OutageRequestIsNeverSent(t *testing.T) {
	next := &recordingRoundTripper{}
	boom := goErrors.New("connection refused")
	tr := &Transport{Next: next, Authorizer: &stubAuthorizer{err: boom}, Bearer: "Bearer abc", ServicePrefix: "/core-metadata"}

	_, err := tr.RoundTrip(newRequest(t, http.MethodGet, "http://core-metadata:59881/api/v3/device/all"))

	require.Error(t, err)
	assert.Zero(t, next.count(), "an unanswered authorization must not let the request out")

	var outage *OutageError
	require.True(t, goErrors.As(err, &outage))
	assert.ErrorIs(t, err, boom, "the cause must stay reachable for logging")
}

// The two failures must be distinguishable by type, and must not cross-hit.
// They cannot be told apart by error kind, which is why this matters: a denial
// misread as an outage invites a retry, and an outage misread as a denial tells
// the user they lack a permission they may well have.
func TestTransport_DenialAndOutageDoNotCrossHit(t *testing.T) {
	next := &recordingRoundTripper{}

	deniedTr := &Transport{Next: next, Authorizer: &stubAuthorizer{allow: false}, ServicePrefix: "/core-metadata"}
	_, deniedErr := deniedTr.RoundTrip(newRequest(t, http.MethodGet, "http://x/api/v3/device/all"))

	outageTr := &Transport{Next: next, Authorizer: &stubAuthorizer{err: goErrors.New("timeout")}, ServicePrefix: "/core-metadata"}
	_, outageErr := outageTr.RoundTrip(newRequest(t, http.MethodGet, "http://x/api/v3/device/all"))

	var denied *DeniedError
	var outage *OutageError

	assert.True(t, goErrors.As(deniedErr, &denied))
	assert.False(t, goErrors.As(deniedErr, &outage), "a denial must not read as an outage")

	assert.True(t, goErrors.As(outageErr, &outage))
	assert.False(t, goErrors.As(outageErr, &denied), "an outage must not read as a denial")
}

// Both survive the wrapping the client layer and net/http apply on the way
// back up, which is what makes errors.As usable at the middleware.
func TestSentinels_SurviveBaseErrorAndURLErrorWrapping(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"denied", &DeniedError{Route: "/core-metadata/api/v3/device/all", Method: http.MethodGet}},
		{"outage", &OutageError{Route: "/core-metadata/api/v3/device/all", Method: http.MethodGet, Err: goErrors.New("boom")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := errors.NewBaseError(errors.KindServiceUnavailable,
				"failed to send a http request",
				&url.Error{Op: "Get", URL: "http://core-metadata/api/v3/device/all", Err: tc.err})

			assert.Equal(t, errors.KindServiceUnavailable, errors.Kind(wrapped),
				"both wrap to the same kind — this is why kind cannot be used to tell them apart")
			assert.ErrorIs(t, wrapped, tc.err, "the sentinel must stay reachable through both wrappers")
		})
	}
}

// Per-caller isolation at the unit level: two injectors built for two callers
// must authorize with their own bearers.
func TestInjector_BindsOneCallerPerInstance(t *testing.T) {
	authorizer := &stubAuthorizer{allow: true}
	next := &recordingRoundTripper{}

	for _, bearer := range []string{"Bearer alice", "Bearer bob"} {
		inj := &Injector{
			Delegate:      stubDelegate{},
			Authorizer:    authorizer,
			Bearer:        bearer,
			ServicePrefix: "/core-metadata",
		}
		tr, ok := inj.RoundTripper().(*Transport)
		require.True(t, ok, "the injector must hand back the authorizing transport, not the bare one")
		tr.Next = next
		_, err := tr.RoundTrip(newRequest(t, http.MethodGet, "http://x/api/v3/device/all"))
		require.NoError(t, err)
	}

	assert.Equal(t, []string{"Bearer alice", "Bearer bob"}, authorizer.bearers)
}

// The injector must keep decorating requests with the MCP service's own service
// JWT: the upstream service still requires it, and the authorization wrapper is
// additional, not a replacement.
func TestInjector_KeepsDelegateAuthenticationData(t *testing.T) {
	inj := &Injector{Delegate: stubDelegate{}, Authorizer: &stubAuthorizer{allow: true}}

	req := newRequest(t, http.MethodGet, "http://x/api/v3/device/all")
	require.NoError(t, inj.AddAuthenticationData(req))

	assert.Equal(t, "service-jwt", req.Header.Get(mcpCommon.AuthorizationHeader))
}

func TestInjector_NilDelegateStillAuthorizes(t *testing.T) {
	inj := &Injector{Authorizer: &stubAuthorizer{allow: false}}

	req := newRequest(t, http.MethodGet, "http://x/api/v3/device/all")
	require.NoError(t, inj.AddAuthenticationData(req), "a nil delegate must not panic")

	_, err := inj.RoundTripper().RoundTrip(req)

	var denied *DeniedError
	assert.True(t, goErrors.As(err, &denied), "a missing delegate must not mean a missing authorization")
}

// A nil delegate must still hand back a usable transport, never nil.
func TestInjector_NilDelegateFallsBackToDefaultTransport(t *testing.T) {
	inj := &Injector{Authorizer: &stubAuthorizer{allow: true}}

	tr, ok := inj.RoundTripper().(*Transport)
	require.True(t, ok)
	assert.Same(t, http.DefaultTransport, tr.Next)
}

// A delegate whose RoundTripper returns nil must fall back to the default
// transport, never nil: a direct RoundTrip caller would panic otherwise.
func TestInjector_NilDelegateTransportFallsBackToDefault(t *testing.T) {
	inj := &Injector{Delegate: stubDelegate{}, Authorizer: &stubAuthorizer{allow: true}}

	tr, ok := inj.RoundTripper().(*Transport)
	require.True(t, ok)
	assert.Same(t, http.DefaultTransport, tr.Next)
}

// The delegate's transport, when it has one, must be the one the authorized
// request is finally sent with (TLS/mTLS in secure mode).
func TestInjector_PreservesDelegateTransport(t *testing.T) {
	sentinel := &http.Transport{}
	inj := &Injector{
		Delegate:   transportDelegate{transport: sentinel},
		Authorizer: &stubAuthorizer{allow: true},
	}

	tr, ok := inj.RoundTripper().(*Transport)
	require.True(t, ok)
	assert.Same(t, sentinel, tr.Next,
		"must delegate to the configured transport so secure-mode (TLS/mTLS) calls work")
}

type stubDelegate struct{}

func (stubDelegate) AddAuthenticationData(req *http.Request) error {
	req.Header.Set(mcpCommon.AuthorizationHeader, "service-jwt")
	return nil
}
func (stubDelegate) RoundTripper() http.RoundTripper { return nil }

// transportDelegate carries a configured transport and adds nothing.
type transportDelegate struct {
	transport http.RoundTripper
}

func (transportDelegate) AddAuthenticationData(*http.Request) error { return nil }
func (d transportDelegate) RoundTripper() http.RoundTripper         { return d.transport }

// ProxyAuth maps proxy-auth's answers: 204 allows, 403 denies, everything else is
// no answer at all.
func TestProxyAuth_MapsStatusesToDecisions(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		wantAllow bool
		wantErr   bool
	}{
		{"204 allows", http.StatusNoContent, true, false},
		{"403 denies", http.StatusForbidden, false, false},
		{"401 is not a decision", http.StatusUnauthorized, false, true},
		{"500 is not a decision", http.StatusInternalServerError, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotHeaders http.Header
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeaders = r.Header.Clone()
				assert.Equal(t, OAuthRouteAuthPath, r.URL.Path)
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			p := ProxyAuth{BaseURL: srv.URL, Resource: "http://rs/mcp"}
			allow, err := p.Allow(context.Background(), "Bearer abc",
				"/core-metadata/api/v3/device/all", http.MethodGet)

			assert.Equal(t, tc.wantAllow, allow)
			if tc.wantErr {
				assert.Error(t, err, "no answer must never read as an allow")
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, "Bearer abc", gotHeaders.Get(mcpCommon.AuthorizationHeader))
			assert.Equal(t, "/core-metadata/api/v3/device/all", gotHeaders.Get(mcpCommon.ForwardedUriHeader))
			assert.Equal(t, http.MethodGet, gotHeaders.Get(mcpCommon.ForwardedMethodHeader))
			assert.NotEmpty(t, gotHeaders.Get(mcpCommon.ForwardedResourceHeader),
				"the resource must be forwarded so proxy-auth confines the token's audience")
		})
	}
}

// A 401 from proxy-auth is an authentication failure, not an outage. Both used to
// arrive as "some error", so the caller was told the authorization service was
// down and a client that re-runs its OAuth flow on a 401 reported a broken server
// instead.
//
// The negative control is the 500 case: if UnauthenticatedError were returned for
// every error, this test would pass while the distinction it exists to make was
// gone.
func TestProxyAuth_UnauthenticatedIsItsOwnSentinel(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		wantUnauth bool
	}{
		{"401 is an authentication failure", http.StatusUnauthorized, true},
		{"500 is an outage, not an authentication failure", http.StatusInternalServerError, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			p := ProxyAuth{BaseURL: srv.URL, Resource: "http://rs/mcp"}
			allow, err := p.Allow(context.Background(), "Bearer abc",
				"/core-metadata/api/v3/device/all", http.MethodGet)

			assert.False(t, allow, "no answer must never read as an allow")
			require.Error(t, err)
			var unauth *UnauthenticatedError
			assert.Equal(t, tc.wantUnauth, goErrors.As(err, &unauth),
				"401 and 5xx must be distinguishable by type, not by kind")
		})
	}
}

// The transport must not flatten an authentication failure into an outage on the
// way out — that is where the distinction was lost before.
func TestTransport_UnauthenticatedIsNotAnOutage(t *testing.T) {
	next := &recordingRoundTripper{}
	authorizer := &stubAuthorizer{err: &UnauthenticatedError{
		Route: "/core-metadata/api/v3/device/all", Method: http.MethodGet,
	}}
	tr := &Transport{Next: next, Authorizer: authorizer, Bearer: "Bearer abc",
		ServicePrefix: "/core-metadata"}

	_, err := tr.RoundTrip(newRequest(t, http.MethodGet, "http://core-metadata/api/v3/device/all"))

	require.Error(t, err)
	var unauth *UnauthenticatedError
	assert.True(t, goErrors.As(err, &unauth), "the authentication failure must survive the transport")
	var outage *OutageError
	assert.False(t, goErrors.As(err, &outage), "an expired token is not a proxy-auth outage")
	assert.Zero(t, next.count(), "nothing may be sent when the caller is not authenticated")
}

// The authorization call must be cancellable. req.Context() is always Background
// here (see the Transport doc) and the http.Client underneath has no Timeout, so
// a proxy-auth that accepts the connection and never answers would pin every
// in-flight tools/call goroutine. The caller's context is available because the
// Injector is built per call, from it.
func TestTransport_AuthorizationUsesTheCallerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Checked INSIDE the authorizer, while the call is still in flight. Asserting
	// on the context after RoundTrip returns proves nothing: RoundTrip defers its
	// own cancel, so the context is closed by then whatever the caller did. That
	// version of this test passed even with t.Ctx ignored entirely.
	propagated := false
	authorizer := authorizerFunc(func(c context.Context, _, _, _ string) (bool, error) {
		cancel()
		select {
		case <-c.Done():
			propagated = true
		default:
		}
		return true, nil
	})
	tr := &Transport{Next: &recordingRoundTripper{}, Authorizer: authorizer,
		Bearer: "Bearer abc", ServicePrefix: "/core-metadata", Ctx: ctx}

	_, err := tr.RoundTrip(newRequest(t, http.MethodGet, "http://core-metadata/api/v3/device/all"))
	require.NoError(t, err)

	assert.True(t, propagated,
		"cancelling the caller's context must cancel the in-flight authorization call")
}

// Belt and braces for the case the caller never gives up either: an unanswered
// proxy-auth must not pin the goroutine indefinitely.
func TestTransport_AuthorizationHasADeadline(t *testing.T) {
	seen := make(chan context.Context, 1)
	authorizer := authorizerFunc(func(c context.Context, _, _, _ string) (bool, error) {
		seen <- c
		return true, nil
	})
	tr := &Transport{Next: &recordingRoundTripper{}, Authorizer: authorizer,
		Bearer: "Bearer abc", ServicePrefix: "/core-metadata"}

	_, err := tr.RoundTrip(newRequest(t, http.MethodGet, "http://core-metadata/api/v3/device/all"))
	require.NoError(t, err)

	_, ok := (<-seen).Deadline()
	assert.True(t, ok, "the authorization call must be bounded even with no caller context")
}

// authorizerFunc adapts a function to Authorizer so a test can inspect the
// context the transport actually passes.
type authorizerFunc func(ctx context.Context, bearer, route, method string) (bool, error)

func (f authorizerFunc) Allow(ctx context.Context, bearer, route, method string) (bool, error) {
	return f(ctx, bearer, route, method)
}

// The string authorized must be the string that goes on the wire, or this
// package's premise ("facts rather than forecasts") is quietly untrue.
//
// req.URL.Path is DECODED: a device named "a/b" is sent as .../name/a%2Fb but
// would be authorized as .../name/a/b — one segment longer, so proxy-auth's
// per-segment patterns stop matching and the call is refused. Reachable wherever
// EnableNameFieldEscape is on, which is the flag that makes such names usable.
func TestTransport_AuthorizesTheEscapedPathThatIsActuallySent(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rawURL    string
		wantRoute string
	}{
		{
			name:      "a name containing a slash keeps its escaping",
			rawURL:    "http://core-metadata/api/v3/device/name/a%2Fb",
			wantRoute: "/core-metadata/api/v3/device/name/a%2Fb",
		},
		{
			// The control: for an ordinary name the two forms are identical, so
			// this change must be a no-op everywhere else.
			name:      "an ordinary name is unchanged",
			rawURL:    "http://core-metadata/api/v3/device/name/Random-Integer-Device",
			wantRoute: "/core-metadata/api/v3/device/name/Random-Integer-Device",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			next := &recordingRoundTripper{}
			authorizer := &stubAuthorizer{allow: true}
			tr := &Transport{Next: next, Authorizer: authorizer, Bearer: "Bearer abc",
				ServicePrefix: "/core-metadata"}

			_, err := tr.RoundTrip(newRequest(t, http.MethodGet, tc.rawURL))
			require.NoError(t, err)

			require.Len(t, authorizer.asked, 1)
			assert.Equal(t, http.MethodGet+" "+tc.wantRoute, authorizer.asked[0],
				"proxy-auth must be asked about the path the upstream actually receives")
		})
	}
}
