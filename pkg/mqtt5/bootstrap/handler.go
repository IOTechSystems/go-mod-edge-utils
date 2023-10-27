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
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/config"
)

func Mqtt5BootstrapHandler(ctx context.Context, wg *sync.WaitGroup, startupTimer startup.Timer, dic *di.Container) bool {
	logger := container.LoggerFrom(dic.Get)
	secretProvider := container.SecretProviderFrom(dic.Get)
	if secretProvider == nil {
		logger.Error("Secret provider is missing. Make sure it is specified to be used in bootstrap.Run()")
	}
	mqtt5ConfigMap := container.ConfigurationFrom(dic.Get).Mqtt5Config
	if mqtt5ConfigMap == nil {
		logger.Error("No Mqtt5Config configuration provided")
		return false
	}

	// create client and connect
	mutex := &sync.RWMutex{}
	var clientMap Mqtt5ClientMap
	if clientMap.mqtt5Clients == nil {
		clientMap.mqtt5Clients = map[string]*mqtt5.Mqtt5Client{}
		clientMap.mutex = mutex
	}

	for configName, mqttConfig := range mqtt5ConfigMap {
		if err := config.Validate(mqttConfig); err != nil {
			logger.Errorf("Mqtt5Config %s validation error: %s", configName, err)
			return false
		}

		client := mqtt5.NewMqtt5Client(mqttConfig)
		client.SetMutex(mutex)

		if err := client.SetAuthData(secretProvider, logger); err != nil {
			logger.Errorf("Setting MQTT 5 auth data failed: %v", err)
			return false
		}

		clientMap.mqtt5Clients[configName] = &client

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
			if err := clientMap.ConnectAll(ctx, logger); err != nil {
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
