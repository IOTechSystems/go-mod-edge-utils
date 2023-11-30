/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2023 Intel Inc.
 * Copyright 2023 IOTech Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package environment

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/config"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/mocks"
)

const (
	defaultHostValue     = "defaultHost"
	defaultPortValue     = 987654321
	defaultTypeValue     = "defaultType"
	defaultProtocolValue = "defaultProtocol"
)

func initializeTest() (config.ServiceInfo, *loggerMocks.Logger) {
	os.Clearenv()
	providerConfig := config.ServiceInfo{
		Host: defaultHostValue,
		Port: defaultPortValue,
	}

	return providerConfig, &loggerMocks.Logger{}
}

func TestGetStartupInfo(t *testing.T) {
	testCases := []struct {
		TestName         string
		DurationEnvName  string
		ExpectedDuration int
		IntervalEnvName  string
		ExpectedInterval int
	}{
		{"V2 Envs", common.EnvKeyStartupDuration, 120, common.EnvKeyStartupInterval, 30},
		{"No Envs", "", bootTimeoutSecondsDefault, "", bootRetrySecondsDefault},
	}

	for _, test := range testCases {
		t.Run(test.TestName, func(t *testing.T) {
			os.Clearenv()

			if len(test.DurationEnvName) > 0 {
				err := os.Setenv(test.DurationEnvName, strconv.Itoa(test.ExpectedDuration))
				require.NoError(t, err)
			}

			if len(test.IntervalEnvName) > 0 {
				err := os.Setenv(test.IntervalEnvName, strconv.Itoa(test.ExpectedInterval))
				require.NoError(t, err)
			}

			actual := GetStartupInfo("unit-test")
			assert.Equal(t, test.ExpectedDuration, actual.Duration)
			assert.Equal(t, test.ExpectedInterval, actual.Interval)
		})
	}
}

func TestGetConfigDir(t *testing.T) {
	_, mockLogger := initializeTest()
	mockLogger.On("Infof", mock.Anything, "-cd/-configDir", common.EnvKeyConfigDir, mock.AnythingOfType("string"))

	testCases := []struct {
		TestName     string
		EnvName      string
		PassedInName string
		ExpectedName string
	}{
		{"With Env Var", common.EnvKeyConfigDir, "res", "myres"},
		{"With No Env Var", "", "res", "res"},
		{"With No Env Var and no passed in", "", "", defaultConfigDirValue},
	}

	for _, test := range testCases {
		t.Run(test.TestName, func(t *testing.T) {
			os.Clearenv()

			if len(test.EnvName) > 0 {
				err := os.Setenv(test.EnvName, test.ExpectedName)
				require.NoError(t, err)
			}

			actual := GetConfigDir(mockLogger, test.PassedInName)
			assert.Equal(t, test.ExpectedName, actual)
		})
	}
}

func TestGetConfigFileName(t *testing.T) {
	_, mockLogger := initializeTest()
	mockLogger.On("Infof", mock.Anything, "-cf/--configFile", common.EnvKeyConfigFile, mock.AnythingOfType("string"))

	testCases := []struct {
		TestName     string
		EnvName      string
		PassedInName string
		ExpectedName string
	}{
		{"With Env Var", common.EnvKeyConfigFile, "configuration.yaml", "configuration.yaml"},
		{"With No Env Var", "", "configuration.yml", "configuration.yml"},
		{"With No Env Var and no passed in", "", "", "configuration.json"},
	}

	for _, test := range testCases {
		t.Run(test.TestName, func(t *testing.T) {
			os.Clearenv()

			if len(test.EnvName) > 0 {
				err := os.Setenv(test.EnvName, test.ExpectedName)
				require.NoError(t, err)
			}

			actual := GetConfigFileName(mockLogger, test.PassedInName)
			assert.Equal(t, test.ExpectedName, actual)
		})
	}
}

func TestConvertToType(t *testing.T) {
	tests := []struct {
		Name          string
		Value         string
		OldValue      any
		ExpectedValue any
		ExpectedError string
	}{
		{Name: "String", Value: "This is string", OldValue: "string", ExpectedValue: "This is string"},
		{Name: "Valid String slice", Value: " val1 , val2 ", OldValue: []string{}, ExpectedValue: []any{"val1", "val2"}},
		{Name: "Invalid slice type", Value: "", OldValue: []int{}, ExpectedError: "'[]int' is not supported"},
		{Name: "Valid bool", Value: "true", OldValue: true, ExpectedValue: true},
		{Name: "Invalid bool", Value: "bad bool", OldValue: false, ExpectedError: "invalid syntax"},
		{Name: "Valid int", Value: "234", OldValue: 0, ExpectedValue: 234},
		{Name: "Invalid int", Value: "one", OldValue: 0, ExpectedError: "invalid syntax"},
		{Name: "Valid int8", Value: "123", OldValue: int8(0), ExpectedValue: int8(123)},
		{Name: "Invalid int8", Value: "897", OldValue: int8(0), ExpectedError: "value out of range"},
		{Name: "Valid int16", Value: "897", OldValue: int16(0), ExpectedValue: int16(897)},
		{Name: "Invalid int16", Value: "89756789", OldValue: int16(0), ExpectedError: "value out of range"},
		{Name: "Valid int32", Value: "89756789", OldValue: int32(0), ExpectedValue: int32(89756789)},
		{Name: "Invalid int32", Value: "89756789324414221", OldValue: int32(0), ExpectedError: "value out of range"},
		{Name: "Valid int64", Value: "89756789324414221", OldValue: int64(0), ExpectedValue: int64(89756789324414221)},
		{Name: "Invalid int64", Value: "one", OldValue: int64(0), ExpectedError: "invalid syntax"},
		{Name: "Valid uint", Value: "234", OldValue: uint(0), ExpectedValue: uint(234)},
		{Name: "Invalid uint", Value: "one", OldValue: uint(0), ExpectedError: "invalid syntax"},
		{Name: "Valid uint8", Value: "123", OldValue: uint8(0), ExpectedValue: uint8(123)},
		{Name: "Invalid uint8", Value: "897", OldValue: uint8(0), ExpectedError: "value out of range"},
		{Name: "Valid uint16", Value: "897", OldValue: uint16(0), ExpectedValue: uint16(897)},
		{Name: "Invalid uint16", Value: "89756789", OldValue: uint16(0), ExpectedError: "value out of range"},
		{Name: "Valid uint32", Value: "89756789", OldValue: uint32(0), ExpectedValue: uint32(89756789)},
		{Name: "Invalid uint32", Value: "89756789324414221", OldValue: uint32(0), ExpectedError: "value out of range"},
		{Name: "Valid uint64", Value: "89756789324414221", OldValue: uint64(0), ExpectedValue: uint64(89756789324414221)},
		{Name: "Invalid uint64", Value: "one", OldValue: uint64(0), ExpectedError: "invalid syntax"},
		{Name: "Valid float32", Value: "895.89", OldValue: float32(0), ExpectedValue: float32(895.89)},
		{Name: "Invalid float32", Value: "one", OldValue: float32(0), ExpectedError: "invalid syntax"},
		{Name: "Valid float64", Value: "89756789324414221.5689", OldValue: float64(0), ExpectedValue: 89756789324414221.5689},
		{Name: "Invalid float64", Value: "one", OldValue: float64(0), ExpectedError: "invalid syntax"},
		{Name: "Invalid Value Type", Value: "anything", OldValue: make(chan int), ExpectedError: "type of 'chan int' is not supported"},
	}

	env := Variables{}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			actual, err := env.convertToType(test.OldValue, test.Value)
			if len(test.ExpectedError) > 0 {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.ExpectedError)
				return // test complete
			}

			require.NoError(t, err)
			assert.Equal(t, test.ExpectedValue, actual)
		})
	}
}

func TestLogEnvironmentOverride(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		value    string
		redacted bool
	}{
		{
			name:     "basic variable - not redacted",
			path:     "LogLevel",
			value:    "DEBUG",
			redacted: false,
		},
		{
			name:     "insecure secret value - redacted",
			path:     "InsecureSecrets.credentials001.secretData.password",
			value:    "HelloWorld!",
			redacted: true,
		},
		{
			name:     "insecure secret value - redacted 2",
			path:     "InsecureSecrets.credentials001.secretData.username",
			value:    "admin",
			redacted: true,
		},
		{
			name:     "insecure secret name - not redacted",
			path:     "InsecureSecrets.credentials001.secretName",
			value:    "credentials001",
			redacted: false,
		},
	}

	mockLogger := &loggerMocks.Logger{}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			key := strings.ReplaceAll(strings.ToUpper(test.path), ".", "_")

			// specifically expect the method to be called with the values we pass in plus the format string
			// and any value (can be redacted or not)
			mockLogger.On("Infof", mock.AnythingOfType("string"),
				test.path, key, mock.AnythingOfType("string")).Return().Once()

			logEnvironmentOverride(mockLogger, test.path, key, test.value)

			mockLogger.AssertExpectations(t)
			if test.redacted {
				// make sure it was called with the redacted placeholder string.
				mockLogger.AssertCalled(t, "Infof", mock.AnythingOfType("string"), test.path, key, redactedStr)
			} else {
				// make sure the original value was logged.
				mockLogger.AssertCalled(t, "Infof", mock.AnythingOfType("string"), test.path, key, test.value)
			}
		})
	}
}

func TestOverrideConfigMapValues(t *testing.T) {
	flatMap := map[string]any{
		"top":             "top value",
		"some/value":      "my string",
		"some/thing/here": 123,
		"my/other/value":  12.89,
	}

	nonFlatMap := map[string]any{
		"top": "top value",
		"some": map[string]any{
			"value": "my string",
			"thing": map[string]any{
				"here": 123,
			},
		},
		"my": map[string]any{
			"other": map[string]any{
				"value": 12.89,
			},
		},
	}

	tests := []struct {
		Name      string
		ConfigMap map[string]any
	}{
		{"Flat Map", flatMap},
		{"Non Flat Map", nonFlatMap},
	}

	expectedCount := 4
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			defer os.Clearenv()
			os.Setenv("TOP", "new top value")
			os.Setenv("SOME_VALUE", "your string")
			os.Setenv("SOME_THING_HERE", "321")
			os.Setenv("MY_OTHER_VALUE", "89.12")

			mockLogger := &loggerMocks.Logger{}
			mockLogger.On("Infof", mock.Anything, "top", "TOP", "new top value")
			mockLogger.On("Infof", mock.Anything, "some/value", "SOME_VALUE", "your string")
			mockLogger.On("Infof", mock.Anything, "some/thing/here", "SOME_THING_HERE", "321")
			mockLogger.On("Infof", mock.Anything, "my/other/value", "MY_OTHER_VALUE", "89.12")
			target := NewVariables(mockLogger)

			actualCount, err := target.OverrideConfigMapValues(test.ConfigMap)
			require.NoError(t, err)
			assert.Equal(t, expectedCount, actualCount)
			mockLogger.AssertExpectations(t)
		})
	}
}
