/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2021 Intel Corp.
 * Copyright 2024-2025 IOTech Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package openbao

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets/types"
)

// Client defines the behavior for interacting with the OpenBao REST secret key/value store via HTTP(S).
type Client struct {
	Config     types.SecretConfig
	HttpCaller secrets.Caller
	lc         log.Logger
	context    context.Context
	// secretStoreTokenToCancelFuncMap is an internal map with token as key and the context.cancel function as value
	secretStoreTokenToCancelFuncMap secretStoreTokenToCancelFuncMap
	mapMutex                        sync.Mutex
	tokenExpiredCallback            secrets.TokenExpiredCallback
}

// NewClient constructs a secret store *Client which communicates with OpenBao via HTTP(S)
// lc is any logging client that implements the Logger interface.
func NewClient(config types.SecretConfig, requester secrets.Caller, forSecrets bool, lc log.Logger) (*Client, error) {

	if forSecrets && config.Authentication.AuthToken == "" {
		return nil, secrets.NewErrSecretStore("AuthToken is required in config")
	}

	var err error
	if requester == nil {
		requester, err = createHTTPClient(config)
		if err != nil {
			return nil, err
		}
	}

	secretStoreClient := Client{
		Config:                          config,
		HttpCaller:                      requester,
		lc:                              lc,
		mapMutex:                        sync.Mutex{},
		secretStoreTokenToCancelFuncMap: make(secretStoreTokenToCancelFuncMap),
	}

	return &secretStoreClient, err
}

func createHTTPClient(config types.SecretConfig) (secrets.Caller, error) {

	if config.RootCaCertPath == "" {
		return http.DefaultClient, nil
	}

	// Read and load the CA Root certificate so the client will be able to use TLS without skipping the verification of
	// the cert received by the server.
	caCert, err := os.ReadFile(config.RootCaCertPath)
	if err != nil {
		return nil, ErrCaRootCert{
			path:        config.RootCaCertPath,
			description: err.Error(),
		}
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				ServerName: config.ServerName,
				MinVersion: tls.VersionTLS12,
			},
		},
	}, nil
}
