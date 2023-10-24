// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5"
)

type Mqtt5ClientMap struct {
	mqtt5Clients map[string]mqtt5.Mqtt5Client
	mutex        *sync.RWMutex
}

// Mqtt5ClientMapName contains the name of the Mqtt5ClientMap struct in the DIC.
var Mqtt5ClientMapName = di.TypeInstanceToName((*Mqtt5ClientMap)(nil))

// Mqtt5ClientMapFrom helper function queries the DIC and returns the Dev and Remotes mode flags.
func Mqtt5ClientMapFrom(get di.Get) Mqtt5ClientMap {
	mqtt5Config, ok := get(Mqtt5ClientMapName).(*Mqtt5ClientMap)
	if !ok {
		return Mqtt5ClientMap{}
	}

	return *mqtt5Config
}

func (mc *Mqtt5ClientMap) Get(brokerName string) (mqtt5.Mqtt5Client, error) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	for name, client := range mc.mqtt5Clients {
		if name == brokerName {
			return client, nil
		}
	}

	return mqtt5.Mqtt5Client{}, fmt.Errorf("%s Mqtt5Client not found", brokerName)
}

func (mc *Mqtt5ClientMap) ConnectAll(ctx context.Context, logger log.Logger) bool {
	for name, client := range mc.mqtt5Clients {
		if err := client.Connect(ctx, logger); err != nil {
			logger.Warnf("Failed to connect mqtt5Client %s: %v", name, err)
			return false
		}
	}
	logger.Info("All mqtt5Clients are connected")

	return true
}

func (mc *Mqtt5ClientMap) DisconnectAll(logger log.Logger) error {
	for name, client := range mc.mqtt5Clients {
		if err := client.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect mqtt5Client %s: %v", name, err)
			return err
		}
	}

	logger.Info("All mqtt5Clients are disconnected")

	return nil
}
