// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"github.com/stretchr/testify/mock"
	"testing"

	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/config"
	"github.com/stretchr/testify/assert"
)

func TestValidateMqtt5Config(t *testing.T) {
	mockLogger := &loggerMocks.Logger{}
	testConfigName := "testConfig"
	validConfig := config.Mqtt5Config{
		Host:       "localhost",
		Port:       1883,
		Protocol:   "tcp",
		AuthMode:   "none",
		SecretName: "secret",
	}
	invalidConfig := config.Mqtt5Config{}
	missingConfig := []string{"Host", "Port", "Protocol", "AuthMode", "SecretName"}
	tests := []struct {
		Name           string
		Config         config.Mqtt5Config
		ExpectedResult bool
	}{
		{"Valid Mqtt5Config", validConfig, true},
		{"Invalid Mqtt5Config", invalidConfig, false},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if !test.ExpectedResult {
				mockLogger.On("Errorf", mock.Anything, missingConfig, testConfigName)

			}
			actual := validateMqtt5Config(testConfigName, test.Config, mockLogger)

			assert.Equal(t, test.ExpectedResult, actual)
		})
	}
}
