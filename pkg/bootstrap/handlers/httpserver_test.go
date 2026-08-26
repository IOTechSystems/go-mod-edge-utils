//
// Copyright (C) 2022-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/stretchr/testify/mock"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log/mocks"
)

func TestRequestLimitMiddleware(t *testing.T) {
	e := echo.New()
	mockLogger := &loggerMocks.Logger{}
	mockLogger.On("Error", mock.AnythingOfType("string")).Return().Once()
	payload := make([]byte, 2048)

	tests := []struct {
		name          string
		sizeLimit     int64
		errorExpected bool
	}{
		{"Valid unlimited size", int64(0), false},
		{"Valid size", int64(2), false},
		{"Invalid size", int64(1), true},
	}

	for _, testCase := range tests {
		handlerCalled := false
		middleware := RequestLimitMiddleware(testCase.sizeLimit, mockLogger)
		handler := middleware(func(c echo.Context) error {
			handlerCalled = true
			c.Response().WriteHeader(http.StatusOK)
			return nil
		})

		reader := strings.NewReader(string(payload))
		req, err := http.NewRequest(http.MethodPost, "/", reader)
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		c := e.NewContext(req, recorder)
		err = handler(c)
		assert.NoError(t, err)

		resp := recorder.Result()

		if testCase.errorExpected {
			var res models.BaseResponse
			err = json.Unmarshal(recorder.Body.Bytes(), &res)
			require.NoError(t, err)

			assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "http status code is not as expected")
			assert.Equal(t, common.ContentTypeJSON, resp.Header.Get(common.ContentType), "http header Content-Type is not as expected")
			assert.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode, "Response status code not as expected")
			assert.NotEmpty(t, res.Message, "Response message doesn't contain the error message")
			assert.False(t, handlerCalled, "downstream handler should not be invoked for an oversized request")
		} else {
			assert.True(t, handlerCalled, "downstream handler should be invoked for a request within the limit")
		}
	}
}

// TestRequestLimitMiddleware_SpoofedContentLength verifies that a request advertising a
// Content-Length within the limit but sending a larger body cannot slip past the limit,
// because the limit is also enforced while the body is read.
func TestRequestLimitMiddleware_SpoofedContentLength(t *testing.T) {
	e := echo.New()
	mockLogger := &loggerMocks.Logger{}
	mockLogger.On("Error", mock.AnythingOfType("string")).Return()

	// 2 KB body against a 1 KB limit.
	payload := make([]byte, 2048)
	middleware := RequestLimitMiddleware(int64(1), mockLogger)

	handler := middleware(func(c echo.Context) error {
		// A real handler reads the body (e.g. binding JSON); reading past the limit
		// is what triggers http.MaxBytesReader.
		_, err := io.ReadAll(c.Request().Body)
		return err
	})

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	require.NoError(t, err)
	// Lie about the size: advertise 512 bytes (within the 1 KB limit) while the actual
	// body is 2 KB, so the Content-Length fast path is bypassed.
	req.ContentLength = 512

	recorder := httptest.NewRecorder()
	c := e.NewContext(req, recorder)
	err = handler(c)
	assert.NoError(t, err)

	resp := recorder.Result()
	var res models.BaseResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &res))

	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode, "spoofed Content-Length request should be rejected with 413")
	assert.Equal(t, common.ContentTypeJSON, resp.Header.Get(common.ContentType), "http header Content-Type is not as expected")
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode, "Response status code not as expected")
	assert.NotEmpty(t, res.Message, "Response message doesn't contain the error message")
}
