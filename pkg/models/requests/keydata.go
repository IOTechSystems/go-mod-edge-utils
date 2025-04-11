//
// Copyright (C) 2024 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package requests

import (
	"encoding/json"
	"fmt"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/models"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/validator"
	"github.com/labstack/echo/v4"
	"net/http"
)

// AddKeyDataRequest defines the Request Content for POST Key DTO.
type AddKeyDataRequest struct {
	models.BaseRequest `json:",inline"`
	KeyData            models.KeyData `json:"keyData"`
}

// Validate satisfies the Validator interface
func (a *AddKeyDataRequest) Validate() error {
	err := validator.Validate(a)
	return err
}

// UnmarshalJSON implements the Unmarshaler interface for the AddUserRequest type
func (a *AddKeyDataRequest) UnmarshalJSON(b []byte) error {
	var alias struct {
		models.BaseRequest
		KeyData models.KeyData
	}
	if err := json.Unmarshal(b, &alias); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Failed to unmarshal request body as JSON: %s", err))
	}

	*a = AddKeyDataRequest(alias)
	if err := a.Validate(); err != nil {
		return err
	}
	return nil
}
