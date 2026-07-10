//
// Copyright (C) 2020-2026 IOTech Ltd
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

// ErrorDetail represents a single error detail entry in an ErrorResponse.
type ErrorDetail struct {
	Message string `json:"message,omitempty"`
}

// ErrorResponse defines an error response that may include actionable details.
// This is compatible with services (e.g., alarm service) that return a "details"
// array alongside the top-level "message". It also handles standard BaseResponse
// format (message-only) since the Details field is optional.
type ErrorResponse struct {
	Message string        `json:"message,omitempty"`
	Details []ErrorDetail `json:"details,omitempty"`
}
