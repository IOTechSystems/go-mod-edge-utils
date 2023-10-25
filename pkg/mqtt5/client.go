// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package mqtt5

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/config"
	"github.com/eclipse/paho.golang/paho"
)

type Mqtt5Client struct {
	configuration config.Mqtt5Config
	authData      secret.SecretData
	mqtt5Client   *paho.Client
	connect       *paho.Connect
	isConnected   bool
}

func NewMqtt5Client(config config.Mqtt5Config) Mqtt5Client {
	return Mqtt5Client{
		configuration: config,
		mqtt5Client: paho.NewClient(paho.ClientConfig{
			ClientID: config.ClientID,
		}),
		connect: &paho.Connect{
			ClientID:   config.ClientID,
			KeepAlive:  config.KeepAlive,
			CleanStart: config.CleanStart,
		},
	}
}
func (c *Mqtt5Client) SetAuthData(secretProvider interfaces.SecretProvider, logger log.Logger) error {
	authMode := strings.ToLower(c.configuration.AuthMode)
	if len(authMode) == 0 || authMode == secret.AuthModeNone {
		return nil
	}

	if len(c.configuration.SecretName) == 0 {
		return errors.New("missing SecretName")
	}

	logger.Infof("Setting auth data for secure MessageBus with AuthMode='%s' and SecretName='%s",
		authMode,
		c.configuration.SecretName)

	secretData, err := secret.GetSecretData(c.configuration.SecretName, secretProvider)
	if err != nil {
		return fmt.Errorf("Unable to get Secret Data for secure message bus: %w", err)
	}

	switch authMode {
	case secret.AuthModeUsernamePassword:
		if secretData.Username == "" || secretData.Password == "" {
			return fmt.Errorf("AuthModeUsernamePassword selected however Username or Password was not found for secret=%s", c.configuration.SecretName)
		}
		c.authData.Username = secretData.Username
		c.authData.Password = secretData.Password
		c.connect.Username = c.authData.Username
		c.connect.Password = []byte(c.authData.Password)
		c.connect.UsernameFlag = true
		c.connect.PasswordFlag = true
	case secret.AuthModeCert:
		if secretData.KeyPemBlock == "" || secretData.CertPemBlock == "" {
			return fmt.Errorf("AuthModeCert selected however the key or cert PEM block was not found for secret=%s", c.configuration.SecretName)
		}
		c.authData.CertPemBlock = secretData.CertPemBlock
		c.authData.KeyPemBlock = secretData.KeyPemBlock
	case secret.AuthModeCA:
		if secretData.CaPemBlock == "" {
			return fmt.Errorf("AuthModeCA selected however no PEM Block was found for secret=%s", c.configuration.SecretName)
		}

	default:
		return fmt.Errorf("Invalid AuthMode of '%s' selected", c.configuration.AuthMode)
	}

	if len(secretData.CaPemBlock) > 0 {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(secretData.CaPemBlock))
		if !ok {
			return errors.New("Error parsing CA Certificate")
		}
		c.authData.CaPemBlock = secretData.CaPemBlock
	}

	return nil
}

func (c *Mqtt5Client) Connect(ctx context.Context, logger log.Logger) error {
	// Avoid reconnecting if already connected.
	if c.isConnected {
		return nil
	}

	server := c.configuration.Host + ":" + strconv.Itoa(c.configuration.Port)
	conn, err := net.Dial(c.configuration.Protocol, server)
	if err != nil {
		return err
	}
	c.mqtt5Client.Conn = conn
	ca, err := c.mqtt5Client.Connect(ctx, c.connect)
	if err != nil {
		if ca.ReasonCode != 0 {
			logger.Errorf("Failed to connect to %s://%s with reason code: %d - %s", c.configuration.Protocol, server, ca.ReasonCode, ca.Properties.ReasonString)
			return err
		}
		return err
	}

	c.isConnected = true
	logger.Infof("Connected to %s://%s", c.configuration.Protocol, server)
	return nil
}

func (c *Mqtt5Client) Disconnect() error {
	d := &paho.Disconnect{ReasonCode: 0}
	err := c.mqtt5Client.Disconnect(d)
	c.isConnected = false
	return err
}
