//
// Copyright (C) 2020-2023 Intel Corporation
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0
//

package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
)

func TestNewVersionResponse(t *testing.T) {
	serviceName := uuid.NewString()

	expectedVersion := "1.2.2"
	target := NewVersionResponse(expectedVersion, serviceName)

	assert.Equal(t, common.ApiVersion, target.ApiVersion)
	assert.Equal(t, expectedVersion, target.Version)
	assert.Equal(t, serviceName, target.ServiceName)
}
