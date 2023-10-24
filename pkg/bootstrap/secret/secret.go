/********************************************************************************
 *  Copyright 2019 Dell Inc.
 *  Copyright 2022 Intel Corp.
 *  Copyright 2023 IOTech Ltd.
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
	"context"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/config"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/environment"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
)

// secret service Metric Names
const (
	secretsRequestedMetricName = "SecuritySecretsRequested"
	secretsStoredMetricName    = "SecuritySecretsStored"
)

// NewSecretProvider creates a new fully initialized the Secret Provider.
func NewSecretProvider(
	configuration *config.GeneralConfiguration,
	envVars *environment.Variables,
	ctx context.Context,
	startupTimer startup.Timer,
	dic *di.Container,
	serviceKey string) (interfaces.SecretProviderExt, error) {
	logger := container.LoggerFrom(dic.Get)

	var provider interfaces.SecretProviderExt

	switch IsSecurityEnabled() {
	case true:
		// attempt to create a new Secure client only if security is enabled.
		logger.Error("Secure client and authentication token are not implemented")
	case false:
		provider = NewInsecureProvider(configuration, logger, dic) // return 501
	}

	dic.Update(di.ServiceConstructorMap{
		// Must put the SecretProvider instance in the DIC for both the standard API use by service code
		// and the extended API used by boostrap code
		container.SecretProviderName: func(get di.Get) any {
			return provider
		},
		container.SecretProviderExtName: func(get di.Get) any {
			return provider
		},
	})

	return provider, nil
}
