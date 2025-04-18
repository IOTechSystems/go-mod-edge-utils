/*******************************************************************************
 * Copyright 2022 Intel Inc.
 * Copyright 2023-2025 IOTech Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package secret

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/config"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/environment"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret/token/authtokenloader/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	expectedUsername    = "admin"
	expectedPassword    = "password"
	expectedSecretName  = "postgres"
	expectedInsecureJWT = "" // Empty when in non-secure mode
	expectedSecureJWT   = "secureJwtToken"
)

const (
	UsernameKey = "username"
	PasswordKey = "password"
)

// nolint: gosec
var testTokenResponse = `{"auth":{"accessor":"9OvxnrjgV0JTYMeBreak7YJ9","client_token":"s.oPJ8uuJCkTRb2RDdcNova8wg","entity_id":"","lease_duration":3600,"metadata":{"edgex-service-name":"edgex-core-data"},"orphan":true,"policies":["default","edgex-service-edgex-core-data"],"renewable":true,"token_policies":["default","edgex-service-edgex-core-data"],"token_type":"service"},"data":null,"lease_duration":0,"lease_id":"","renewable":false,"request_id":"ee749ee1-c8bf-6fa9-3ed5-644181fc25b0","warnings":null,"wrap_info":null}`
var expectedSecrets = map[string]string{UsernameKey: expectedUsername, PasswordKey: expectedPassword}
var expectedSecureJwtData = map[string]string{"token": expectedSecureJWT}

func TestNewSecretProvider(t *testing.T) {
	tests := []struct {
		Name   string
		Secure string
	}{
		{"Valid Secure", "true"},
		{"Valid Insecure", "false"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			_ = os.Setenv(common.EnvSecretStore, tc.Secure)
			timer := startup.NewStartUpTimer("UnitTest")

			mockLogger := log.NewMockClient()
			dic := di.NewContainer(di.ServiceConstructorMap{
				container.LoggerInterfaceName: func(get di.Get) any {
					return mockLogger
				},
			})

			var configuration config.GeneralConfiguration
			expectedJWT := expectedInsecureJWT

			if tc.Secure == "true" {
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.RequestURI {
					case "/v1/auth/token/lookup-self":
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(testTokenResponse))
					case "/v1/secret/edgex/testServiceKey/postgres":
						w.WriteHeader(http.StatusOK)
						data := make(map[string]any)
						data["data"] = expectedSecrets
						response, _ := json.Marshal(data)
						_, _ = w.Write(response)
					case "/v1/identity/oidc/token/testServiceKey":
						w.WriteHeader(http.StatusOK)
						data := make(map[string]any)
						data["data"] = expectedSecureJwtData
						response, _ := json.Marshal(data)
						_, _ = w.Write(response)
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				}))
				defer testServer.Close()

				serverUrl, _ := url.Parse(testServer.URL)
				err := os.Setenv("SECRETSTORE_PORT", serverUrl.Port())
				require.NoError(t, err)

				mockTokenLoader := &mocks.AuthTokenLoader{}
				mockTokenLoader.On("Load", "/tmp/edgex/secrets/testServiceKey/secrets-token.json").Return("Test Token", nil)
				dic.Update(di.ServiceConstructorMap{
					container.AuthTokenLoaderInterfaceName: func(get di.Get) any {
						return mockTokenLoader
					},
				})

				expectedJWT = expectedSecureJWT
			} else {
				configuration = config.GeneralConfiguration{
					InsecureSecrets: map[string]config.InsecureSecretsInfo{
						"DB": {
							SecretName: expectedSecretName,
							SecretData: expectedSecrets,
						},
					},
				}
			}

			envVars := environment.NewVariables(mockLogger)

			actual, err := NewSecretProvider(&configuration, envVars, context.Background(), timer, dic, "testServiceKey")
			require.NoError(t, err)
			require.NotNil(t, actual)

			actualProvider := container.SecretProviderFrom(dic.Get)
			assert.NotNil(t, actualProvider)
			actualSecrets, err := actualProvider.GetSecret(expectedSecretName)
			require.NoError(t, err)
			assert.Equal(t, expectedUsername, actualSecrets[UsernameKey])
			assert.Equal(t, expectedPassword, actualSecrets[PasswordKey])

			actualProviderExt := container.SecretProviderExtFrom(dic.Get)
			assert.NotNil(t, actualProviderExt)

			actualJWT, err := actualProviderExt.GetSelfJWT()
			require.NoError(t, err)
			assert.Equal(t, expectedJWT, actualJWT)

		})
	}
}
