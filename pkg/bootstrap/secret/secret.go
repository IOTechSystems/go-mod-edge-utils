/********************************************************************************
 *  Copyright 2019 Dell Inc.
 *  Copyright 2022 Intel Corp.
 *  Copyright 2023-2025 IOTech Ltd.
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
	"fmt"
	"path"
	"strings"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/config"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/environment"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret/clients"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret/token/authtokenloader"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret/token/fileioperformer"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/secrets/client"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/secrets/types"
)

// secret service Metric Names
const (
	secretsRequestedMetricName    = "SecuritySecretsRequested"
	secretsStoredMetricName       = "SecuritySecretsStored"
	securityGetSecretDurationName = "SecurityGetSecretDuration"
)

// NewSecretProvider creates a new fully initialized the Secret Provider.
func NewSecretProvider(
	configuration interfaces.Configuration,
	envVars *environment.Variables,
	ctx context.Context,
	startupTimer startup.Timer,
	dic *di.Container,
	serviceKey string) (interfaces.SecretProvider, error) {
	logger := container.LoggerFrom(dic.Get)

	var provider interfaces.SecretProvider

	switch IsSecurityEnabled() {
	case true:
		// attempt to create a new Secure client only if security is enabled.
		var err error

		logger.Info("Creating SecretClient")

		secretStoreConfig, err := BuildSecretStoreConfig(serviceKey, envVars, logger)
		if err != nil {
			return nil, err
		}

		for startupTimer.HasNotElapsed() {
			var secretConfig types.SecretConfig

			logger.Info("Reading secret store configuration and authentication token")

			tokenLoader := container.AuthTokenLoaderFrom(dic.Get)
			if tokenLoader == nil {
				tokenLoader = authtokenloader.NewAuthTokenLoader(fileioperformer.NewDefaultFileIoPerformer())
			}

			secretConfig, err = getSecretConfig(secretStoreConfig, tokenLoader, serviceKey, logger)
			if err == nil {
				secureProvider := NewSecureProvider(ctx, secretStoreConfig, logger, tokenLoader, serviceKey)
				var secretClient client.SecretClient

				logger.Info("Attempting to create secret client")

				tokenCallbackFunc := secureProvider.DefaultTokenExpiredCallback

				secretClient, err = client.NewSecretsClient(ctx, secretConfig, logger, tokenCallbackFunc)
				if err == nil {
					secureProvider.SetClient(secretClient)
					provider = secureProvider
					logger.Info("Created SecretClient")

					logger.Debugf("SecretsFile is '%s'", secretConfig.SecretsFile)

					if len(strings.TrimSpace(secretConfig.SecretsFile)) == 0 {
						logger.Infof("SecretsFile not set, skipping seeding of service secrets.")
						break
					}

					provider = secureProvider
					logger.Info("Created SecretClient")

					err = secureProvider.LoadServiceSecrets(secretStoreConfig)
					if err != nil {
						return nil, err
					}
					break
				} else if strings.Contains(err.Error(), AccessTokenAuthError) {
					logger.Warnf("token expired, invoking secret-store-setup regen token API ........")

					clientCollection, err := BuildSecretStoreSetupClientConfig(envVars, logger)
					if err != nil {
						return nil, err
					}

					clientConfigs := *clientCollection
					ssSetupClient, ok := clientConfigs[config.SecuritySecretStoreSetupServiceKey]
					if !ok {
						return nil, fmt.Errorf("failed to obtain %s client from config", config.SecuritySecretStoreSetupServiceKey)
					}
					baseUrl := ssSetupClient.Url()

					entityId, err := tokenLoader.ReadEntityId(secretStoreConfig.TokenFile)
					if err != nil {
						return nil, err
					}

					// Use InsecureProvider here since the client token has been expired and cannot be used to obtain the JWT
					secretProvider := NewInsecureProvider(configuration, logger, dic)
					jwtProvider := NewJWTSecretProvider(secretProvider)
					httpClient := clients.NewSecretStoreTokenClient(baseUrl, jwtProvider)
					_, err = httpClient.RegenToken(ctx, entityId)
					if err != nil {
						return nil, err
					}

					logger.Infof("token file re-generated, trying to create the secret client again later")
				}
			}

			logger.Warn(fmt.Sprintf("Retryable failure while creating SecretClient: %s", err.Error()))
			startupTimer.SleepForInterval()
		}

		if err != nil {
			return nil, fmt.Errorf("unable to create SecretClient: %s", err.Error())
		}
	case false:
		provider = NewInsecureProvider(configuration, logger, dic) // return 501
	}

	dic.Update(di.ServiceConstructorMap{
		// Must put the SecretProvider instance in the DIC for both the standard API use by service code
		// and the extended API used by boostrap code
		container.SecretProviderName: func(get di.Get) any {
			return provider
		},
	})

	return provider, nil
}

// BuildSecretStoreConfig is public helper function that builds the SecretStore configuration
// from default values and  environment override.
func BuildSecretStoreConfig(serviceKey string, envVars *environment.Variables, lc log.Logger) (*config.SecretStoreInfo, error) {
	configWrapper := struct {
		SecretStore config.SecretStoreInfo
	}{
		SecretStore: config.NewSecretStoreInfo(serviceKey),
	}

	count, err := envVars.OverrideConfiguration(&configWrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to override SecretStore information: %v", err)
	}

	lc.Infof("SecretStore information created with %d overrides applied", count)
	return &configWrapper.SecretStore, nil
}

// getSecretConfig creates a SecretConfig based on the SecretStoreInfo configuration properties.
// If a token file is present it will override the Authentication.AuthToken value.
func getSecretConfig(secretStoreInfo *config.SecretStoreInfo,
	tokenLoader authtokenloader.AuthTokenLoader,
	serviceKey string,
	lc log.Logger) (types.SecretConfig, error) {
	secretConfig := types.SecretConfig{
		Type:           secretStoreInfo.Type, // Type of SecretStore implementation, i.e. OpenBao
		Host:           secretStoreInfo.Host,
		Port:           secretStoreInfo.Port,
		BasePath:       addSecretNamePrefix(secretStoreInfo.StoreName),
		SecretsFile:    secretStoreInfo.SecretsFile,
		Protocol:       secretStoreInfo.Protocol,
		Namespace:      secretStoreInfo.Namespace,
		RootCaCertPath: secretStoreInfo.RootCaCertPath,
		ServerName:     secretStoreInfo.ServerName,
		Authentication: secretStoreInfo.Authentication,
	}

	// maybe insecure mode
	// if configs of token file is empty or disabled
	// then we treat that as insecure mode
	if !IsSecurityEnabled() || (secretStoreInfo.TokenFile == "") {
		lc.Info("insecure mode")
		return secretConfig, nil
	}

	// based on whether token provider config is configured or not, we will obtain token in different way
	var token string
	var err error
	lc.Info("load token from file")
	// else obtain the token from TokenFile
	token, err = tokenLoader.Load(secretStoreInfo.TokenFile)

	if err != nil {
		return secretConfig, err
	}

	secretConfig.Authentication.AuthToken = token
	return secretConfig, nil
}

func addSecretNamePrefix(secretName string) string {
	trimmedSecretName := strings.TrimSpace(secretName)

	// in this case, treat it as no secret name prefix
	if len(trimmedSecretName) == 0 {
		return ""
	}

	// All API routes are prefixed with "/v1", which is currently the only version. https://openbao.org/api-docs/
	// The second part, "/secret", is the secret store mount point. This is hardcoded in edgex-go: https://github.com/edgexfoundry/edgex-go/blob/3fa30c341bbe9c3881ba0169229bc34ff150aef2/internal/security/secretstore/secretsengine/enabler.go#L28
	return "/" + path.Join("v1", "secret", trimmedSecretName)
}

// BuildSecretStoreSetupClientConfig is public helper function that builds the ClientsCollection configuration
// from default values and environment override.
func BuildSecretStoreSetupClientConfig(envVars *environment.Variables, lc log.Logger) (*config.ClientsCollection, error) {
	configWrapper := struct {
		Clients *config.ClientsCollection
	}{
		Clients: config.NewSecretStoreSetupClientInfo(),
	}

	count, err := envVars.OverrideConfiguration(&configWrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to override SecretStore information: %v", err)
	}

	lc.Infof("SecretStoreSetup client information created with %d overrides applied", count)
	return configWrapper.Clients, nil
}
