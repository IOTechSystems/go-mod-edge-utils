/*******************************************************************************
 * Copyright (C) 2023 Intel Corp.
 * Copyright (C) 2023 IOTech Ltd
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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/models"
)

// ConvertFromMap uses json to marshal and unmarshal a map into a target type
func ConvertFromMap(m map[string]any, target any) error {
	jsonBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("could not marshal map to JSON: %v", err)
	}

	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("could not unmarshal JSON to %T: %v", target, err)
	}

	return nil
}

// SendJsonResp puts together the response packet for the APIs
func SendJsonResp(
	logger log.Logger,
	writer *echo.Response,
	request *http.Request,
	response interface{},
	statusCode int) error {

	correlationID := request.Header.Get(common.CorrelationID)

	writer.Header().Set(common.CorrelationID, correlationID)
	writer.Header().Set(common.ContentType, common.ContentTypeJSON)
	// when the request destination  server is shut down or unreachable
	// the statusCode in the response header  would be  zero .
	// http.ResponseWriter.WriteHeader will check statusCode,if less than 100 or bigger than 900,
	// when this check not pass would raise a panic, response to the caller can not be completed
	// to avoid panic see http.checkWriteHeaderCode
	if statusCode < 100 || statusCode > 900 {
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.WriteHeader(statusCode)
	}

	if response != nil {
		enc := json.NewEncoder(writer)
		err := enc.Encode(response)
		if err != nil {
			logger.Errorf("Error encoding the data: %v, correlation id: %s"+err.Error(), correlationID)
			// set Response.Committed to false in order to rewrite the status code
			writer.Committed = false
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return nil
}

// SendJsonErrResp puts together the error response packet for the APIs
func SendJsonErrResp(
	logger log.Logger,
	writer *echo.Response,
	request *http.Request,
	errKind errors.ErrKind,
	message string,
	err error,
	requestID string) error {

	httpErr := errors.NewHTTPError(errors.NewBaseError(errKind, message, err, nil))
	logger.Error(httpErr.Error())
	logger.Debug(httpErr.DebugMessages())
	response := models.NewBaseResponse(requestID, httpErr.Message(), httpErr.Code())
	return SendJsonResp(logger, writer, request, response, httpErr.Code())
}
