//
// Copyright (C) 2020 Intel Corporation
// Copyright (C) 2021-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0
//

package models

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
)

func TestNewConfigResponse(t *testing.T) {
	serviceName := uuid.NewString()

	type testConfig struct {
		Name string
		Host string
		Port int
	}

	expected := testConfig{
		Name: "UnitTest",
		Host: "localhost",
		Port: 8080,
	}

	target := NewConfigResponse(expected, serviceName)

	assert.Equal(t, common.ApiVersion, target.ApiVersion)
	assert.Equal(t, serviceName, target.ServiceName)

	data, _ := json.Marshal(target.Config)
	actual := testConfig{}
	err := json.Unmarshal(data, &actual)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}
