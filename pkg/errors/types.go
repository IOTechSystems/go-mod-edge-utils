// Copyright (C) 2023 IOTech Ltd

package errors

// ErrKind a categorical identifier used to give high-level insight as to the error type.
type ErrKind string

const (
	// Constant Kind identifiers which can be used to label and group errors.
	KindUnknown               ErrKind = "Unknown"
	KindDatabaseError         ErrKind = "Database"
	KindCommunicationError    ErrKind = "Communication"
	KindEntityDoesNotExist    ErrKind = "NotFound"
	KindContractInvalid       ErrKind = "ContractInvalid"
	KindServerError           ErrKind = "UnexpectedServerError"
	KindLimitExceeded         ErrKind = "LimitExceeded"
	KindStatusConflict        ErrKind = "StatusConflict"
	KindDuplicateName         ErrKind = "DuplicateName"
	KindInvalidId             ErrKind = "InvalidId"
	KindServiceUnavailable    ErrKind = "ServiceUnavailable"
	KindNotAllowed            ErrKind = "NotAllowed"
	KindServiceLocked         ErrKind = "ServiceLocked"
	KindNotImplemented        ErrKind = "NotImplemented"
	KindRangeNotSatisfiable   ErrKind = "RangeNotSatisfiable"
	KindIOError               ErrKind = "IOError"
	KindOverflowError         ErrKind = "OverflowError"
	KindNaNError              ErrKind = "NaNError"
	KindUnauthorized          ErrKind = "Unauthorized"
	KindAuthenticationFailure ErrKind = "AuthenticationFailure"
	KindPayloadDecodeFailure  ErrKind = "PayloadDecodeFailure"
	KindTimeout               ErrKind = "Timeout"
)

// ErrDetails is a detailed mapping to set extra information with the error
type ErrDetails map[string]any

// Error provides an abstraction for all internal errors.
// This exists so that we can use this type in our method signatures and return nil which will fit better with the way
// the Go builtin errors are normally handled.
type Error interface {
	// Error obtains the error message associated with the error.
	Error() string
	// Message returns the first level error message without further details.
	Message() string
	// DebugMessages returns a detailed string for debug purpose.
	DebugMessages() string
	// Kind returns the error kind of this edge error.
	Kind() ErrKind
	// Details returns the detailed mapping associated with this edge error.
	Details() ErrDetails
}
