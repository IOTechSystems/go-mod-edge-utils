//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package rs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpConfig "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/config"
)

// fakeValidator is a controllable TokenValidator for the authn middleware tests.
type fakeValidator struct {
	err   error
	calls int
}

func (f *fakeValidator) Validate(_ context.Context, _, _ string) error {
	f.calls++
	return f.err
}

func TestProtectedResourceMetadata_RFC9728Fields(t *testing.T) {
	e := echo.New()
	e.GET(MetadataPath, protectedResourceMetadata(
		"http://localhost:59894/mcp", "http://localhost:59842"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, MetadataPath, nil)
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "http://localhost:59894/mcp", body.Resource)
	assert.Equal(t, []string{"http://localhost:59842"}, body.AuthorizationServers)
}

func TestBearerAuthn_NoBearer_Returns401WithChallenge(t *testing.T) {
	e := echo.New()
	v := &fakeValidator{}
	// RFC 9728 §3.1 derivation (well-known segment before the resource path).
	const metadataURL = "http://localhost:59894" + MetadataPath + "/mcp"
	e.GET("/mcp", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		bearerAuthn(v, "http://localhost:59894/mcp", metadataURL, "mcp"))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `resource_metadata="`+metadataURL+`"`)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `scope="mcp"`)
	assert.Zero(t, v.calls, "no validator call when the bearer is absent")
}

func TestBearerAuthn_ValidToken_PassesThrough(t *testing.T) {
	e := echo.New()
	v := &fakeValidator{} // err nil → valid
	reached := false
	e.GET("/mcp", func(c echo.Context) error { reached = true; return c.NoContent(http.StatusOK) },
		bearerAuthn(v, "http://localhost:59894/mcp", "http://meta", "mcp"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer good")
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, reached)
	assert.Equal(t, 1, v.calls)
}

func TestBearerAuthn_InvalidToken_Returns401WithChallenge(t *testing.T) {
	e := echo.New()
	v := &fakeValidator{err: errors.NewBaseError(errors.KindUnauthorized, "bad", nil)}
	reached := false
	e.GET("/mcp", func(c echo.Context) error { reached = true; return c.NoContent(http.StatusOK) },
		bearerAuthn(v, "http://localhost:59894/mcp", "http://meta", "mcp"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer bad")
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "Bearer ")
	assert.False(t, reached, "invalid token must not reach the handler")
}

func TestBearerAuthn_Outage_Returns503(t *testing.T) {
	e := echo.New()
	v := &fakeValidator{err: errors.NewBaseError(errors.KindServiceUnavailable, "down", nil)}
	e.GET("/mcp", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		bearerAuthn(v, "http://localhost:59894/mcp", "http://meta", "mcp"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer x")
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "an outage must not be reported as an auth failure")
}

func TestBearerAuthn_OptionsPreflight_NotChallenged(t *testing.T) {
	e := echo.New()
	v := &fakeValidator{}
	reached := false
	e.Add(http.MethodOptions, "/mcp", func(c echo.Context) error { reached = true; return c.NoContent(http.StatusNoContent) },
		bearerAuthn(v, "http://localhost:59894/mcp", "http://meta", "mcp"))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/mcp", nil))

	assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
	assert.True(t, reached)
	assert.Zero(t, v.calls, "CORS preflight must not be validated")
}

func TestValidateConfig(t *testing.T) {
	full := mcpConfig.OAuthInfo{
		Resource:            "http://localhost:59894/mcp",
		AuthorizationServer: "http://localhost:59842",
	}

	assert.NoError(t, ValidateConfig(full),
		"complete config must pass")
	assert.Error(t, ValidateConfig(mcpConfig.OAuthInfo{AuthorizationServer: "http://as"}),
		"missing Resource must fail")
	assert.Error(t, ValidateConfig(mcpConfig.OAuthInfo{Resource: "http://rs"}),
		"missing AuthorizationServer must fail")
	assert.Error(t, ValidateConfig(mcpConfig.OAuthInfo{Resource: "localhost:59894/mcp", AuthorizationServer: "http://as"}),
		"non-absolute Resource must fail")
	assert.Error(t, ValidateConfig(mcpConfig.OAuthInfo{Resource: "http://rs/mcp", AuthorizationServer: "proxy-auth"}),
		"non-absolute AuthorizationServer must fail")
	assert.Error(t, ValidateConfig(mcpConfig.OAuthInfo{Resource: "http://rs/mcp#frag", AuthorizationServer: "http://as"}),
		"fragment in Resource must fail")
	assert.Error(t, ValidateConfig(mcpConfig.OAuthInfo{Resource: "http://rs/mcp?x=1", AuthorizationServer: "http://as"}),
		"query in Resource must fail")
}

func TestRegister_MountsMetadataAndChallenge(t *testing.T) {
	e := echo.New()
	mcpReached := false
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mcpReached = true
		w.WriteHeader(http.StatusOK)
	})

	require.NoError(t, Register(e, mcpHandler, mcpConfig.OAuthInfo{
		Resource:            "http://localhost:59894/mcp",
		AuthorizationServer: "http://localhost:59842",
		Scope:               "mcp",
	}, &fakeValidator{}))

	// metadata is reachable unauthenticated at the RFC 9728 path-insertion URL
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetadataPath+"/mcp", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	// MCP path without bearer is challenged
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, mcpReached, "challenge must block before the MCP handler")
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"),
		`resource_metadata="http://localhost:59894/.well-known/oauth-protected-resource/mcp"`)

	// MCP path with bearer passes through
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer t")
	e.ServeHTTP(rec, req)
	assert.True(t, mcpReached, "bearer present must reach the MCP handler")
}

func TestRegister_InvalidConfigFailsFast(t *testing.T) {
	e := echo.New()
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	// Incomplete config must fail before mounting anything, despite a valid validator.
	err := Register(e, mcpHandler, mcpConfig.OAuthInfo{
		Resource: "http://localhost:59894/mcp", // AuthorizationServer missing
	}, &fakeValidator{})

	require.Error(t, err, "invalid OAuth config must fail fast in Register, not mount a broken RS surface")
	assert.Contains(t, err.Error(), "OAuth.AuthorizationServer", "error must name the offending field")

	// Nothing must have been mounted: neither the metadata endpoint nor /mcp.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, MetadataPath+"/mcp", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code, "no metadata route may be mounted after a failed Register")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code, "no /mcp route may be mounted after a failed Register")
}

func TestRegister_NilValidatorFailsFast(t *testing.T) {
	e := echo.New()
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	err := Register(e, mcpHandler, mcpConfig.OAuthInfo{
		Resource:            "http://localhost:59894/mcp",
		AuthorizationServer: "http://localhost:59842",
	}, nil)

	require.Error(t, err, "a nil validator must fail fast, not panic later")
}

func TestRegister_AdvertisedMetadataURLIsServed(t *testing.T) {
	e := echo.New()
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	require.NoError(t, Register(e, mcpHandler, mcpConfig.OAuthInfo{
		Resource:            "http://localhost:59894/mcp",
		AuthorizationServer: "http://localhost:59842",
		Scope:               "mcp",
	}, &fakeValidator{}))

	// Trigger the challenge to learn the advertised metadata URL.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// RFC 9728 §3.1: well-known segment is inserted between host and resource path.
	const wantURL = "http://localhost:59894/.well-known/oauth-protected-resource/mcp"
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), `resource_metadata="`+wantURL+`"`)

	// The advertised URL must actually be served (this is what a client follows).
	u, err := neturl.Parse(wantURL)
	require.NoError(t, err)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u.Path, nil))
	assert.Equal(t, http.StatusOK, rec.Code, "the advertised metadata URL must be served, not 404")
}

func TestMetadataLocation_RFC9728Derivation(t *testing.T) {
	tests := []struct {
		name              string
		resource          string
		wantPath, wantURL string
	}{
		{"path", "http://host/mcp",
			MetadataPath + "/mcp", "http://host" + MetadataPath + "/mcp"},
		{"trailing slash is preserved", "http://host/mcp/",
			MetadataPath + "/mcp/", "http://host" + MetadataPath + "/mcp/"},
		{"percent-encoding is preserved", "http://host/a%20b",
			MetadataPath + "/a%20b", "http://host" + MetadataPath + "/a%20b"},
		{"root path collapses to bare", "http://host/",
			MetadataPath, "http://host" + MetadataPath},
		{"no path collapses to bare", "http://host",
			MetadataPath, "http://host" + MetadataPath},
		{"unparseable falls back to bare relative path", "://nope",
			MetadataPath, MetadataPath},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPath, gotURL := metadataLocation(tc.resource)
			assert.Equal(t, tc.wantPath, gotPath)
			assert.Equal(t, tc.wantURL, gotURL)
		})
	}
}
