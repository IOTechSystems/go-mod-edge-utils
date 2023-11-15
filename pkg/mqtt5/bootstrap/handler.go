// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/validator"
)

// Mqtt5BootstrapHandler fulfills the BootstrapHandler contract. It creates and initializes the MQTT 5 clients
// and adds the MQTT 5 client map to the DIC
func Mqtt5BootstrapHandler(ctx context.Context, wg *sync.WaitGroup, startupTimer startup.Timer, dic *di.Container) bool {
	logger := container.LoggerFrom(dic.Get)
	secretProvider := container.SecretProviderFrom(dic.Get)
	if secretProvider == nil {
		logger.Error("Secret provider is missing. Make sure it is specified to be used in bootstrap.Run()")
	}
	mqtt5ConfigMap := container.ConfigurationFrom(dic.Get).GetMqtt5Configs()
	if mqtt5ConfigMap == nil {
		logger.Error("No Mqtt5Config configuration provided")
		return false
	}

	// create client and connect
	clientMap := NewMqtt5ClientMap()
	for configName, mqttConfig := range mqtt5ConfigMap {
		if err := validator.Validate(mqttConfig); err != nil {
			logger.Errorf("Mqtt5Config %s validation error: %s", configName, err)
			return false
		}

		client := mqtt5.NewMqtt5Client(logger, ctx, mqttConfig)

		if err := client.SetAuthData(secretProvider); err != nil {
			logger.Errorf("Setting MQTT 5 auth data failed: %v", err)
			return false
		}

		clientMap.Put(configName, &client)

		logger.Infof(
			"Created MQTT 5 client %s://%s:%d with Authmode=%s",
			mqttConfig.Protocol,
			mqttConfig.Host,
			mqttConfig.Port,
			mqttConfig.AuthMode)
	}

	for startupTimer.HasNotElapsed() {
		select {
		case <-ctx.Done():
			return false
		default:
			if err := clientMap.ConnectAll(logger); err != nil {
				startupTimer.SleepForInterval()
				continue
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				<-ctx.Done()
				if len(mqtt5ConfigMap) > 0 {
					_ = clientMap.DisconnectAll(logger)
				}
			}()

			dic.Update(di.ServiceConstructorMap{
				Mqtt5ClientMapName: func(get di.Get) interface{} {
					return &clientMap
				},
			})
			return true
		}
	}

	logger.Error("Connecting to MQTT 5 clients time out")
	// Disconnect already connected clients
	if len(clientMap.mqtt5Clients) > 0 {
		_ = clientMap.DisconnectAll(logger)
	}
	return false
}
