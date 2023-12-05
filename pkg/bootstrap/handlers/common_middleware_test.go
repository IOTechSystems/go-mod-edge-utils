//
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/mocks"
)

var expectedCorrelationId = "927e91d3-864c-4c26-852d-b68c39492d14"

var handler = func(c echo.Context) error {
	return c.String(http.StatusOK, "OK")
}

func TestManageHeader(t *testing.T) {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		c.Response().Header().Set(common.CorrelationID, c.Request().Context().Value(common.CorrelationID).(string))
		c.Response().Header().Set(common.ContentType, c.Request().Context().Value(common.ContentType).(string))
		c.Response().WriteHeader(http.StatusOK)
		return nil
	})
	e.Use(ManageHeader)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(common.CorrelationID, expectedCorrelationId)
	expectedContentType := common.ContentTypeJSON
	req.Header.Set(common.ContentType, expectedContentType)
	res := httptest.NewRecorder()
	e.ServeHTTP(res, req)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, expectedCorrelationId, res.Header().Get(common.CorrelationID))
	assert.Equal(t, expectedContentType, res.Header().Get(common.ContentType))
}

func TestLoggingMiddleware(t *testing.T) {
	e := echo.New()
	e.GET("/", handler)
	mockLogger := &loggerMocks.Logger{}
	mockLogger.On("Trace", "Begin request", common.CorrelationID, expectedCorrelationId, "path", "/")
	mockLogger.On("Trace", "Response complete", common.CorrelationID, expectedCorrelationId, "duration", mock.Anything)
	mockLogger.On("LogLevel").Return("TRACE")
	e.Use(LoggingMiddleware(mockLogger))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// lint:ignore SA1029 legacy
	// nolint:staticcheck // See golangci-lint #741
	ctx := context.WithValue(req.Context(), common.CorrelationID, expectedCorrelationId)
	req = req.WithContext(ctx)
	res := httptest.NewRecorder()
	e.ServeHTTP(res, req)

	mockLogger.AssertCalled(t, "Trace", "Begin request", common.CorrelationID, expectedCorrelationId, "path", "/")
	mockLogger.AssertCalled(t, "Trace", "Response complete", common.CorrelationID, expectedCorrelationId, "duration", mock.Anything)
	assert.Equal(t, http.StatusOK, res.Code)
}
