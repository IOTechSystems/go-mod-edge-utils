//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package rs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
)

func newValidator(t *testing.T, status int) (TokenValidator, *string) {
	t.Helper()
	gotResource := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotResource = r.Header.Get(mcpCommon.ForwardedResourceHeader)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return NewHTTPValidator(srv.URL, nil), gotResource
}

func TestHTTPValidator_OKReturnsNil(t *testing.T) {
	v, gotResource := newValidator(t, http.StatusNoContent)
	err := v.Validate(context.Background(), "Bearer good", "http://localhost:59894/mcp")
	assert.NoError(t, err)
	assert.Equal(t, "http://localhost:59894/mcp", *gotResource, "the resource must be forwarded for the aud check")
}

func TestHTTPValidator_401IsUnauthorized(t *testing.T) {
	v, _ := newValidator(t, http.StatusUnauthorized)
	err := v.Validate(context.Background(), "Bearer bad", "http://localhost:59894/mcp")
	assert.Equal(t, errors.KindUnauthorized, errors.Kind(err))
}

func TestHTTPValidator_503IsServiceUnavailable(t *testing.T) {
	v, _ := newValidator(t, http.StatusServiceUnavailable)
	err := v.Validate(context.Background(), "Bearer x", "http://localhost:59894/mcp")
	assert.Equal(t, errors.KindServiceUnavailable, errors.Kind(err))
}

func TestHTTPValidator_ApiVersion(t *testing.T) {
	for _, tc := range []struct {
		name      string
		validator func(baseURL string) TokenValidator
		want      string
	}{
		{"NewHTTPValidator is proxy-auth's", func(u string) TokenValidator { return NewHTTPValidator(u, nil) },
			"/api/v3/oauth/validate"},
		{"empty version is proxy-auth's", func(u string) TokenValidator { return NewHTTPValidatorWithApiVersion(u, "", nil) },
			"/api/v3/oauth/validate"},
		{"connect-ui's", func(u string) TokenValidator { return NewHTTPValidatorWithApiVersion(u, "v1", nil) },
			"/api/v1/oauth/validate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(srv.Close)

			err := tc.validator(srv.URL).Validate(context.Background(), "Bearer good", "http://rs/mcp")

			assert.NoError(t, err)
			assert.Equal(t, tc.want, gotPath)
		})
	}
}

// The constant spells out mcpCommon.OAuthRoute's path, as a constant cannot call it.
func TestValidateRoute(t *testing.T) {
	assert.Equal(t, mcpCommon.OAuthRoute("", validateEndpoint), ValidateRoute)
}
