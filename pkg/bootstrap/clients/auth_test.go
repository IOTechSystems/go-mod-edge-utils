//
// Copyright (C) 2024-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"

	"github.com/stretchr/testify/require"
)

type emptyAuthenticationInjector struct {
}

func (_ *emptyAuthenticationInjector) AddAuthenticationData(_ *http.Request) error {
	// Do nothing to the request; used for unit tests
	return nil
}

// NewNullAuthenticationInjector creates an instance of AuthenticationInjector
func NewNullAuthenticationInjector() interfaces.AuthenticationInjector {
	return &emptyAuthenticationInjector{}
}

func newTestServer(httpMethod string, apiRoute string, expectedResponse interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != httpMethod {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.URL.EscapedPath() != apiRoute {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(expectedResponse)
		_, _ = w.Write(b)
	}))
}

func TestVerificationKeyByIssuer(t *testing.T) {
	mockIssuer := "mockIssuer"

	path := common.NewPathBuilder().EnableNameFieldEscape(false).
		SetPath(common.EdgeXApiKeyRoute).SetPath(common.VerificationKeyType).SetPath(common.Issuer).SetNameFieldPath(mockIssuer).BuildPath()
	ts := newTestServer(http.MethodGet, path, models.KeyDataResponse{})
	defer ts.Close()

	client := NewAuthClient(ts.URL, NewNullAuthenticationInjector())
	res, err := client.VerificationKeyByIssuer(context.Background(), mockIssuer)
	require.NoError(t, err)
	require.IsType(t, models.KeyDataResponse{}, res)
}
