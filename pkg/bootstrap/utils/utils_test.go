/*******************************************************************************
 * Copyright 2023 Intel Corp.
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

package utils

import (
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/config"
)

type ConfigurationMockStruct struct {
	LogLevel string
	Service  config.ServiceInfo
	Trigger  TriggerInfo
}

type TriggerInfo struct {
	Type string
}

func TestMergeMaps(t *testing.T) {
	expectedTriggerType := "edge-messagebus"
	expectedHost := "localhost"
	expectedLogLevel := log.InfoLog

	destMap := map[string]any{
		"loglevel": expectedLogLevel,
		"service": config.ServiceInfo{
			Host: expectedHost,
		},
		"trigger": TriggerInfo{
			expectedTriggerType,
		},
	}

	actualConfig := &ConfigurationMockStruct{}

	err := ConvertFromMap(destMap, actualConfig)
	require.NoError(t, err)
	assert.Equal(t, expectedLogLevel, actualConfig.LogLevel)
	assert.Equal(t, expectedTriggerType, actualConfig.Trigger.Type)
	assert.Equal(t, expectedHost, actualConfig.Service.Host)
}
