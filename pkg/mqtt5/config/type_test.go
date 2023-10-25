// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	//mockLogger := &loggerMocks.Logger{}
	//testConfigName := "testConfig"
	validConfig := Mqtt5Config{
		Host:       "localhost",
		Port:       1883,
		Protocol:   "tcp",
		AuthMode:   "none",
		SecretName: "secret",
	}
	invalidConfig := Mqtt5Config{}
	//missingConfig := []string{"Host", "Port", "Protocol", "AuthMode", "SecretName"}
	tests := []struct {
		Name          string
		Config        Mqtt5Config
		ExpectedError bool
	}{
		{"Valid Mqtt5Config", validConfig, false},
		{"Invalid Mqtt5Config", invalidConfig, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := Validate(test.Config)
			if test.ExpectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "field is required")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
