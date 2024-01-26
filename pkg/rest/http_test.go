//
// Copyright (C) 2024 IOTech Ltd
//

package pkg

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	secretProviderMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces/mocks"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/pkg/log/mocks"
)

const (
	msgStr             = "test message"
	path               = "/some-path/foo"
	badPath            = "/some-path/bad"
	testSecretName     = "test"
	testSecretValueKey = "test_key"
	testSecretHeader   = "Authorization"
	testSecret         = "secret token"
)

func TestHTTPPostErr(t *testing.T) {
	mockLogger := &loggerMocks.Logger{}
	mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()

	dic := di.NewContainer(di.ServiceConstructorMap{
		container.LoggerInterfaceName: func(get di.Get) any {
			return mockLogger
		},
	})

	testUrl := "http://host" + path
	noSecret := secretData{}
	missingSecretName := secretData{
		secretValueKey: testSecretValueKey,
		secretHeader:   testSecretHeader,
	}
	missingSecretValueKey := secretData{
		secretName:   testSecretName,
		secretHeader: testSecretHeader,
	}
	missingSecretHeader := secretData{
		secretName:     testSecretName,
		secretValueKey: testSecretValueKey,
	}

	tests := []struct {
		Name   string
		Url    string
		Data   any
		Secret secretData
	}{
		{"Invalid url", path, msgStr, noSecret},
		{"Empty data", testUrl, nil, noSecret},
		{"Unsupported data type", testUrl, make(chan int), noSecret},
		{"Missing secretName", testUrl, msgStr, missingSecretName},
		{"Missing secretValueKey", testUrl, msgStr, missingSecretValueKey},
		{"Missing secretHeader", testUrl, msgStr, missingSecretHeader},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			sender := NewHTTPSender(test.Url, common.ContentTypeJSON)
			if test.Secret != noSecret {
				sender.SetSecretData(test.Secret.secretName, test.Secret.secretValueKey,
					test.Secret.secretHeader, test.Secret.secretValuePrefix)
			}
			err := sender.HTTPPost(dic, test.Data)
			assert.Error(t, err, "Should return an error")
		})
	}

}

func TestHTTPPost(t *testing.T) {
	mockLogger := &loggerMocks.Logger{}
	mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.Anything, mock.Anything).Return()

	noSecret := secretData{}
	secret := secretData{
		secretName:        testSecretName,
		secretValuePrefix: testSecretValueKey,
		secretHeader:      testSecretHeader,
	}
	mockSP := &secretProviderMocks.SecretProvider{}
	mockSP.On("GetSecret", testSecretName, testSecretValueKey).Return(map[string]string{testSecretValueKey: testSecret}, nil)

	dic := di.NewContainer(di.ServiceConstructorMap{
		container.LoggerInterfaceName: func(get di.Get) any {
			return mockLogger
		},
		container.SecretProviderName: func(get di.Get) any {
			return mockSP
		},
	})

	errBody := "Not Found"
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() == badPath {
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(errBody))
			if err != nil {
				t.Error(err)
			}
			return
		}

		w.WriteHeader(http.StatusOK)

		readMsg, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if strings.Compare((string)(readMsg), msgStr) != 0 {
			t.Errorf("Invalid msg received %v, expected %v", readMsg, msgStr)
		}
		_, err := w.Write(readMsg)
		if err != nil {
			t.Error(err)
		}

		if r.URL.EscapedPath() != path {
			t.Errorf("Invalid path received %s, expected %s",
				r.URL.EscapedPath(), path)
		}
	}

	// create test server with handler
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	targetUrl, err := url.Parse(ts.URL)
	require.NoError(t, err)

	tests := []struct {
		Name   string
		Path   string
		Secret secretData
		Err    bool
	}{
		{"Successfully POST", path, noSecret, false},
		{"Successfully POST with secret", path, secret, false},
		{"Failed POST with invalid Path", badPath, noSecret, true},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			sender := NewHTTPSender(`http://`+targetUrl.Host+test.Path, common.ContentTypeJSON)
			err = sender.HTTPPost(dic, msgStr)
			if test.Err {
				assert.Error(t, err, "Should return an error")
				assert.Contains(t, err.Error(), errBody)
			} else {
				assert.NoError(t, err, "Should not return an error")
			}
		})
	}
}
