// Copyright (C) 2023 IOTech Ltd

package errors

import (
	"errors"
	"fmt"
	"runtime"
)

// BaseError generalizes an error structure which can be used for any type of error
type BaseError struct {
	// wrappedErr is a nested error which is used to form a chain of errors for better context
	wrappedErr error
	// kind contains information regarding the error type
	kind ErrKind
	// message contains detailed information about the error.
	message string
	// callerInfo contains information of function call stacks when this BaseError is invoked.
	callerInfo string
	// extensions contains extra information regarding the error
	details ErrDetails
}

// Error implements the Error interface for the BaseError struct
func (be BaseError) Error() string {
	if be.wrappedErr == nil {
		return be.message
	}

	// be.wrappedErr.Error functionality gets the error message of the wrapped error and which will handle both BaseError
	// types and Go standard errors(both wrapped and non-wrapped).
	if be.message != "" {
		return be.message + " -> " + be.wrappedErr.Error()
	} else {
		return be.wrappedErr.Error()
	}
}

// Message returns the first level error message without further details.
func (be BaseError) Message() string {
	if be.message == "" && be.wrappedErr != nil {
		if w, ok := be.wrappedErr.(BaseError); ok {
			return w.Message()
		} else {
			return be.wrappedErr.Error()
		}
	}

	return be.message
}

// DebugMessages returns a string taking all nested and wrapped operations and errors into account.
func (be BaseError) DebugMessages() string {
	if be.wrappedErr == nil {
		return be.callerInfo + ": " + be.message
	}

	if w, ok := be.wrappedErr.(BaseError); ok {
		return be.callerInfo + ": " + be.message + " -> " + w.DebugMessages()
	} else {
		return be.callerInfo + ": " + be.message + " -> " + be.wrappedErr.Error()
	}
}

// Kind returns the error kind of this BaseError.
func (be BaseError) Kind() ErrKind {
	return be.kind
}

// Details returns the detail maps of this BaseError.
func (be BaseError) Details() ErrDetails {
	return be.details
}

// AddDetail adds a detail associated with key for this BaseError.
func (be BaseError) AddDetail(key string, detail any) {
	if be.details == nil {
		be.details = make(ErrDetails)
	}
	be.details[key] = detail
}

// Unwrap retrieves the next nested error in the wrapped error chain.
// This is used by the new wrapping and unwrapping features available in Go 1.13 and aids in traversing the error chain
// of wrapped errors.
func (be BaseError) Unwrap() error {
	return be.wrappedErr
}

// Is determines if an error is of type BaseError.
// This is used by the new wrapping and unwrapping features available in Go 1.13 and aids the errors.Is function when
// determining is an error or any error in the wrapped chain contains an error of a particular type.
func (be BaseError) Is(err error) bool {
	var baseError BaseError
	switch {
	case errors.As(err, &baseError):
		return true
	default:
		return false
	}
}

// Kind determines the Kind associated with an error by inspecting the chain of errors. The first non-KindUnknown Kind
// found from the chain of wrappedErr is returned.  If Kind cannot be determined from wrappedErr, KindUnknown is returned
func Kind(err error) ErrKind {
	var e BaseError
	if !errors.As(err, &e) {
		return KindUnknown
	}
	// We want to return the first "Kind" we see that isn't Unknown, because
	// the higher in the stack the Kind was specified the more context we had.
	if e.kind != KindUnknown || e.wrappedErr == nil {
		return e.kind
	}
	return Kind(e.wrappedErr)
}

// NewBaseError creates a new BaseError with the information provided
func NewBaseError(kind ErrKind, errMsg string, err error, detail ErrDetails) BaseError {
	return BaseError{
		kind:       kind,
		message:    errMsg,
		wrappedErr: err,
		callerInfo: getCallerInformation(),
		details:    detail,
	}
}

// ToBaseError creates a new BaseError with Kind found from err
func ToBaseError(err error) BaseError {
	kind := Kind(err)
	return NewBaseError(kind, "", err, nil)
}

// getCallerInformation generates information about the caller function. This function skips the caller which has
// invoked this function, but rather introspects the calling function 3 frames below this frame in the call stack. This
// function is a helper function which eliminates the need for the 'callerInfo' field in the `BaseError` type and
// providing an 'callerInfo' string when creating an 'BaseError'
func getCallerInformation() string {
	pc := make([]uintptr, 10)
	runtime.Callers(3, pc)
	f := runtime.FuncForPC(pc[0])
	file, line := f.FileLine(pc[0])
	return fmt.Sprintf("[%s]-%s(line %d)", file, f.Name(), line)
}
