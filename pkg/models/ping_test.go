//
// Copyright (C) 2020 Intel Corporation
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0
//

package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
)

func TestNewPingResponse(t *testing.T) {
	serviceName := uuid.NewString()
	target := NewPingResponse(serviceName)

	assert.Equal(t, common.ApiVersion, target.ApiVersion)
	_, err := time.Parse(time.UnixDate, target.Timestamp)
	assert.NoError(t, err)
	assert.Equal(t, serviceName, target.ServiceName)
}
