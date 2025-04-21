//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

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
