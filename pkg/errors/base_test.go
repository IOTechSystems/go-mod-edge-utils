// Copyright (C) 2023 IOTech Ltd

package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	ErrL0        = NewBaseError(KindUnknown, "", nil)
	ErrL1        = fmt.Errorf("nothing")
	ErrL1Wrapper = ToBaseError(ErrL1)
	ErrL2Wrapper = ToBaseError(ErrL1Wrapper)
	ErrL2        = NewBaseError(KindDatabaseError, "database failed", ErrL1)
	ErrL3        = ToBaseError(ErrL2)
	ErrL4        = NewBaseError(KindUnknown, "don't know", ErrL3)
	ErrL5        = NewBaseError(KindCommunicationError, "network disconnected", ErrL4)
)

func TestKind(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind ErrKind
	}{
		{"Check the empty BaseError", ErrL0, KindUnknown},
		{"Check the non-BaseError", ErrL1, KindUnknown},
		{"Get the first error kind with 1 error wrapped", ErrL2, KindDatabaseError},
		{"Get the first error kind with 2 error wrapped", ErrL3, KindDatabaseError},
		{"Get the first non-unknown error kind with 3 error wrapped", ErrL4, KindDatabaseError},
		{"Get the first error kind with 4 error wrapped", ErrL5, KindCommunicationError},
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
		{"Get the first level error message from an empty error", ErrL0, ""},
		{"Get the first level error message from an empty Error with 1 error wrapped", ErrL1Wrapper, ErrL1.Error()},
		{"Get the first level error message from an empty Error with 1 empty error wrapped", ErrL2Wrapper, ErrL1.Error()},
		{"Get the first level error message from an Error with 1 error wrapped", ErrL2, ErrL2.message},
		{"Get the first level error message from an empty Error with 2 error wrapped", ErrL3, ErrL2.message},
		{"Get the first level error message from an Error with 3 error wrapped", ErrL4, ErrL4.message},
		{"Get the first level error message from an Error with 4 error wrapped", ErrL5, ErrL5.message},
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
		{"Get the chained error message from an empty error", ErrL0, []string{""}},
		{"Get the chained error message from an empty Error with 1 error wrapped", ErrL1Wrapper, []string{ErrL1.Error()}},
		{"Get the chained error message from an empty Error with 1 empty error wrapped", ErrL2Wrapper, []string{ErrL1.Error()}},
		{"Get the chained error message from an Error with 1 error wrapped", ErrL2, []string{ErrL2.message, ErrL1.Error()}},
		{"Get the chained error message from an empty Error with 2 error wrapped", ErrL3, []string{ErrL2.message, ErrL1.Error()}},
		{"Get the chained error message from an Error with 3 error wrapped", ErrL4, []string{ErrL4.message, ErrL2.message, ErrL1.Error()}},
		{"Get the chained error message from an Error with 4 error wrapped", ErrL5, []string{ErrL5.message, ErrL4.message, ErrL2.message, ErrL1.Error()}},
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
