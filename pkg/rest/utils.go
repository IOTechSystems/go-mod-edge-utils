//
// Copyright (C) 2024-2025 IOTech Ltd
//

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	goErr "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/contracts/clients/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"
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
	w.Header().Set(common.CorrelationID, FromContext(ctx, common.CorrelationID))
	w.Header().Set(common.ContentType, common.ContentTypeJSON)
	w.WriteHeader(statusCode)
}

func WriteHttpContentTypeHeader(w http.ResponseWriter, ctx context.Context, statusCode int, contentType string) {
	w.Header().Set(common.CorrelationID, FromContext(ctx, common.CorrelationID))
	w.Header().Set(common.ContentType, contentType)
	w.WriteHeader(statusCode)
}

func NewVersionable() Versionable {
	return Versionable{ApiVersion: common.ApiVersion}
}

func NewBaseResponse(apiVersion, requestId, message string, statusCode int) BaseResponse {
	v := Versionable{ApiVersion: apiVersion}
	if v.ApiVersion == "" {
		v = NewVersionable()
	}
	return BaseResponse{
		Versionable: v,
		RequestId:   requestId,
		Message:     message,
		StatusCode:  statusCode,
	}
}

// WriteErrorResponse writes Http header, encode error response with JSON format and writes to the HTTP response.
func WriteErrorResponse(w *echo.Response, ctx context.Context, lc log.Logger, err errors.Error, apiVersion, requestId string) error {
	correlationId := FromContext(ctx, common.CorrelationID)
	if err.Kind() == string(errors.KindServiceUnavailable) {
		lc.Warn(err.Message())
	} else if err.Kind() != string(errors.KindEntityDoesNotExist) {
		lc.Error(err.Error(), common.CorrelationID, correlationId)
	}

	lc.Debug(err.DebugMessages(), common.CorrelationID, correlationId)

	var (
		e    errors.BaseError
		code int
	)
	if goErr.As(err, &e) {
		httpErr := errors.NewHTTPError(e)
		code = httpErr.Code()
	} else {
		code = err.Code()
	}

	errResponses := NewBaseResponse(apiVersion, requestId, err.Error(), code)
	WriteDefaultHttpHeader(w, ctx, code)
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

func EncodeAndWriteYamlResponse(i interface{}, w *echo.Response, lc log.Logger) error {
	enc := yaml.NewEncoder(w)
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

// ParseGetAllObjectsRequestQueryString parses offset, limit, and labels from the query parameters. And check that the offset and limit values are within the valid range when needed.
func ParseGetAllObjectsRequestQueryString(r *http.Request, maxOffSet, maxResultCount int) (offset int, limit int, labels []string, err errors.Error) {
	offset, err = parseQueryStringToInt(r, common.Offset, 0)
	if err != nil {
		return offset, limit, labels, err
	}
	if maxOffSet > 0 {
		if err = checkValueRange(common.Offset, offset, 0, maxOffSet); err != nil {
			return
		}
	}

	limit, err = parseQueryStringToInt(r, common.Limit, 20)
	if err != nil {
		return offset, limit, labels, err
	}
	if maxResultCount > 0 {
		if err = checkValueRange(common.Limit, limit, -1, maxResultCount); err != nil {
			return
		}
	}

	labels = parseQueryStringToStrings(r, common.Labels, common.CommaSeparator)

	return offset, limit, labels, err
}

func ParseStartEndRequestQueryString(r *http.Request) (start, end int64, err errors.Error) {
	start, parseErr := parseQueryStringToInt64(r, common.Start, 0)
	if parseErr != nil {
		err = errors.NewBaseError(errors.KindContractInvalid, "unable to convert 'start' value to int", parseErr, nil)
	}
	end, parseErr = parseQueryStringToInt64(r, common.End, time.Now().UnixMilli())
	if parseErr != nil {
		err = errors.NewBaseError(errors.KindContractInvalid, "unable to convert 'end' value to int", parseErr, nil)
	}

	return start, end, err
}

func ParseQueryStringToString(r *http.Request, queryStringKey string, defaultValue string) string {
	value := r.URL.Query().Get(queryStringKey)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

// ParseGetLogsRequestQueryString parses since, until, tail and timestamps from the query parameters.
func ParseGetLogsRequestQueryString(r *http.Request) (since int, until int, tail int, timestamps bool, err errors.Error) {
	since, err = parseQueryStringToInt(r, common.Since, 0)
	if err != nil {
		return since, until, tail, timestamps, err
	}

	until, err = parseQueryStringToInt(r, common.Until, 0)
	if err != nil {
		return since, until, tail, timestamps, err
	}

	tail, err = parseQueryStringToInt(r, common.Tail, 200)
	if err != nil {
		return since, until, tail, timestamps, err
	}

	timestamps, err = parseQueryStringToBool(r, common.Timestamps)
	if err != nil {
		return since, until, tail, timestamps, err
	}

	return since, until, tail, timestamps, nil
}

func parseQueryStringToStrings(r *http.Request, queryStringKey string, separator string) (stringArray []string) {
	if len(separator) == 0 {
		separator = common.CommaSeparator
	}

	value := r.URL.Query().Get(queryStringKey)
	if len(value) > 0 {
		stringArray = strings.Split(strings.TrimSpace(value), separator)
	}

	return stringArray
}

func parseQueryStringToInt(r *http.Request, queryStringKey string, defaultValue int) (int, errors.Error) {
	var result = defaultValue
	var parsingErr error

	value := r.URL.Query().Get(queryStringKey)
	if len(value) > 0 {
		result, parsingErr = strconv.Atoi(strings.TrimSpace(value))
		if parsingErr != nil {
			return 0, errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("failed to parse querystring %s's value %s into integer. Error:%s", queryStringKey, value, parsingErr.Error()), nil, nil)
		}
	}

	return result, nil
}

func parseQueryStringToInt64(r *http.Request, queryStringKey string, defaultValue int64) (int64, errors.Error) {
	var result = defaultValue
	var parsingErr error

	value := r.URL.Query().Get(queryStringKey)
	if value != "" {
		result, parsingErr = strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if parsingErr != nil {
			return 0, errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("failed to parse querystring %s's value %s into int64. Error:%s", queryStringKey, value, parsingErr.Error()), nil, nil)
		}
	}
	return result, nil
}

func parseQueryStringToBool(r *http.Request, queryStringKey string) (bool, errors.Error) {
	var result bool
	var parsingErr error
	param := r.URL.Query().Get(queryStringKey)

	if param != "" {
		result, parsingErr = strconv.ParseBool(strings.TrimSpace(param))
		if parsingErr != nil {
			return false, errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("failed to parse querystring %s into bool. Error:%s", queryStringKey, parsingErr.Error()), nil, nil)
		}
	}
	return result, nil
}

func checkValueRange(name string, value, min, max int) errors.Error {
	// first check if specified min is bigger than max, throw error for such case
	if min > max {
		return errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("specified min %v is bigger than specified max %v", min, max), nil, nil)
	}

	if value < min || value > max {
		return errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("querystring %s's value %v is out of min %v ~ max %v range.", name, value, min, max), nil, nil)
	}

	return nil
}

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
