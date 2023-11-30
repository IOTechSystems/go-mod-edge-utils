//
// Copyright (C) 2020 Intel Corporation
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0
//

package models

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/stretchr/testify/assert"
)

func TestNewVersionable(t *testing.T) {
	actual := NewVersionable()
	assert.Equal(t, common.ApiVersion, actual.ApiVersion)
}

func TestNewBaseRequest(t *testing.T) {
	actual := NewBaseRequest()
	assert.Equal(t, common.ApiVersion, actual.ApiVersion)
	assert.NotEmpty(t, actual.RequestId)
}

func TestNewBaseResponse(t *testing.T) {
	expectedRequestId := "123456"
	expectedStatusCode := 200
	expectedMessage := "unit test message"
	actual := NewBaseResponse(expectedRequestId, expectedMessage, expectedStatusCode)

	assert.Equal(t, expectedRequestId, actual.RequestId)
	assert.Equal(t, expectedStatusCode, actual.StatusCode)
	assert.Equal(t, expectedMessage, actual.Message)
}

func TestBaseResponse_Marshal(t *testing.T) {
	expectedRequestId := "123456"
	expectedStatusCode := 200
	expectedMessage := "unit test message"
	response := NewBaseResponse(expectedRequestId, expectedMessage, expectedStatusCode)
	expectedResponseJsonStr := fmt.Sprintf(
		`{"apiVersion":"%s","requestId":"%s","message":"%s","statusCode":%d}`,
		response.ApiVersion, response.RequestId, response.Message, response.StatusCode)
	noRequestId := NewBaseResponse("", expectedMessage, expectedStatusCode)
	expectedNoRequestIdJsonStr := fmt.Sprintf(
		`{"apiVersion":"%s","message":"%s","statusCode":%d}`,
		noRequestId.ApiVersion, noRequestId.Message, noRequestId.StatusCode)
	noMessage := NewBaseResponse(expectedRequestId, "", expectedStatusCode)
	expectedNoMessageJsonStr := fmt.Sprintf(
		`{"apiVersion":"%s","requestId":"%s","statusCode":%d}`,
		noMessage.ApiVersion, noMessage.RequestId, noMessage.StatusCode)

	tests := []struct {
		name     string
		data     BaseResponse
		expected string
	}{
		{"JSON marshal base response", response, expectedResponseJsonStr},
		{"JSON marshal base response, no requestId", noRequestId, expectedNoRequestIdJsonStr},
		{"JSON marshal base response, no message", noMessage, expectedNoMessageJsonStr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
