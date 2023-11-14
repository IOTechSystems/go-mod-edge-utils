// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/validator"
)

func TestValidate(t *testing.T) {
	validConfig := Mqtt5Config{
		Host:       "localhost",
		Port:       1883,
		Protocol:   "tcp",
		AuthMode:   "none",
		SecretName: "secret",
	}

	validIpv4Config := validConfig
	validIpv4Config.Host = "127.0.0.1"

	validIpv6Config := validConfig
	validIpv6Config.Host = "::1"

	invalidConfig := Mqtt5Config{}

	tests := []struct {
		Name          string
		Config        Mqtt5Config
		ExpectedError bool
	}{
		{"Valid Mqtt5Config", validConfig, false},
		{"Valid Mqtt5Config with IPv4", validIpv4Config, false},
		{"Valid Mqtt5Config with IPv6", validIpv6Config, false},
		{"Invalid Mqtt5Config", invalidConfig, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := validator.Validate(test.Config)
			if test.ExpectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "field is required")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
