//
// Copyright (C) 2024 IOTech Ltd
//

package pkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	utilsErrors "github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
)

type httpSender struct {
	url                string
	mimeType           string
	secretData         secretData
	httpRequestHeaders map[string]string
}

type secretData struct {
	secretName        string
	secretValueKey    string
	secretHeader      string
	secretValuePrefix string
}

// NewHTTPSender creates, initializes and returns a new instance of HTTPSender
func NewHTTPSender(url string, mimeType string) HTTPSender {
	return &httpSender{
		url:      url,
		mimeType: mimeType,
	}
}

// SetHTTPRequestHeaders will set all the header parameters for the http request
func (sender *httpSender) SetHTTPRequestHeaders(httpRequestHeaders map[string]string) {
	if httpRequestHeaders != nil {
		sender.httpRequestHeaders = httpRequestHeaders
	}
}

// SetSecretData will set the secret header parameter for the http request
func (sender *httpSender) SetSecretData(name, valueKey, headerName, valuePrefix string) {
	sender.secretData = secretData{
		secretName:        name,
		secretValueKey:    valueKey,
		secretHeader:      headerName,
		secretValuePrefix: valuePrefix,
	}
}

// HTTPPost will send data to the specified Endpoint via http POST.
func (sender *httpSender) HTTPPost(dic *di.Container, data any) utilsErrors.Error {
	logger := container.LoggerFrom(dic.Get)
	logger.Debugf("POSTing data to '%s'", sender.url)
	return sender.httpSend(dic, data, http.MethodPost)
}

func (sender *httpSender) httpSend(dic *di.Container, data any, method string) utilsErrors.Error {
	logger := container.LoggerFrom(dic.Get)

	if data == nil {
		return utilsErrors.NewBaseError(utilsErrors.KindEntityDoesNotExist, "No data", nil, nil)
	}

	exportData, err := coerceType(data)
	if err != nil {
		return utilsErrors.NewBaseError(utilsErrors.KindContractInvalid, "", err, nil)
	}

	parsedUrl, err := url.Parse(sender.url)
	if err != nil {
		return utilsErrors.NewBaseError(utilsErrors.KindNotAllowed, "Failed to parse url", err, nil)
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, parsedUrl.String(), bytes.NewReader(exportData))
	if err != nil {
		return utilsErrors.NewBaseError(utilsErrors.KindServerError, "", err, nil)
	}

	// Set content type
	req.Header.Set(common.ContentType, sender.mimeType)

	// Set all the http request headers
	for key, element := range sender.httpRequestHeaders {
		req.Header.Set(key, element)
	}

	// Set secret header
	usingSecrets, err := sender.determineIfUsingSecret()
	if err != nil {
		return utilsErrors.ToBaseError(err)
	}
	if usingSecrets {
		secretProvider := container.SecretProviderFrom(dic.Get)
		secret, err := secretProvider.GetSecret(sender.secretData.secretName, sender.secretData.secretValueKey)
		if err != nil {
			return utilsErrors.NewBaseError(utilsErrors.KindEntityDoesNotExist, "", err, nil)
		}
		element := secret[sender.secretData.secretValueKey]
		if len(sender.secretData.secretValuePrefix) != 0 {
			element = strings.Join([]string{sender.secretData.secretValuePrefix, element}, " ")
		}

		logger.Debugf("Setting HTTP Header '%s' with secret value from SecretProvider at "+
			"secretName='%s' & secretValueKey='%s' and secretValuePrefix='%s'",
			sender.secretData.secretHeader,
			sender.secretData.secretName,
			sender.secretData.secretValueKey,
			sender.secretData.secretValuePrefix,
		)
		req.Header.Set(sender.secretData.secretHeader, element)
	}

	response, err := client.Do(req)
	if err != nil {
		return utilsErrors.NewBaseError(utilsErrors.KindServerError, "", err, nil)
	}

	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return utilsErrors.NewBaseError(utilsErrors.KindIOError, "Fail to read the response body", err, nil)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return utilsErrors.NewBaseError(utilsErrors.KindCommunicationError,
			fmt.Sprintf("Received '%v' status code, and response body: %v", response.StatusCode, string(body)), nil, nil)
	}

	logger.Debugf("Received '%v' status code, and response body: %v", response.StatusCode, string(body))
	return nil
}

func (sender *httpSender) determineIfUsingSecret() (bool, utilsErrors.Error) {
	// not using secret
	if len(sender.secretData.secretName) == 0 && len(sender.secretData.secretValueKey) == 0 &&
		len(sender.secretData.secretHeader) == 0 {
		return false, nil
	}

	//check fields
	if len(sender.secretData.secretName) == 0 {
		return false, utilsErrors.NewBaseError(utilsErrors.KindContractInvalid, "secretName must be specified", nil, nil)
	}
	if len(sender.secretData.secretValueKey) == 0 {
		return false, utilsErrors.NewBaseError(utilsErrors.KindContractInvalid, "secretName was specified but no secretValueKey was provided", nil, nil)
	}
	if len(sender.secretData.secretHeader) == 0 {
		return false, utilsErrors.NewBaseError(utilsErrors.KindContractInvalid, "secretName and secretValueKey were specified but no secretHeader was provided", nil, nil)
	}

	// using secret, all required fields are provided
	return true, nil
}

// coerceType will accept a string, []byte, or json.Marshaller type and convert it to a []byte for use
func coerceType(param any) ([]byte, error) {
	var data []byte
	var err error

	switch p := param.(type) {
	case string:
		input := p
		data = []byte(input)
	case []byte:
		data = p
	default:
		data, err = json.Marshal(param)
		if err != nil {
			return nil, errors.New("marshaling input data to JSON failed, " +
				"passed in data must be of type []byte, string, or support marshaling to JSON",
			)
		}
	}

	return data, nil
}
