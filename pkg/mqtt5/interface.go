// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package mqtt5

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/models"
)

// MessageClient is the messaging interface for publisher-subscriber pattern
type MessageClient interface {
	// SetAuthData sets up message bus auth data
	SetAuthData(secretProvider interfaces.SecretProvider) errors.Error

	// Connect to messaging host specified in Mqtt5Config config
	// returns error if not able to connect
	Connect() errors.Error

	// Disconnect is to close all connections on the message bus
	Disconnect() errors.Error

	// Subscribe is to receive messages from topics
	// the function returns error for any subscribe error
	Subscribe(topics []string, handlerType any) errors.Error

	// Unsubscribe to unsubscribe from the specified topics.
	Unsubscribe(topics []string) errors.Error

	// Publish is to send message to the message bus
	// the message contains data payload to send to the message queue
	Publish(topic string, message models.MessageEnvelope) errors.Error
}
