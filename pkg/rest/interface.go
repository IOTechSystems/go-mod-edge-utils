//
// Copyright (C) 2024 IOTech Ltd
//

package rest

import (
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
)

// HTTPSender is the interface for http requests
type HTTPSender interface {
	// SetHTTPRequestHeaders sets up http request headers
	SetHTTPRequestHeaders(httpRequestHeaders map[string]string)

	// SetSecretData sets up http secret header
	SetSecretData(name, valueKey, headerName, valuePrefix string)

	// HTTPPost sends http POST request with data
	HTTPPost(dic *di.Container, data any) errors.Error
}
