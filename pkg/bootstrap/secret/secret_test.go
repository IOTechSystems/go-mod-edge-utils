/*******************************************************************************
 * Copyright 2022 Intel Inc.
 * Copyright 2023 IOTech Ltd.
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
	"github.com/stretchr/testify/mock"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/config"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/environment"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/mocks"
)

const (
	expectedUsername   = "admin"
	expectedPassword   = "password"
	expectedSecretName = "redisdb"
)

const (
	UsernameKey = "username"
	PasswordKey = "password"
)

// nolint: gosec
var expectedSecrets = map[string]string{UsernameKey: expectedUsername, PasswordKey: expectedPassword}

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
			_ = os.Setenv(EnvSecretStore, tc.Secure)
			timer := startup.NewStartUpTimer("UnitTest")

			mockLogger := &loggerMocks.Logger{}
			dic := di.NewContainer(di.ServiceConstructorMap{
				container.LoggerInterfaceName: func(get di.Get) any {
					return mockLogger
				},
			})

			var configuration config.GeneralConfiguration

			if tc.Secure == "true" {
				mockLogger.On("Error", mock.AnythingOfType("string")).Return().Once()
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
			if tc.Secure == "true" {
				require.Nil(t, actual)
			} else {
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
			}
		})
	}
}
