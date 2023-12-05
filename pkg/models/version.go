//
// Copyright (C) 2020-2023 IOTech Ltd
// Copyright (C) 2020 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package models

// VersionResponse defines the latest version supported by the service.
type VersionResponse struct {
	Versionable `json:",inline"`
	Version     string `json:"version"`
	ServiceName string `json:"serviceName"`
}

// NewVersionResponse creates new VersionResponse with all fields set appropriately
func NewVersionResponse(version string, serviceName string) VersionResponse {
	return VersionResponse{
		Versionable: NewVersionable(),
		Version:     version,
		ServiceName: serviceName,
	}
}
