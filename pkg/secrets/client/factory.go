/*******************************************************************************
 * Copyright 2021 Intel Corp.
 * Copyright 2024 IOTech Ltd
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

package client

import (
	"context"
	"fmt"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets/openbao"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets/types"
)

const DefaultSecretStore = "openbao"

// NewSecretsClient creates a new instance of a SecretClient based on the passed in configuration.
// The SecretClient allows access to secret(s) for the configured token.
func NewSecretsClient(ctx context.Context, config types.SecretConfig, lc log.Logger, callback secrets.TokenExpiredCallback) (SecretClient, error) {
	if ctx == nil {
		return nil, secrets.NewErrSecretStore("background ctx is required and cannot be nil")
	}

	// Currently only have one secret store type implementation, so no need to have/check type.

	switch config.Type {
	// Currently only have one secret store type implementation, so type isn't actual set in configuration
	case DefaultSecretStore:
		return openbao.NewSecretsClient(ctx, config, lc, callback)
	default:
		return nil, fmt.Errorf("invalid secrets client type of '%s'", config.Type)
	}
}
