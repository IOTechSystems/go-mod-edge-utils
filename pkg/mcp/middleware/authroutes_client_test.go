//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	mcpCommon "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/mcp/tool"
)

// authRoutesServer records the path and the request apiVersions, and answers every route allowed.
func authRoutesServer(t *testing.T) (*httptest.Server, *string, *[]string) {
	t.Helper()
	var path string
	var versions []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		var reqs []authRouteRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&reqs))
		results := make([]AuthRouteResult, 0, len(reqs))
		for _, req := range reqs {
			versions = append(versions, req.ApiVersion)
			results = append(results, AuthRouteResult{AuthRoute: req.AuthRoute, AuthResult: true})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"authResponses": results})
	}))
	t.Cleanup(srv.Close)
	return srv, &path, &versions
}

func TestNewAuthRoutesClient_ApiVersion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		client  func(baseURL string) RouteAuthorizer
		path    string
		version string
	}{
		{"NewAuthRoutesClient is proxy-auth's", func(u string) RouteAuthorizer { return NewAuthRoutesClient(u, nil) },
			"/api/v3/oauth/auth-routes", "v3"},
		{"empty version is proxy-auth's", func(u string) RouteAuthorizer { return NewAuthRoutesClientWithApiVersion(u, "", nil) },
			"/api/v3/oauth/auth-routes", "v3"},
		{"connect-ui's", func(u string) RouteAuthorizer { return NewAuthRoutesClientWithApiVersion(u, "v1", nil) },
			"/api/v1/oauth/auth-routes", "v1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, path, versions := authRoutesServer(t)

			results, err := tc.client(srv.URL).AuthRoutes(context.Background(), nil,
				[]tool.Route{getDevices, postDevice})

			require.NoError(t, err)
			assert.Len(t, results, 2)
			assert.Equal(t, tc.path, *path)
			assert.Equal(t, []string{tc.version, tc.version}, *versions, "every request carries the server's apiVersion")
		})
	}
}

// The constant spells out mcpCommon.OAuthRoute's path, as a constant cannot call it.
func TestOAuthAuthRoutesPath(t *testing.T) {
	assert.Equal(t, mcpCommon.OAuthRoute("", authRoutesEndpoint), OAuthAuthRoutesPath)
}

// A wrong apiVersion answers 404, which is only diagnosable if the error names the path.
func TestAuthRoutesClient_ErrorNamesThePath(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	_, err := NewAuthRoutesClientWithApiVersion(srv.URL, "v9", nil).AuthRoutes(context.Background(), nil, []tool.Route{getDevices})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "/api/v9/oauth/auth-routes")
	assert.Equal(t, errors.KindEntityDoesNotExist, errors.Kind(err), "the wrap must keep the status's kind")
}
