// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"errors"
	"fmt"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5"
)

type Mqtt5ClientMap struct {
	mqtt5Clients map[string]mqtt5.MessageClient
	mutex        sync.RWMutex
}

// Mqtt5ClientMapName contains the name of the Mqtt5ClientMap struct in the DIC.
var Mqtt5ClientMapName = di.TypeInstanceToName((*Mqtt5ClientMap)(nil))

// Mqtt5ClientMapFrom helper function queries the DIC and returns Mqtt5ClientMap implementation.
func Mqtt5ClientMapFrom(get di.Get) *Mqtt5ClientMap {
	mqtt5Config, ok := get(Mqtt5ClientMapName).(*Mqtt5ClientMap)
	if !ok {
		return nil
	}

	return mqtt5Config
}

// NewMqtt5ClientMap create, initializes and returns new instance of Mqtt5ClientMap
func NewMqtt5ClientMap() Mqtt5ClientMap {
	return Mqtt5ClientMap{
		mqtt5Clients: make(map[string]mqtt5.MessageClient),
	}
}

// Get the specific client from Mqtt5ClientMap
func (mc *Mqtt5ClientMap) Get(brokerName string) (mqtt5.MessageClient, error) {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	client, ok := mc.mqtt5Clients[brokerName]
	if !ok {
		return nil, fmt.Errorf("%s Mqtt5Client not found", brokerName)
	}

	return client, nil
}

// Put new or updated client into Mqtt5ClientMap
func (mc *Mqtt5ClientMap) Put(brokerName string, c mqtt5.MessageClient) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.mqtt5Clients[brokerName] = c
}

// ConnectAll establishes all the connections to a MQTT server.
func (mc *Mqtt5ClientMap) ConnectAll(logger log.Logger) error {
	var errs error
	for name, client := range mc.mqtt5Clients {
		if err := client.Connect(); err != nil {
			logger.Warnf("Failed to connect mqtt5Client %s: %v", name, err)
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

// DisconnectAll closes all the connections to the connected MQTT server.
func (mc *Mqtt5ClientMap) DisconnectAll(logger log.Logger) error {
	var errs error
	for name, client := range mc.mqtt5Clients {
		if err := client.Disconnect(); err != nil {
			logger.Errorf("Failed to disconnect mqtt5Client %s: %v", name, err)
			errs = errors.Join(errs, err)
		}
		logger.Infof("Disconnected mqtt5Client %s", name)
	}

	return errs
}
