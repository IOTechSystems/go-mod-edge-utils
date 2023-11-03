// Copyright (C) 2023 IOTech Ltd

package errors

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	L0Error        = NewBaseError(KindUnknown, "", nil, nil)
	L1Error        = fmt.Errorf("nothing")
	L1ErrorWrapper = ToBaseError(L1Error)
	L2ErrorWrapper = ToBaseError(L1ErrorWrapper)
	L2Error        = NewBaseError(KindDatabaseError, "database failed", L1Error, nil)
	L3Error        = ToBaseError(L2Error)
	L4Error        = NewBaseError(KindUnknown, "don't know", L3Error, nil)
	L5Error        = NewBaseError(KindCommunicationError, "network disconnected", L4Error, nil)
)

func TestKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind ErrKind
	}{
		{"Check the empty BaseError", L0Error, KindUnknown},
		{"Check the non-BaseError", L1Error, KindUnknown},
		{"Get the first error kind with 1 error wrapped", L2Error, KindDatabaseError},
		{"Get the first error kind with 2 error wrapped", L3Error, KindDatabaseError},
		{"Get the first non-unknown error kind with 3 error wrapped", L4Error, KindDatabaseError},
		{"Get the first error kind with 4 error wrapped", L5Error, KindCommunicationError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := Kind(tt.err)
			assert.Equal(t, tt.kind, k, fmt.Sprintf("Retrieved Error Kind %s is not equal to %s.", k, tt.kind))
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name string
		err  Error
		msg  string
	}{
		{"Get the first level error message from an empty error", L0Error, ""},
		{"Get the first level error message from an empty Error with 1 error wrapped", L1ErrorWrapper, L1Error.Error()},
		{"Get the first level error message from an empty Error with 1 empty error wrapped", L2ErrorWrapper, L1Error.Error()},
		{"Get the first level error message from an Error with 1 error wrapped", L2Error, L2Error.message},
		{"Get the first level error message from an empty Error with 2 error wrapped", L3Error, L2Error.message},
		{"Get the first level error message from an Error with 3 error wrapped", L4Error, L4Error.message},
		{"Get the first level error message from an Error with 4 error wrapped", L5Error, L5Error.message},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.err.Message()
			assert.Equal(t, tt.msg, m, fmt.Sprintf("Returned error message %s is not equal to %s.", m, tt.msg))
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name string
		err  Error
		msgs []string
	}{
		{"Get the chained error message from an empty error", L0Error, []string{""}},
		{"Get the chained error message from an empty Error with 1 error wrapped", L1ErrorWrapper, []string{L1Error.Error()}},
		{"Get the chained error message from an empty Error with 1 empty error wrapped", L2ErrorWrapper, []string{L1Error.Error()}},
		{"Get the chained error message from an Error with 1 error wrapped", L2Error, []string{L2Error.message, L1Error.Error()}},
		{"Get the chained error message from an empty Error with 2 error wrapped", L3Error, []string{L2Error.message, L1Error.Error()}},
		{"Get the chained error message from an Error with 3 error wrapped", L4Error, []string{L4Error.message, L2Error.message, L1Error.Error()}},
		{"Get the chained error message from an Error with 4 error wrapped", L5Error, []string{L5Error.message, L4Error.message, L2Error.message, L1Error.Error()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.err.Error()
			for _, msg := range tt.msgs {
				assert.Contains(t, m, msg, fmt.Sprintf("Returned error message %s doesn't contain %s.", m, msg))
			}
		})
	}
}
