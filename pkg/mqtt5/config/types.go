// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"strings"

	validator "github.com/go-playground/validator/v10"
)

type Mqtt5Config struct {
	Host       string `validate:"required"`
	Port       int    `validate:"required"`
	Protocol   string `validate:"required"`
	AuthMode   string
	SecretName string
	ClientID   string // Client ID to use when connecting to server
	QoS        int    // QOS to use when publishing
	KeepAlive  uint16 // seconds between keepalive packets
	CleanStart bool
}

var val *validator.Validate

// Validate function will use the validator package to validate the struct annotation
func Validate(a interface{}) error {
	val = validator.New()
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
	var msg string
	switch tag {
	case "required":
		msg = fmt.Sprintf("%s field is required", fieldName)
	default:
		msg = fmt.Sprintf("%s field validation failed on the %s tag", fieldName, tag)
	}
	return msg
}
