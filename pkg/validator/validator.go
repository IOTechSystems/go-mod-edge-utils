// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var val *validator.Validate

const (
	dtoNoneEmptyStringTag = "dto-none-empty-string"
)

func init() {
	val = validator.New()
	_ = val.RegisterValidation(dtoNoneEmptyStringTag, ValidateDtoNoneEmptyString)
}

// Validate function will use the validator package to validate the struct annotation
func Validate(a any) error {
	err := val.Struct(a)
	// translate all error at once
	if err != nil {
		errs := err.(validator.ValidationErrors)
		var errMsg []string
		for _, e := range errs {
			errMsg = append(errMsg, getErrorMessage(e))
		}
		return errors.New(strings.Join(errMsg, "; "))
	}

	return nil
}

// Internal: generate representative validation error messages
func getErrorMessage(e validator.FieldError) string {
	tag := e.Tag()
	// StructNamespace returns the namespace for the field error, with the field's actual name.
	fieldName := e.StructNamespace()
	fieldValue := e.Param()
	var msg string
	switch tag {
	case "uuid":
		msg = fmt.Sprintf("%s field needs a uuid", fieldName)
	case "required":
		msg = fmt.Sprintf("%s field is required", fieldName)
	case "required_without":
		msg = fmt.Sprintf("%s field is required if the %s is not present", fieldName, fieldValue)
	case "len":
		msg = fmt.Sprintf("The length of %s field is not %s", fieldName, fieldValue)
	case "oneof":
		msg = fmt.Sprintf("%s field should be one of %s", fieldName, fieldValue)
	case "gt":
		msg = fmt.Sprintf("%s field should greater than %s", fieldName, fieldValue)
	case dtoNoneEmptyStringTag:
		msg = fmt.Sprintf("%s field should not be empty string", fieldName)
	default:
		msg = fmt.Sprintf("%s field validation failed on the %s tag", fieldName, tag)
	}
	return msg
}

// ValidateDtoNoneEmptyString used to check the UpdateDTO name pointer value
func ValidateDtoNoneEmptyString(fl validator.FieldLevel) bool {
	val := fl.Field()
	// Skip the validation if the pointer value is nil
	if isNilPointer(val) {
		return true
	}
	// The string value should not be empty
	if len(strings.TrimSpace(val.String())) > 0 {
		return true
	} else {
		return false
	}
}

func isNilPointer(value reflect.Value) bool {
	return value.Kind() == reflect.Ptr && value.IsNil()
}
