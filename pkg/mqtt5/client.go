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

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/config"
	"github.com/eclipse/paho.golang/paho"
)

const (
	AuthModeNone             = "none"
	AuthModeUsernamePassword = "usernamepassword"
	AuthModeCert             = "clientcert"
	AuthModeCA               = "cacert"

	SecretUsernameKey = "username"
	SecretPasswordKey = "password"
	SecretClientKey   = "clientkey"
	SecretClientCert  = AuthModeCert
	SecretCACert      = AuthModeCA
)

type Mqtt5Client struct {
	configuration config.Mqtt5Config
	authData      SecretData
	mqtt5Client   *paho.Client
	connect       *paho.Connect
	isConnected   bool
}

type SecretData struct {
	Username     string
	Password     string
	KeyPemBlock  string
	CertPemBlock string
	CaPemBlock   string
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
	logger.Infof("Setting auth data for secure MessageBus with AuthMode='%s' and SecretName='%s",
		c.configuration.AuthMode,
		c.configuration.SecretName)
	if secretProvider == nil {
		logger.Error("No secret provider")
		return nil
	}
	secrets, err := secretProvider.GetSecret(c.configuration.SecretName)
	if err != nil {
		return fmt.Errorf("Unable to get Secret Data for secure message bus: %w", err)
	}

	switch c.configuration.AuthMode {
	case AuthModeUsernamePassword:
		if secrets[SecretUsernameKey] == "" || secrets[SecretPasswordKey] == "" {
			return fmt.Errorf("AuthModeUsernamePassword selected however Username or Password was not found for secret=%s", c.configuration.SecretName)
		}
		c.authData.Username = secrets[SecretUsernameKey]
		c.authData.Password = secrets[SecretPasswordKey]
		c.connect.Username = c.authData.Username
		c.connect.Password = []byte(c.authData.Password)
		c.connect.UsernameFlag = true
		c.connect.PasswordFlag = true
	case AuthModeCert:
		if secrets[SecretClientCert] == "" || secrets[SecretClientKey] == "" {
			return fmt.Errorf("AuthModeCert selected however the key or cert PEM block was not found for secret=%s", c.configuration.SecretName)
		}
		c.authData.CertPemBlock = secrets[SecretClientCert]
		c.authData.KeyPemBlock = secrets[SecretClientKey]
	case AuthModeCA:
		if secrets[SecretCACert] == "" {
			return fmt.Errorf("AuthModeCA selected however no PEM Block was found for secret=%s", c.configuration.SecretName)
		}

	case AuthModeNone:
		// Nothing to validate
	default:
		return fmt.Errorf("Invalid AuthMode of '%s' selected", c.configuration.AuthMode)
	}

	if len(secrets[SecretCACert]) > 0 {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(secrets[SecretCACert]))
		if !ok {
			return errors.New("Error parsing CA Certificate")
		}
		c.authData.CaPemBlock = secrets[SecretCACert]
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
