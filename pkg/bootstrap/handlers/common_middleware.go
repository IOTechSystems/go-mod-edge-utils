//
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/rest"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func ManageHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		correlationID := r.Header.Get(common.CorrelationID)
		if correlationID == "" {
			correlationID = uuid.New().String()
		}
		// lint:ignore SA1029 legacy
		// nolint:staticcheck // See golangci-lint #741
		ctx := context.WithValue(r.Context(), common.CorrelationID, correlationID)

		contentType := r.Header.Get(common.ContentType)
		// lint:ignore SA1029 legacy
		// nolint:staticcheck // See golangci-lint #741
		ctx = context.WithValue(ctx, common.ContentType, contentType)

		c.SetRequest(r.WithContext(ctx))

		return next(c)
	}
}

func LoggingMiddleware(logger log.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if logger.LogLevel() == log.TraceLog {
				r := c.Request()
				begin := time.Now()
				correlationId := rest.FromContext(r.Context(), common.CorrelationID)
				logger.Trace("Begin request", common.CorrelationID, correlationId, "path", r.URL.Path)
				err := next(c)
				if err != nil {
					logger.Errorf("failed to add the middleware: %v", err)
					return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
				}
				logger.Trace("Response complete", common.CorrelationID, correlationId, "duration", time.Since(begin).String())
				return nil
			}
			return next(c)
		}
	}
}

// RequestLimitMiddleware is a middleware function that limits the request body size to Service.MaxRequestSize in kilobytes
func RequestLimitMiddleware(sizeLimit int64, logger log.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			r := c.Request()
			w := c.Response()
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				if sizeLimit > 0 && r.ContentLength > sizeLimit*1024 {
					response := models.NewBaseResponse("", fmt.Sprintf("request size exceed Service.MaxRequestSize(%d KB)", sizeLimit), http.StatusRequestEntityTooLarge)
					logger.Error(response.Message)

					w.Header().Set(common.ContentType, common.ContentTypeJSON)
					w.WriteHeader(response.StatusCode)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						logger.Errorf("Error encoding the data:  %v", err)
						// set Response.Committed to true in order to rewrite the status code
						w.Committed = false
						return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
					}
				}
			}
			return next(c)
		}
	}
}
