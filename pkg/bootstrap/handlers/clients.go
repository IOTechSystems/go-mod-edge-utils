//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	httpClients "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/clients"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/secret"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/secret/clients"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
)

// ClientsBootstrapHandler creates instances of each of the EdgeX clients that are in the service's configuration
// and place them in the DIC.
func ClientsBootstrapHandler(dic *di.Container, cfg interfaces.Configuration) {
	lc := container.LoggerFrom(dic.Get)
	if cfg.GetBootstrap().Clients != nil {
		for serviceKey, serviceInfo := range *cfg.GetBootstrap().Clients {
			var urlFunc clients.ClientBaseUrlFunc

			sp := container.SecretProviderFrom(dic.Get)
			jwtSecretProvider := secret.NewJWTSecretProvider(sp)

			lc.Infof("Using REST for '%s' clients @ %s", serviceKey, serviceInfo.Url())
			urlFunc = clients.GetDefaultClientBaseUrlFunc(serviceInfo.Url())

			switch serviceKey {
			case common.SecurityProxyAuthServiceKey:
				dic.Update(di.ServiceConstructorMap{
					container.SecurityProxyAuthClientName: func(get di.Get) interface{} {
						return httpClients.NewAuthClientWithUrlCallback(urlFunc, jwtSecretProvider)
					},
				})
			}
		}
	}
}
