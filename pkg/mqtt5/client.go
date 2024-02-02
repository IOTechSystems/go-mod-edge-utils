// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package mqtt5

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/paho"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/mqtt5/models"
)

const (
	DefaultDialTimeOut = 30
)

type Mqtt5Client struct {
	logger        log.Logger
	ctx           context.Context
	configuration models.Mqtt5Config
	authData      secret.SecretData
	mqtt5Client   *paho.Client
	connect       *paho.Connect
	isConnected   bool
	mutex         sync.Mutex
}

// NewMqtt5Client create, initializes and returns new instance of Mqtt5Client
func NewMqtt5Client(logger log.Logger, ctx context.Context, config models.Mqtt5Config) Mqtt5Client {
	return Mqtt5Client{
		logger:        logger,
		ctx:           ctx,
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
func (c *Mqtt5Client) SetAuthData(secretProvider interfaces.SecretProvider) errors.Error {
	authMode := strings.ToLower(c.configuration.AuthMode)
	if len(authMode) == 0 || authMode == secret.AuthModeNone {
		return nil
	}

	if len(c.configuration.SecretName) == 0 {
		return errors.NewBaseError(errors.KindContractInvalid, "missing SecretName", nil, nil)
	}

	c.logger.Infof("Setting auth data for secure MessageBus with AuthMode='%s' and SecretName='%s",
		authMode,
		c.configuration.SecretName)

	secretData, err := secret.GetSecretData(c.configuration.SecretName, secretProvider)
	if err != nil {
		return errors.NewBaseError(errors.KindEntityDoesNotExist, "Unable to get Secret Data for secure message bus", err, nil)
	}

	switch authMode {
	case secret.AuthModeUsernamePassword:
		if secretData.Username == "" || secretData.Password == "" {
			return errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("AuthModeUsernamePassword selected however Username or Password was not found for secret=%s", c.configuration.SecretName), nil, nil)
		}
		c.authData.Username = secretData.Username
		c.authData.Password = secretData.Password
		c.connect.Username = c.authData.Username
		c.connect.Password = []byte(c.authData.Password)
		c.connect.UsernameFlag = true
		c.connect.PasswordFlag = true
	case secret.AuthModeCert:
		if secretData.KeyPemBlock == "" || secretData.CertPemBlock == "" {
			return errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("AuthModeCert selected however the key or cert PEM block was not found for secret=%s", c.configuration.SecretName), nil, nil)
		}
		c.authData.CertPemBlock = secretData.CertPemBlock
		c.authData.KeyPemBlock = secretData.KeyPemBlock
	case secret.AuthModeCA:
		if secretData.CaPemBlock == "" {
			return errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("AuthModeCA selected however no PEM Block was found for secret=%s", c.configuration.SecretName), nil, nil)
		}

	default:
		return errors.NewBaseError(errors.KindContractInvalid, fmt.Sprintf("Invalid AuthMode of '%s' selected", c.configuration.AuthMode), nil, nil)
	}

	if len(secretData.CaPemBlock) > 0 {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(secretData.CaPemBlock))
		if !ok {
			return errors.NewBaseError(errors.KindContractInvalid, "Error parsing CA Certificate", nil, nil)
		}
		c.authData.CaPemBlock = secretData.CaPemBlock
	}

	return nil
}

// Connect establishes a connection to a MQTT server.
func (c *Mqtt5Client) Connect() errors.Error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Avoid reconnecting if already connected.
	server := c.configuration.Host + ":" + strconv.Itoa(c.configuration.Port)
	if c.isConnected {
		c.logger.Debugf("Already connected to %s://%s", c.configuration.Protocol, server)
		return nil
	}

	// IPv6 address must be enclosed in square brackets
	if ip := net.ParseIP(c.configuration.Host); ip != nil && ip.To4() == nil {
		server = strings.Replace(server, c.configuration.Host, "["+c.configuration.Host+"]", 1)
	}

	conn, err := net.DialTimeout(c.configuration.Protocol, server, time.Second*time.Duration(DefaultDialTimeOut))
	if err != nil {
		return errors.NewBaseError(errors.KindCommunicationError, fmt.Sprintf("dial %s with timeout %vs failed", server, DefaultDialTimeOut), err, nil)
	}
	c.mqtt5Client.Conn = conn
	ca, err := c.mqtt5Client.Connect(c.ctx, c.connect)
	if ca != nil && ca.ReasonCode != 0 {
		c.logger.Debugf("Received an MQTT 5 reason code: 0x%02x - %s", ca.ReasonCode, ca.Properties.ReasonString)
	}
	if err != nil {
		c.logger.Errorf("Failed to connect to %s://%s", c.configuration.Protocol, server)
		return errors.NewBaseError(errors.KindCommunicationError, "", err, nil)
	}

	c.isConnected = true
	c.logger.Infof("Connected to %s://%s with Client ID: %s", c.configuration.Protocol, server, c.configuration.ClientID)
	return nil
}

// Disconnect closes the connection to the connected MQTT server.
func (c *Mqtt5Client) Disconnect() errors.Error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isConnected {
		return nil
	}

	d := &paho.Disconnect{ReasonCode: 0}
	err := c.mqtt5Client.Disconnect(d)
	if err != nil {
		return errors.NewBaseError(errors.KindCommunicationError, "", err, nil)
	}

	c.isConnected = false
	c.logger.Infof("Disconnected to %s://%s:%s with Client ID: %s", c.configuration.Protocol, c.configuration.Host, strconv.Itoa(c.configuration.Port), c.configuration.ClientID)
	return nil
}

// Subscribe creates subscriptions for the specified topics and the message handler.
// Only support two handleType: paho.MessageHandler and chan models.MessageEnvelope
// There is a known issue when using wildcard to subscribe the duplicated messages:
// https://github.com/eclipse/paho.golang/issues/204
// https://github.com/eclipse/mosquitto/issues/2555
func (c *Mqtt5Client) Subscribe(topics []string, handlerType any) errors.Error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var handler paho.MessageHandler
	switch v := handlerType.(type) {
	case chan models.MessageEnvelope:
		messageChannel := v
		handler = newDefaultMessageHandler(messageChannel)
	case paho.MessageHandler:
		handler = v
	default:
		return errors.NewBaseError(errors.KindNotAllowed, "Unsupported handlerType, only support chan models.MessageEnvelope and paho.MessageHandler", nil, nil)

	}

	var subscriptions []paho.SubscribeOptions
	for _, topic := range topics {
		c.logger.Debugf("Register MQTT5 message handler to topic: %v by handler type: %T", topic, handlerType)
		c.mqtt5Client.Router.RegisterHandler(topic, handler)

		sub := paho.SubscribeOptions{Topic: topic, QoS: byte(c.configuration.QoS)}
		subscriptions = append(subscriptions, sub)
	}

	sa, err := c.mqtt5Client.Subscribe(c.ctx, &paho.Subscribe{
		Subscriptions: subscriptions,
	})
	if sa != nil {
		for _, code := range sa.Reasons {
			// SUBACK returning reason code == QoS means successful
			if code != byte(c.configuration.QoS) {
				c.logger.Debugf("Received an MQTT 5 reason code: 0x%02x - %s", code, sa.Properties.ReasonString)
			}
		}
	}
	if err != nil {
		c.logger.Errorf("At least one subscription failed: %v", err)
		return errors.NewBaseError(errors.KindCommunicationError, "", err, nil)
	}

	c.logger.Debugf("Subscribed to %v", strings.Join(topics, ","))

	return nil
}

// Unsubscribe to unsubscribe from the specified topics.
func (c *Mqtt5Client) Unsubscribe(topics []string) errors.Error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	ua, err := c.mqtt5Client.Unsubscribe(c.ctx, &paho.Unsubscribe{Topics: topics})
	if ua != nil {
		for _, code := range ua.Reasons {
			if code != 0 {
				c.logger.Debugf("Received an MQTT 5 reason code: 0x%02x - %s", code, ua.Properties.ReasonString)
			}
		}
	}
	if err != nil {
		c.logger.Errorf("At least one unsubscription failed: %v", err)
		return errors.NewBaseError(errors.KindCommunicationError, "", err, nil)
	}

	for _, t := range topics {
		c.mqtt5Client.Router.UnregisterHandler(t)
		c.logger.Debugf("Unregister topic %s from MQTT5 message handler", t)
	}

	c.logger.Debugf("Unsubscribed to %v", strings.Join(topics, ","))

	return nil
}

// Publish sends a message to the connected MQTT server.
func (c *Mqtt5Client) Publish(topic string, message models.MessageEnvelope) errors.Error {
	c.logger.Debugf("Sending message: %v to topic: %s", message, topic)

	pa, err := c.mqtt5Client.Publish(c.ctx, &paho.Publish{
		Topic:   topic,
		QoS:     byte(c.configuration.QoS),
		Payload: message.Payload,
		Properties: &paho.PublishProperties{
			CorrelationData: []byte(message.CorrelationID),
			ContentType:     message.ContentType,
		},
	})

	if pa != nil && pa.ReasonCode != 0 {
		c.logger.Debugf("Received an MQTT 5 reason code: 0x%02x - %s", pa.ReasonCode, pa.Properties.ReasonString)
	}
	if err != nil {
		c.logger.Errorf("Failed to send message to %s", topic)
		return errors.NewBaseError(errors.KindCommunicationError, "", err, nil)
	}

	c.logger.Tracef("Message is sent to %s with correlation id: %s", topic, message.CorrelationID)

	return nil
}

func newDefaultMessageHandler(messageChannel chan<- models.MessageEnvelope) paho.MessageHandler {
	handler := func(m *paho.Publish) {
		var messageEnvelope models.MessageEnvelope
		messageEnvelope.Payload = m.Payload
		messageEnvelope.ReceivedTopic = m.Topic
		messageEnvelope.ContentType = m.Properties.ContentType
		if len(m.Properties.CorrelationData) > 0 {
			messageEnvelope.CorrelationID = string(m.Properties.CorrelationData)
		}
		messageChannel <- messageEnvelope
	}
	return handler
}
