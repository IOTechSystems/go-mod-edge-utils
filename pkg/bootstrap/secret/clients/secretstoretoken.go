//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest/interfaces"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"
)

type SecretStoreTokenClient struct {
	baseUrlFunc  ClientBaseUrlFunc
	authInjector interfaces.AuthenticationInjector
}

// NewSecretStoreTokenClient creates an instance of SecretStoreTokenClient
func NewSecretStoreTokenClient(baseUrl string, authInjector interfaces.AuthenticationInjector) interfaces.SecretStoreTokenClient {
	return &SecretStoreTokenClient{
		baseUrlFunc:  GetDefaultClientBaseUrlFunc(baseUrl),
		authInjector: authInjector,
	}
}

// RegenToken regenerates the secret store client token based on the specified entity id
func (ac *SecretStoreTokenClient) RegenToken(ctx context.Context, entityId string) (models.BaseResponse, error) {
	var response models.BaseResponse
	baseUrl, err := GetBaseUrl(ac.baseUrlFunc)
	if err != nil {
		return response, err
	}

	path := common.NewPathBuilder().SetPath(common.EdgeXApiTokenRoute).SetPath(common.EntityId).SetPath(entityId).BuildPath()
	err = rest.PutRequest(ctx, &response, baseUrl, path, nil, nil, ac.authInjector)
	if err != nil {
		return response, err
	}
	return response, nil
}
