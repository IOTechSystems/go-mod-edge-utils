//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
)

// SecretClient provides a contract for storing and retrieving secrets from a secret store provider.
type SecretClient interface {
	// GetSecret retrieves secret from a secret store.
	// secretName specifies the type or location of the secret to retrieve. If specified it is appended
	// to the base path from the SecretConfig
	// keys specifies the secret data to retrieve. If no keys are provided then all the keys associated with the
	// specified path will be returned.
	GetSecret(secretName string, keys ...string) (map[string]string, error)

	// StoreSecret stores the secret to a secret store.
	// it sets the values requested at provided keys
	// secretName specifies the type or location of the secret to store.
	// data map specifies the "key": "value" pairs of secret data to store
	StoreSecret(secretName string, data map[string]string) error

	// SetAuthToken sets the internal Auth Token with the new value specified.
	SetAuthToken(ctx context.Context, token string) error

	// GetSecretNames retrieves the secret names currently in service's secret store.
	GetSecretNames() ([]string, error)

	// GetSelfJWT returns an encoded JWT for the current identity-based secret store token
	GetSelfJWT(serviceKey string) (string, error)

	// IsJWTValid evaluates a given JWT and returns a true/false if the JWT is valid (i.e. belongs to us and current) or not
	IsJWTValid(jwt string) (bool, error)
}
