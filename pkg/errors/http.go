// Copyright (C) 2023-2025 IOTech Ltd

package errors

import (
	"fmt"
	"net/http"
)

type HTTPError struct {
	BaseError
	httpStatusCode int
}

func NewHTTPError(wrappedError error) HTTPError {
	kind := Kind(wrappedError)
	baseError := NewBaseError(kind, "", wrappedError)
	return HTTPError{
		BaseError:      baseError,
		httpStatusCode: codeMapping(kind),
	}
}

func (he HTTPError) Message() string {
	return fmt.Sprintf("%s(status code:%d)", he.BaseError.Message(), he.httpStatusCode)
}

// Code returns the status code of this error.
func (he HTTPError) Code() int {
	return he.httpStatusCode
}

// codeMapping determines the correct HTTP response code for the given error kind.
func codeMapping(kind ErrKind) int {
	switch kind {
	case KindUnknown, KindDatabaseError, KindServerError, KindOverflowError, KindNaNError:
		return http.StatusInternalServerError
	case KindCommunicationError:
		return http.StatusBadGateway
	case KindEntityDoesNotExist:
		return http.StatusNotFound
	case KindContractInvalid, KindInvalidId:
		return http.StatusBadRequest
	case KindStatusConflict, KindDuplicateName:
		return http.StatusConflict
	case KindLimitExceeded:
		return http.StatusRequestEntityTooLarge
	case KindServiceUnavailable:
		return http.StatusServiceUnavailable
	case KindServiceLocked:
		return http.StatusLocked
	case KindNotImplemented:
		return http.StatusNotImplemented
	case KindNotAllowed:
		return http.StatusMethodNotAllowed
	case KindRangeNotSatisfiable:
		return http.StatusRequestedRangeNotSatisfiable
	case KindIOError:
		return http.StatusForbidden
	case KindUnauthorized:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// KindMapping determines the correct error kind for the given HTTP response code.
func KindMapping(code int) ErrKind {
	switch code {
	case http.StatusInternalServerError:
		return KindServerError
	case http.StatusBadGateway:
		return KindCommunicationError
	case http.StatusNotFound:
		return KindEntityDoesNotExist
	case http.StatusBadRequest:
		return KindContractInvalid
	case http.StatusConflict:
		return KindStatusConflict
	case http.StatusRequestEntityTooLarge:
		return KindLimitExceeded
	case http.StatusServiceUnavailable:
		return KindServiceUnavailable
	case http.StatusLocked:
		return KindServiceLocked
	case http.StatusNotImplemented:
		return KindNotImplemented
	case http.StatusMethodNotAllowed:
		return KindNotAllowed
	case http.StatusRequestedRangeNotSatisfiable:
		return KindRangeNotSatisfiable
	case http.StatusUnauthorized:
		return KindUnauthorized
	case http.StatusForbidden:
		return KindForbidden
	default:
		return KindUnknown
	}
}
