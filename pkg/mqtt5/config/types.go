// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package config

type Mqtt5Config struct {
	// hostname_rfc1123 refers to https://github.com/go-playground/validator/blob/94a637ab9fbbb0bc0fe8a278f0352d0b14e2c365/regexes.go#L52C22-L52C22
	Host       string `validate:"required,ip|hostname_rfc1123"`
	Port       int    `validate:"required"`
	Protocol   string `validate:"required"`
	AuthMode   string
	SecretName string
	ClientID   string // Client ID to use when connecting to server
	QoS        int    // QOS to use when publishing
	KeepAlive  uint16 // seconds between keepalive packets
	CleanStart bool
}
