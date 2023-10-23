package bootstrap

import (
	"context"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/config"
	"strings"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
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
	var clientMap Mqtt5ClientMap
	if clientMap.mqtt5Clients == nil {
		clientMap.mqtt5Clients = map[string]mqtt5.Mqtt5Client{}
		clientMap.mutex = &sync.RWMutex{}
	}

	for configName, mqttConfig := range mqtt5ConfigMap {
		if !validateMqtt5Config(configName, mqttConfig, logger) {
			return false
		}

		client := mqtt5.NewMqtt5Client(mqttConfig)

		if len(mqttConfig.AuthMode) > 0 &&
			!strings.EqualFold(strings.TrimSpace(mqttConfig.AuthMode), mqtt5.AuthModeNone) {
			if err := client.SetAuthData(secretProvider, logger); err != nil {
				logger.Errorf("Setting MQTT 5 auth data failed: %v", err)
				return false
			}
		}

		clientMap.mqtt5Clients[configName] = client

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
			if !clientMap.ConnectAll(ctx, logger) {
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
	return false
}

func validateMqtt5Config(configName string, config config.Mqtt5Config, logger log.Logger) bool {
	var missingConfig []string
	if config.Host == "" {
		missingConfig = append(missingConfig, "Host")
	}
	if config.Port == 0 {
		missingConfig = append(missingConfig, "Port")
	}
	if config.Protocol == "" {
		missingConfig = append(missingConfig, "Protocol")
	}
	if config.AuthMode == "" {
		missingConfig = append(missingConfig, "AuthMode")
	}
	if config.SecretName == "" {
		missingConfig = append(missingConfig, "SecretName")
	}
	if missingConfig != nil {
		logger.Errorf("Missing required config: %v in Mqtt5Config %s", missingConfig, configName)
		return false
	}
	return true
}