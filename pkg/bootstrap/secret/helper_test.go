// Copyright (C) 2023 IOTech Ltd

package secret

import (
	"errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

var dic *di.Container

func TestGetSecretData(t *testing.T) {
	testUsername := "TEST_USER"
	testPassword := "TEST_PASS"

	// setup mock secret client
	secrets := map[string]string{
		SecretUsernameKey: testUsername,
		SecretPasswordKey: testPassword,
	}

	expectedSecretData := SecretData{
		Username: testUsername,
		Password: testPassword,
	}

	mockSecretProvider := &mocks.SecretProvider{}
	mockSecretProvider.On("GetSecret", "notfound").Return(nil, errors.New("Not Found"))
	mockSecretProvider.On("GetSecret", "mqtt").Return(secrets, nil)

	dic = di.NewContainer(di.ServiceConstructorMap{
		container.SecretProviderName: func(get di.Get) interface{} {
			return mockSecretProvider
		},
	})

	tests := []struct {
		Name            string
		SecretName      string
		ExpectedSecrets SecretData
		ExpectingError  bool
	}{
		//{"No Auth No error", "", nil, false},
		{"Auth No SecretData found", "notfound", SecretData{}, true},
		{"Auth With SecretData", "mqtt", expectedSecretData, false},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			secretData, err := GetSecretData(test.SecretName, mockSecretProvider)
			if test.ExpectingError {
				assert.Error(t, err, "Expecting error")
				return
			}
			require.Equal(t, test.ExpectedSecrets, secretData)
		})
	}
}
