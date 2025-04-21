//
// Copyright (C) 2020-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"github.com/google/uuid"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
)

// Versionable shows the API version in DTOs
type Versionable struct {
	ApiVersion string `json:"apiVersion" yaml:"apiVersion" validate:"required"`
}

func NewVersionable() Versionable {
	return Versionable{ApiVersion: common.ApiVersion}
}

// BaseRequest defines the base content for request DTOs (data transfer objects).
type BaseRequest struct {
	Versionable `json:",inline"`
	RequestId   string `json:"requestId" validate:"len=0|uuid"`
}

func NewBaseRequest() BaseRequest {
	return BaseRequest{
		Versionable: NewVersionable(),
		RequestId:   uuid.NewString(),
	}
}

// BaseResponse defines the base content for response DTOs (data transfer objects).
type BaseResponse struct {
	Versionable `json:",inline"`
	RequestId   string `json:"requestId,omitempty"`
	Message     string `json:"message,omitempty"`
	StatusCode  int    `json:"statusCode"`
}

func NewBaseResponse(requestId string, message string, statusCode int) BaseResponse {
	return BaseResponse{
		Versionable: NewVersionable(),
		RequestId:   requestId,
		Message:     message,
		StatusCode:  statusCode,
	}
}
