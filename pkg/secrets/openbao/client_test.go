//
// Copyright (c) 2021 Intel Corporation
// Copyright (C) 2025 IOTech Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the License
// is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
// or implied. See the License for the specific language governing permissions and limitations under
// the License.
//

package openbao

import (
	"testing"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets/types"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	mockLogger := log.NewNopeLogger()

	validConfig := types.SecretConfig{
		RootCaCertPath: "", // Leave empty so it uses default HTTP Client
		Authentication: types.AuthenticationInfo{
			AuthToken: "my-unit-test-token",
		},
	}
	noToken := validConfig
	noToken.Authentication.AuthToken = ""

	tests := []struct {
		Name        string
		Config      types.SecretConfig
		ExpectError bool
	}{
		{"Valid", validConfig, false},
		{"Invalid - no token", noToken, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			client, err := NewClient(test.Config, nil, true, mockLogger)
			if test.ExpectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}
}
