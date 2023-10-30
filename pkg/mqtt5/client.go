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
	"sync"
	"time"

	"github.com/eclipse/paho.golang/paho"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/config"
)

const (
	DefaultDialTimeOut = 30
)

type Mqtt5Client struct {
	configuration config.Mqtt5Config
	authData      secret.SecretData
	mqtt5Client   *paho.Client
	connect       *paho.Connect
	isConnected   bool
	mutex         sync.Mutex
}

// NewMqtt5Client create, initializes and returns new instance of Mqtt5Client
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

// SetAuthData retrieves and sets up auth data from secret provider according to AuthMode and SecretName
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

// Connect establishes a connection to a MQTT server.
func (c *Mqtt5Client) Connect(ctx context.Context, logger log.Logger) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Avoid reconnecting if already connected.
	server := c.configuration.Host + ":" + strconv.Itoa(c.configuration.Port)
	if c.isConnected {
		logger.Debugf("Already connected to %s://%s", c.configuration.Protocol, server)
		return nil
	}

	// IPv6 address must be enclosed in square brackets
	if ip := net.ParseIP(c.configuration.Host); ip != nil && ip.To4() == nil {
		server = strings.Replace(server, c.configuration.Host, "["+c.configuration.Host+"]", 1)
	}

	conn, err := net.DialTimeout(c.configuration.Protocol, server, time.Second*time.Duration(DefaultDialTimeOut))
	if err != nil {
		return fmt.Errorf("dial %s with timeout %vs failed: %w", server, DefaultDialTimeOut, err)
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

// Disconnect closes the connection to the connected MQTT server.
func (c *Mqtt5Client) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isConnected {
		return nil
	}

	d := &paho.Disconnect{ReasonCode: 0}
	err := c.mqtt5Client.Disconnect(d)
	if err != nil {
		return err
	}

	c.isConnected = false

	return nil
}
