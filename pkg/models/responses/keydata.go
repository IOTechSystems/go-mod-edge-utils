//
// Copyright (C) 2024-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/models"
)

// KeyDataResponse defines the Response Content for GET KeyData DTOs.
type KeyDataResponse struct {
	models.BaseResponse `json:",inline"`
	KeyData             models.KeyData `json:"keyData"`
}

func NewKeyDataResponse(requestId string, message string, statusCode int, keyData models.KeyData) KeyDataResponse {
	return KeyDataResponse{
		BaseResponse: models.NewBaseResponse(requestId, message, statusCode),
		KeyData:      keyData,
	}
}
