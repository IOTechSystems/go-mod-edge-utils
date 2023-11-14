/*******************************************************************************
 * Copyright 2018 Dell Inc.
 * Copyright 2023 Intel Corporation
 * Copyright 2021-2023 IOTech Ltd.
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

package config

import "github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/models"

const (
	Vault = "vault"
)

type GeneralConfiguration struct {
	LogLevel        string
	Service         ServiceInfo
	SecretStore     SecretStoreInfo
	InsecureSecrets InsecureSecrets
	Mqtt5Config     map[string]models.Mqtt5Config
}

// GetBootstrap returns the configuration elements required by the bootstrap.
func (c *GeneralConfiguration) GetBootstrap() BootstrapConfiguration {
	// temporary until we can make backwards-breaking configuration.yaml change
	return BootstrapConfiguration{
		Service: &c.Service,
	}
}

// GetLogLevel returns the current ConfigurationStruct's log level.
func (c *GeneralConfiguration) GetLogLevel() string {
	return c.LogLevel
}

// GetInsecureSecrets gets the config.InsecureSecrets field from the ConfigurationStruct.
func (c *GeneralConfiguration) GetInsecureSecrets() InsecureSecrets {
	return c.InsecureSecrets
}

// ServiceInfo contains configuration settings necessary for the basic operation of any Edge service.
type ServiceInfo struct {
	// Host is the hostname or IP address of the service.
	Host string
	// Port is the HTTP port of the service.
	Port int
	// ServerBindAddr specifies an IP address or hostname
	// for ListenAndServe to bind to, such as 0.0.0.0
	ServerBindAddr string
	// StartupMsg specifies a string to log once service
	// initialization and startup is completed.
	StartupMsg string
	// MaxResultCount specifies the maximum size list supported
	// in response to REST calls to other services.
	MaxResultCount int
	// MaxRequestSize defines the maximum size of http request body in kilobytes
	MaxRequestSize int64
	// RequestTimeout specifies a timeout (in ISO8601 format) for
	// processing REST request calls from other services.
	RequestTimeout string
}

// SecretStoreInfo encapsulates configuration properties used to create a SecretClient.
type SecretStoreInfo struct {
	Type           string
	Host           string
	Port           int
	StoreName      string
	Protocol       string
	Namespace      string
	RootCaCertPath string
	ServerName     string
	Authentication AuthenticationInfo
	// TokenFile provides a location to a token file.
	TokenFile string
	// SecretsFile is optional Path to JSON file containing secrets to seed into service's SecretStore
	SecretsFile string
	// DisableScrubSecretsFile specifies to not scrub secrets file after importing. Service will fail start-up if
	// not disabled and file can not be written.
	DisableScrubSecretsFile bool
}

// AuthenticationInfo contains authentication information to be used when communicating with an HTTP based provider
type AuthenticationInfo struct {
	AuthType  string
	AuthToken string
}

// InsecureSecrets is used to hold the secrets stored in the configuration
type InsecureSecrets map[string]InsecureSecretsInfo

// InsecureSecretsInfo encapsulates info used to retrieve insecure secrets
type InsecureSecretsInfo struct {
	SecretName string
	SecretData map[string]string
}

// BootstrapConfiguration defines the configuration elements required by the bootstrap.
type BootstrapConfiguration struct {
	Service *ServiceInfo
}
