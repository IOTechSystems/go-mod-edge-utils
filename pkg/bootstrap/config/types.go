/*******************************************************************************
 * Copyright 2018 Dell Inc.
 * Copyright 2023 Intel Corporation
 * Copyright 2021-2025 IOTech Ltd.
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

import (
	"fmt"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets/client"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/secrets/types"
)

const (
	SecuritySecretStoreSetupServiceKey = "security-secretstore-setup"
)

const (
	Vault = "vault"
)

type GeneralConfiguration struct {
	LogLevel        string
	Service         ServiceInfo
	SecretStore     SecretStoreInfo
	InsecureSecrets InsecureSecrets
}

// GetBootstrap returns the configuration elements required by the bootstrap.
func (c *GeneralConfiguration) GetBootstrap() BootstrapConfiguration {
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
	Authentication types.AuthenticationInfo
	// TokenFile provides a location to a token file.
	TokenFile string
	// SecretsFile is optional Path to JSON file containing secrets to seed into service's SecretStore
	SecretsFile string
	// DisableScrubSecretsFile specifies to not scrub secrets file after importing. Service will fail start-up if
	// not disabled and file can not be written.
	DisableScrubSecretsFile bool
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

// SecretsSetupInfo encapsulates the configuration used to auto-generate TLS certificates
// This is not a general config for all services. Only services that require auto-generated TLS certificates need this config.
type SecretsSetupInfo struct {
	// CertConfig is used for auto-generating the TLS certificates when user didn't specify the TLS_KEY_PATH and TLS_CERT_PATH
	CertConfig string
	// CertOutputDir indicates the folder for auto-generated TLS certificates
	CertOutputDir string
}

// ClientsCollection is a collection of Client information for communicating to dependent clients.
type ClientsCollection map[string]*ClientInfo

// ClientInfo provides the host and port of another service in the eco-system.
type ClientInfo struct {
	// Host is the hostname or IP address of a service.
	Host string
	// Port defines the port on which to access a given service
	Port int
	// Protocol indicates the protocol to use when accessing a given service
	Protocol string
	// UseMessageBus indicates weather to use Messaging version of client
	UseMessageBus bool
	// SecurityOptions is a key/value map, used for configuring clients. Currently used for zero trust but
	// could be for other options additional security related configuration
	SecurityOptions map[string]string
}

func (c ClientInfo) Url() string {
	url := fmt.Sprintf("%s://%s:%v", c.Protocol, c.Host, c.Port)
	return url
}

func NewSecretStoreSetupClientInfo() *ClientsCollection {
	secretStoreStepClient := ClientsCollection{
		SecuritySecretStoreSetupServiceKey: &ClientInfo{
			Host:     "localhost",
			Port:     59843,
			Protocol: "http",
		}}
	return &secretStoreStepClient
}

func NewSecretStoreInfo(serviceKey string) SecretStoreInfo {
	return SecretStoreInfo{
		Type:                    client.DefaultSecretStore,
		Protocol:                "http",
		Host:                    "localhost",
		Port:                    8200,
		StoreName:               serviceKey,
		TokenFile:               fmt.Sprintf("/tmp/edgex/secrets/%s/secrets-token.json", serviceKey),
		DisableScrubSecretsFile: false,
		Namespace:               "",
		RootCaCertPath:          "",
		ServerName:              "",
		SecretsFile:             "",
		Authentication: types.AuthenticationInfo{
			AuthType:  "X-Vault-Token",
			AuthToken: "",
		},
	}
}
