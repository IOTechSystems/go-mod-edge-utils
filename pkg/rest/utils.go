//
// Copyright (C) 2024 IOTech Ltd
//

package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/handlers"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

// Versionable shows the API version in DTOs
type Versionable struct {
	ApiVersion string `json:"apiVersion" validate:"required"`
}

// BaseResponse defines the base content for response DTOs (data transfer objects).
type BaseResponse struct {
	Versionable `json:",inline"`
	RequestId   string `json:"requestId,omitempty"`
	Message     string `json:"message,omitempty"`
	StatusCode  int    `json:"statusCode"`
}

func WriteDefaultHttpHeader(w http.ResponseWriter, ctx context.Context, statusCode int) {
	w.Header().Set(common.CorrelationID, handlers.FromContext(ctx))
	w.Header().Set(common.ContentType, common.ContentTypeJSON)
	w.WriteHeader(statusCode)
}

func WriteHttpContentTypeHeader(w http.ResponseWriter, ctx context.Context, statusCode int, contentType string) {
	w.Header().Set(common.CorrelationID, handlers.FromContext(ctx))
	w.Header().Set(common.ContentType, contentType)
	w.WriteHeader(statusCode)
}

func NewVersionable() Versionable {
	return Versionable{ApiVersion: common.ApiVersion}
}

func NewBaseResponse(requestId string, message string, statusCode int) BaseResponse {
	return BaseResponse{
		Versionable: NewVersionable(),
		RequestId:   requestId,
		Message:     message,
		StatusCode:  statusCode,
	}
}

// WriteErrorResponse writes Http header, encode error response with JSON format and writes to the HTTP response.
func WriteErrorResponse(w *echo.Response, ctx context.Context, lc log.Logger, err errors.Error, requestId string) error {
	correlationId := handlers.FromContext(ctx)
	if errors.Kind(err) == errors.KindServiceUnavailable {
		lc.Warn(err.Message())
	} else if errors.Kind(err) != errors.KindEntityDoesNotExist {
		lc.Error(err.Error(), common.CorrelationID, correlationId)
	}
	lc.Debug(err.DebugMessages(), common.CorrelationID, correlationId)
	httpErr := errors.NewHTTPError(err)
	errResponses := NewBaseResponse(requestId, err.Error(), httpErr.Code())
	WriteDefaultHttpHeader(w, ctx, httpErr.Code())
	return EncodeAndWriteResponse(errResponses, w, lc)
}

func EncodeAndWriteResponse(i any, w *echo.Response, lc log.Logger) error {
	w.Header().Set(common.ContentType, common.ContentTypeJSON)

	enc := json.NewEncoder(w)
	err := enc.Encode(i)

	// Problems encoding
	if err != nil {
		lc.Error("Error encoding the data: " + err.Error())
		// set Response.Committed to false in order to rewrite the status code
		w.Committed = false
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return nil
}
