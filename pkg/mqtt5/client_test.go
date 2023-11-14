// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package mqtt5

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"

	bootstrapMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/models"
)

const testCACert = `-----BEGIN CERTIFICATE-----
MIIDhTCCAm2gAwIBAgIUQl1RUGewZOXaSLnmH1i12zSYOtswDQYJKoZIhvcNAQEL
BQAwUjELMAkGA1UEBhMCVVMxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDELMAkGA1UEAwwCY2EwHhcNMjAwNDA4
MDExNDQ2WhcNMjUwNDA4MDExNDQ2WjBSMQswCQYDVQQGEwJVUzETMBEGA1UECAwK
U29tZS1TdGF0ZTEhMB8GA1UECgwYSW50ZXJuZXQgV2lkZ2l0cyBQdHkgTHRkMQsw
CQYDVQQDDAJjYTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAOqslFtX
nxr6yBZdLDKp1iTmsnFreEit7Z1BnNy9vQW6xrKRH+nxZWr0n9UIbx7KtmFkSBQ9
Bb5zC/3ZdjcuQAuKSTgQB7AP1D2dX6geJPo1Ph9NS0aVmuUqQ6dU+/4R5ATfoWag
M7slCixfkBzbHEh0mCqr7FoDWq2h+Cz2n8K85tBZjLyUuzyRaqH7ZkHfJD1cxkGK
FcwudCg4zpKYOSctm+JpTlF6YPjlngN79jaJIQEAmx/twv1lOCAGBw/hZM3FGmQx
5dA1W7qaJ6NHgNRXWRS1AERtHpAAsWNBT1CKuAS/j0PlreRyR3aMgQYQ5camxi9a
qCrMiHybaqj+UCkCAwEAAaNTMFEwHQYDVR0OBBYEFPNCbvrfw2QDoOyYfNjT9sNO
52xOMB8GA1UdIwQYMBaAFPNCbvrfw2QDoOyYfNjT9sNO52xOMA8GA1UdEwEB/wQF
MAMBAf8wDQYJKoZIhvcNAQELBQADggEBAHdFTqe6vi3BzgOMJEMO+81ZmiMohgKZ
Alyo8wH1C5RgwWW5w1OU+2RQfdOZgDfFkuQzmj0Kt2gzqACuAEtKzDt78lJ4f+WZ
MmRKBudJONUHTTm1micK3pqmn++nSygag0KxDvVbL+stSEgZwEBSOEvGDPXrL5qs
5yVOCi4xvsOCa1ymSnW6sX0z5GcgJQj2Znrr5QbEKHFSG86+WYEYnZ2zCNV7ahQo
bwXGZPOCUkpQzOstie/lPsf3Sd13/NIAk23TQ+rtaWIP9syQ85XWGRKRAUFOJEK0
2/jr0Xot+Y/3raEfNSrq6sHTzX1q4PoWkSwNEEGXifBqDr+9PXK3mOQ=
-----END CERTIFICATE-----
`

func TestSetAuthData(t *testing.T) {
	mockSecretProvider := &bootstrapMocks.SecretProvider{}
	mockLogger := &loggerMocks.Logger{}

	mqtt5ConfigWithNoAuthMode := models.Mqtt5Config{}
	mqtt5ConfigWithNoneAuthMode := models.Mqtt5Config{AuthMode: secret.AuthModeNone}
	mqtt5ConfigWithoutSecretName := models.Mqtt5Config{AuthMode: secret.AuthModeUsernamePassword}

	mqtt5ConfigWithUsernamePassword := models.Mqtt5Config{AuthMode: secret.AuthModeUsernamePassword, SecretName: "usernamepassword"}
	mqtt5ConfigWithoutUsername := models.Mqtt5Config{AuthMode: secret.AuthModeUsernamePassword, SecretName: "noUsername"}
	mqtt5ConfigWithoutPassword := models.Mqtt5Config{AuthMode: secret.AuthModeUsernamePassword, SecretName: "noPassword"}
	expectedUsernamePassword := map[string]string{secret.SecretUsernameKey: "admin", secret.SecretPasswordKey: "admin"}
	missingUsername := map[string]string{secret.SecretPasswordKey: "admin"}
	missingPassword := map[string]string{secret.SecretPasswordKey: "admin"}

	mqtt5ConfigWithClientCert := models.Mqtt5Config{AuthMode: secret.AuthModeCert, SecretName: "clientcert"}
	mqtt5ConfigWithoutClientKey := models.Mqtt5Config{AuthMode: secret.AuthModeUsernamePassword, SecretName: "noClientkey"}
	mqtt5ConfigWithoutClientCert := models.Mqtt5Config{AuthMode: secret.AuthModeUsernamePassword, SecretName: "noClientcert"}
	expectedClientCert := map[string]string{secret.SecretClientKey: "clientkey", secret.SecretClientCert: "clientcert"}
	missingClientKey := map[string]string{secret.SecretClientKey: "clientkey"}
	missingClientCert := map[string]string{secret.SecretClientCert: "clientcert"}

	mqtt5ConfigWithCAAuthMode := models.Mqtt5Config{AuthMode: secret.AuthModeCA, SecretName: "ca"}
	mqtt5ConfigWithNoCA := models.Mqtt5Config{AuthMode: secret.AuthModeCA, SecretName: "noCA"}
	mqtt5ConfigWithInvalidCA := models.Mqtt5Config{AuthMode: secret.AuthModeCA, SecretName: "invalidCA"}
	expectedCA := map[string]string{secret.SecretCACert: testCACert}
	noCA := map[string]string{secret.SecretCACert: ""}
	invalidCA := map[string]string{secret.SecretCACert: "------"}

	tests := []struct {
		Name        string
		Mqtt5Config models.Mqtt5Config
		Err         bool
		SecretData  map[string]string
	}{
		{"Set AuthData with no AuthMode", mqtt5ConfigWithNoAuthMode, false, nil},
		{"Set AuthData with none AuthMode", mqtt5ConfigWithNoneAuthMode, false, nil},
		{"Set AuthData without SecretName", mqtt5ConfigWithoutSecretName, true, nil},
		{"Set AuthData with usernamepassword AuthMode", mqtt5ConfigWithUsernamePassword, false, expectedUsernamePassword},
		{"Set AuthData without username", mqtt5ConfigWithoutUsername, true, missingUsername},
		{"Set AuthData without password", mqtt5ConfigWithoutPassword, true, missingPassword},
		{"Set AuthData with clientcert AuthMode", mqtt5ConfigWithClientCert, false, expectedClientCert},
		{"Set AuthData without client key", mqtt5ConfigWithoutClientKey, true, missingClientKey},
		{"Set AuthData without client cert", mqtt5ConfigWithoutClientCert, true, missingClientCert},
		{"Set AuthData with CA Certificate", mqtt5ConfigWithCAAuthMode, false, expectedCA},
		{"Set AuthData with CA empty", mqtt5ConfigWithNoCA, true, noCA},
		{"Set AuthData with invalid CA", mqtt5ConfigWithInvalidCA, true, invalidCA},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			mockLogger.On("Infof", mock.AnythingOfType("string"), test.Mqtt5Config.AuthMode, test.Mqtt5Config.SecretName)
			if test.SecretData != nil {
				mockSecretProvider.On("GetSecret", test.Mqtt5Config.SecretName).Return(test.SecretData, nil)
			}

			client := NewMqtt5Client(mockLogger, context.Background(), test.Mqtt5Config)
			err := client.SetAuthData(mockSecretProvider)
			if test.Err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			switch test.Mqtt5Config.AuthMode {
			case secret.AuthModeUsernamePassword:
				assert.Equal(t, secret.SecretData{Username: test.SecretData[secret.SecretUsernameKey], Password: test.SecretData[secret.SecretPasswordKey]}, client.authData)
				assert.Equal(t, test.SecretData[secret.SecretUsernameKey], client.connect.Username)
				assert.Equal(t, test.SecretData[secret.SecretPasswordKey], string(client.connect.Password))
			case secret.AuthModeCert:
				assert.Equal(t, secret.SecretData{KeyPemBlock: test.SecretData[secret.SecretClientKey], CertPemBlock: test.SecretData[secret.SecretClientCert]}, client.authData)
			case secret.AuthModeCA:
				assert.Equal(t, secret.SecretData{CaPemBlock: test.SecretData[secret.SecretCACert]}, client.authData)
			}
		})
	}
}
