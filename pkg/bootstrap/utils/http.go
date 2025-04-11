//
// Copyright (C) 2020-2025 IOTech Ltd
// Copyright (C) 2023 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/contracts/clients/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// PutRequest makes the put JSON request and return the body
func PutRequest(
	ctx context.Context,
	returnValuePointer interface{},
	baseUrl string, requestPath string,
	requestParams url.Values,
	data interface{}, authInjector interfaces.AuthenticationInjector) error {

	req, err := CreateRequestWithRawData(ctx, http.MethodPut, baseUrl, requestPath, requestParams, data)
	if err != nil {
		return err
	}

	return processRequest(ctx, returnValuePointer, req, authInjector)
}

// processRequest is a helper function to process the request and get the return value
func processRequest(ctx context.Context,
	returnValuePointer any, req *http.Request, authInjector interfaces.AuthenticationInjector) error {
	resp, err := SendRequest(ctx, req, authInjector)
	if err != nil {
		return err
	}
	// Check the response content length to avoid json unmarshal error
	if len(resp) == 0 {
		return nil
	}
	if err := json.Unmarshal(resp, returnValuePointer); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to parse the response body", err)
	}
	return nil
}

func CreateRequest(ctx context.Context, httpMethod string, baseUrl string, requestPath string, requestParams url.Values) (*http.Request, error) {
	u, err := parseBaseUrlAndRequestPath(baseUrl, requestPath)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to parse baseUrl and requestPath", err)
	}
	if requestParams != nil {
		u.RawQuery = requestParams.Encode()
	}
	req, err := http.NewRequest(httpMethod, u.String(), nil)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to create a http request", err)
	}
	req.Header.Set(common.CorrelationHeader, correlatedId(ctx))
	return req, nil
}

func parseBaseUrlAndRequestPath(baseUrl, requestPath string) (*url.URL, error) {
	fullPath, err := url.JoinPath(baseUrl, requestPath)
	if err != nil {
		return nil, err
	}
	return url.Parse(fullPath)
}

// correlatedId gets Correlation ID from supplied context. If no Correlation ID header is
// present in the supplied context, one will be created along with a value.
func correlatedId(ctx context.Context) string {
	correlation := FromContext(ctx, common.CorrelationHeader)
	if len(correlation) == 0 {
		correlation = uuid.New().String()
	}
	return correlation
}

// FromContext allows for the retrieval of the specified key's value from the supplied Context.
// If the value is not found, an empty string is returned.
func FromContext(ctx context.Context, key string) string {
	hdr, ok := ctx.Value(key).(string)
	if !ok {
		hdr = ""
	}
	return hdr
}

// SendRequest will make a request with raw data to the specified URL.
// It returns the body as a byte array if successful and an error otherwise.
func SendRequest(ctx context.Context, req *http.Request, authInjector interfaces.AuthenticationInjector) ([]byte, error) {
	resp, err := makeRequest(req, authInjector)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := getBody(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode <= http.StatusMultiStatus {
		return bodyBytes, nil
	}

	var errMsg string
	var errResp models.BaseResponse
	// If the bodyBytes can be unmarshalled to BaseResponse DTO, use the BaseResponse.Message field as the error message
	// Otherwise, use the whole bodyBytes string as the error message
	baseRespErr := json.Unmarshal(bodyBytes, &errResp)
	if baseRespErr == nil {
		errMsg = errResp.Message
	} else {
		errMsg = string(bodyBytes)
	}

	// Handle error response
	msg := fmt.Sprintf("request failed, status code: %d, err: %s", resp.StatusCode, errMsg)
	return bodyBytes, echo.NewHTTPError(resp.StatusCode, msg)
}

// Helper method to make the request and return the response
func makeRequest(req *http.Request, authInjector interfaces.AuthenticationInjector) (*http.Response, error) {
	if authInjector != nil {
		if err := authInjector.AddAuthenticationData(req); err != nil {
			return nil, err
		}
	}

	client := &http.Client{Transport: authInjector.RoundTripper()}

	resp, err := client.Do(req)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "failed to send a http request", err)
	}
	if resp == nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "the response should not be a nil")
	}
	return resp, nil
}

// Helper method to get the body from the response after making the request
func getBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return body, echo.NewHTTPError(http.StatusRequestedRangeNotSatisfiable, "failed to read the response body", err)
	}
	return body, nil
}

func CreateRequestWithRawData(ctx context.Context, httpMethod string, baseUrl string, requestPath string, requestParams url.Values, data interface{}) (*http.Request, error) {
	u, err := parseBaseUrlAndRequestPath(baseUrl, requestPath)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to parse baseUrl and requestPath", err)
	}
	if requestParams != nil {
		u.RawQuery = requestParams.Encode()
	}

	jsonEncodedData, err := json.Marshal(data)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "failed to encode input data to JSON", err)
	}

	content := FromContext(ctx, common.ContentType)
	if content == "" {
		content = common.ContentTypeJSON
	}

	req, err := http.NewRequest(httpMethod, u.String(), bytes.NewReader(jsonEncodedData))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to create a http request", err)
	}
	req.Header.Set(common.ContentType, content)
	req.Header.Set(common.CorrelationHeader, correlatedId(ctx))
	return req, nil
}
