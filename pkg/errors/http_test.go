// Copyright (C) 2023-2025 IOTech Ltd

package errors

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpError(t *testing.T) {
	tests := []struct {
		name       string
		wrappedErr error
		kind       ErrKind
		errMsg     string
	}{
		{"Wrapped error is go error", fmt.Errorf("go base error"), KindUnknown, ""},
		{"Wrapped error is BaseError", ToBaseError(fmt.Errorf("base error")), KindCommunicationError, "communication base error"},
		{"Wrapped error is BaseError with 4 error wrapped", L5Error, KindAuthenticationFailure, "http authentication failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseErr := NewBaseError(tt.kind, tt.errMsg, nil)
			httpErr := NewHTTPError(baseErr)
			expectedCode := codeMapping(tt.kind)
			assert.Equal(t, httpErr.Code(), expectedCode, fmt.Sprintf("Retrieved http status code %v is not equal to %v.", httpErr.Code(), expectedCode))
			assert.Contains(t, httpErr.Message(), strconv.Itoa(expectedCode), fmt.Sprintf("Retrieved http error message %v doesn't contain %v.", httpErr.Message(), expectedCode))
			assert.Equal(t, httpErr.Kind(), string(tt.kind), fmt.Sprintf("Retrieved http error kind %v is not equal to %v.", httpErr.Kind(), tt.kind))
		})
	}
}

func TestKindMapping(t *testing.T) {
	tests := []struct {
		name           string
		httpStatusCode int
	}{
		{fmt.Sprintf("status code: %d", http.StatusInternalServerError), http.StatusInternalServerError},
		{fmt.Sprintf("status code: %d", http.StatusBadGateway), http.StatusBadGateway},
		{fmt.Sprintf("status code: %d", http.StatusNotFound), http.StatusNotFound},
		{fmt.Sprintf("status code: %d", http.StatusBadRequest), http.StatusBadRequest},
		{fmt.Sprintf("status code: %d", http.StatusConflict), http.StatusConflict},
		{fmt.Sprintf("status code: %d", http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge},
		{fmt.Sprintf("status code: %d", http.StatusServiceUnavailable), http.StatusServiceUnavailable},
		{fmt.Sprintf("status code: %d", http.StatusLocked), http.StatusLocked},
		{fmt.Sprintf("status code: %d", http.StatusNotImplemented), http.StatusNotImplemented},
		{fmt.Sprintf("status code: %d", http.StatusMethodNotAllowed), http.StatusMethodNotAllowed},
		{fmt.Sprintf("status code: %d", http.StatusRequestedRangeNotSatisfiable), http.StatusRequestedRangeNotSatisfiable},
		{fmt.Sprintf("status code: %d", http.StatusUnauthorized), http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind := KindMapping(tt.httpStatusCode)
			code := codeMapping(kind)
			assert.Equal(t, tt.httpStatusCode, code, fmt.Sprintf("Retrieved http status code %v is not equal to %v.", tt.httpStatusCode, code))
		})
	}
}
