// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

// MessageEnvelope is the data structure for messages. It wraps the generic message payload with attributes.
type MessageEnvelope struct {
	// Payload is byte representation of the data being transferred.
	Payload []byte `json:"payload"`
	// ReceivedTopic is the topic that the message was received on.
	ReceivedTopic string `json:"receivedTopic"`
	// ContentType is the marshaled type of payload, i.e. application/json, application/xml, application/cbor, etc
	ContentType string `json:"contentType"`
	// CorrelationID is an object id to identify the envelope.
	CorrelationID string `json:"correlationID"`
}
