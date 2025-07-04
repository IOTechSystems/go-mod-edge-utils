//
// Copyright (C) 2024-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package interfaces

import (
	"context"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
)

// AuthClient defines the interface for interactions with the auth API endpoint on the security-proxy-auth service.
type AuthClient interface {
	// VerificationKeyByIssuer returns the JWT verification key by the specified issuer
	VerificationKeyByIssuer(ctx context.Context, issuer string) (res models.KeyDataResponse, err error)
}
