//
// Copyright (C) 2024-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/secret/clients"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"
)

type AuthClient struct {
	baseUrlFunc  clients.ClientBaseUrlFunc
	authInjector interfaces.AuthenticationInjector
}

// NewAuthClient creates an instance of AuthClient
func NewAuthClient(baseUrl string, authInjector interfaces.AuthenticationInjector) interfaces.AuthClient {
	return &AuthClient{
		baseUrlFunc:  clients.GetDefaultClientBaseUrlFunc(baseUrl),
		authInjector: authInjector,
	}
}

// NewAuthClientWithUrlCallback creates an instance of AuthClient with ClientBaseUrlFunc.
func NewAuthClientWithUrlCallback(baseUrlFunc clients.ClientBaseUrlFunc, authInjector interfaces.AuthenticationInjector) interfaces.AuthClient {
	return &AuthClient{
		baseUrlFunc:  baseUrlFunc,
		authInjector: authInjector,
	}
}

// VerificationKeyByIssuer returns the JWT verification key by the specified issuer
func (ac *AuthClient) VerificationKeyByIssuer(ctx context.Context, issuer string) (res models.KeyDataResponse, err error) {
	path := common.NewPathBuilder().SetPath(common.EdgeXApiKeyRoute).SetPath(common.VerificationKeyType).SetPath(common.Issuer).SetNameFieldPath(issuer).BuildPath()
	baseUrl, goErr := clients.GetBaseUrl(ac.baseUrlFunc)
	if goErr != nil {
		return res, goErr
	}
	err = rest.GetRequest(ctx, &res, baseUrl, path, nil, ac.authInjector)
	if err != nil {
		return res, err
	}
	return res, nil
}
