/*******************************************************************************
 * Copyright 2020 Intel Inc.
 * Copyright 2023 IOTech Ltd.
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

import "os"

const (
	EnvSecretStore = "EDGE_SECURITY_SECRET_STORE"
	// WildcardName is a special secret name that can be used to register a secret callback for any secret.
	WildcardName = "*"
)

// IsSecurityEnabled determines if security has been enabled.
func IsSecurityEnabled() bool {
	env := os.Getenv(EnvSecretStore)
	return env != "false" // Any other value is considered secure mode enabled
}
