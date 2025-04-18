//
// Copyright (C) 2020-2023 IOTech Ltd
// Copyright (C) 2020 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package models

// ConfigResponse defines the configuration for the targeted service.
type ConfigResponse struct {
	Versionable `json:",inline"`
	Config      any    `json:"config"`
	ServiceName string `json:"serviceName"`
}

// NewConfigResponse creates new ConfigResponse with all fields set appropriately
func NewConfigResponse(serviceConfig any, serviceName string) ConfigResponse {
	return ConfigResponse{
		Versionable: NewVersionable(),
		Config:      serviceConfig,
		ServiceName: serviceName,
	}
}
