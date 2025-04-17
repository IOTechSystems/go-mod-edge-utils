//
// Copyright (C) 2024-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

// KeyData contains the signing or verification key for the JWT token
type KeyData struct {
	Issuer string
	Type   string
	Key    string
}

// KeyDataResponse defines the Response Content for GET KeyData DTOs.
type KeyDataResponse struct {
	BaseResponse `json:",inline"`
	KeyData      KeyData `json:"keyData"`
}
