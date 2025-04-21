/*******************************************************************************
 * Copyright 2020 Intel Inc.
 * Copyright 2023-2025 IOTech Ltd.
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

package secret

import (
	"os"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
)

const (
	// WildcardName is a special secret name that can be used to register a secret callback for any secret.
	WildcardName = "*"

	AuthModeNone             = "none"
	AuthModeUsernamePassword = "usernamepassword"
	AuthModeCert             = "clientcert"
	AuthModeCA               = "cacert"

	SecretUsernameKey = "username"
	SecretPasswordKey = "password"
	SecretClientKey   = "clientkey"
	SecretClientCert  = AuthModeCert
	SecretCACert      = AuthModeCA
)

type SecretData struct {
	Username     string
	Password     string
	KeyPemBlock  string
	CertPemBlock string
	CaPemBlock   string
}

func GetSecretData(secretName string, provider interfaces.SecretProvider) (SecretData, error) {
	result := SecretData{}

	secrets, err := provider.GetSecret(secretName)
	if err != nil {
		return result, err
	}

	result.Username = secrets[SecretUsernameKey]
	result.Password = secrets[SecretPasswordKey]
	result.KeyPemBlock = secrets[SecretClientKey]
	result.CertPemBlock = secrets[SecretClientCert]
	result.CaPemBlock = secrets[SecretCACert]

	return result, nil
}

// IsSecurityEnabled determines if security has been enabled.
func IsSecurityEnabled() bool {
	env := os.Getenv(common.EnvSecretStore)
	return env != "false" // Any other value is considered secure mode enabled
}
