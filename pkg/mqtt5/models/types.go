// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"context"
	"github.com/google/uuid"
)

const (
	CorrelationID   = "X-Correlation-ID"
	ContentType     = "Content-Type"
	ContentTypeJSON = "application/json"
)

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

// NewMessageEnvelope creates a new MessageEnvelope for the specified payload with attributes from the specified context
func NewMessageEnvelope(payload []byte, ctx context.Context) MessageEnvelope {
	correlationID := fromContext(ctx, CorrelationID)
	if len(correlationID) == 0 {
		correlationID = uuid.NewString()
	}
	contentType := fromContext(ctx, ContentType)
	if len(correlationID) == 0 {
		contentType = ContentTypeJSON
	}

	envelope := MessageEnvelope{
		CorrelationID: correlationID,
		ContentType:   contentType,
		Payload:       payload,
	}

	return envelope
}

func fromContext(ctx context.Context, key string) string {
	hdr, ok := ctx.Value(key).(string)
	if !ok {
		hdr = ""
	}
	return hdr
}
