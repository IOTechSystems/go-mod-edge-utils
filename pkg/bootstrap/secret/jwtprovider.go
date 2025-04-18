//
// Copyright (C) 2022-2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package secret

import (
	"fmt"
	"net/http"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	clientinterfaces "github.com/IOTechSystems/go-mod-edge-utils/pkg/contracts/clients/interfaces"
)

type jwtSecretProvider struct {
	secretProvider interfaces.SecretProvider
}

func NewJWTSecretProvider(secretProvider interfaces.SecretProvider) clientinterfaces.AuthenticationInjector {
	return &jwtSecretProvider{
		secretProvider: secretProvider,
	}
}

func (self *jwtSecretProvider) AddAuthenticationData(req *http.Request) error {
	if self.secretProvider == nil {
		// Test cases or real code may invoke NewJWTSecretProvider(nil),
		// though this is discouraged. In that case, just do nothing.
		return nil
	}

	// Otherwise if there is a secret provider, get the JWT
	jwt, err := self.secretProvider.GetSelfJWT()
	if err != nil {
		return err
	}

	// Only add authorization header if we get non-empty token back
	if len(jwt) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", jwt))
	}

	return nil
}
