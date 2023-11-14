// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package mqtt5

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
)

// MessageClient is the messaging interface for publisher-subscriber pattern
type MessageClient interface {
	// SetAuthData sets up message bus auth data
	SetAuthData(secretProvider interfaces.SecretProvider) error

	// Connect to messaging host specified in Mqtt5Config config
	// returns error if not able to connect
	Connect() error

	// Disconnect is to close all connections on the message bus
	Disconnect() error

	// Subscribe is to receive messages from topics
	// the function returns error for any subscribe error
	Subscribe(topics []string, handlerType any) error

	// Unsubscribe to unsubscribe from the specified topics.
	Unsubscribe(topics []string) error
}
