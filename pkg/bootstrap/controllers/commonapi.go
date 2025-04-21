//
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/mitchellh/mapstructure"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/utils"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/models"
)

// CommonController controller for common REST APIs
type CommonController struct {
	dic            *di.Container
	serviceName    string
	router         *echo.Echo
	serviceVersion string
	config         interfaces.Configuration
	logger         log.Logger
}

func NewCommonController(dic *di.Container, r *echo.Echo, serviceName string, serviceVersion string) *CommonController {
	logger := container.LoggerFrom(dic.Get)
	authenticationHook := handlers.AutoConfigAuthenticationFunc(dic)
	configuration := container.ConfigurationFrom(dic.Get)
	c := CommonController{
		dic:            dic,
		serviceName:    serviceName,
		router:         r,
		logger:         logger,
		serviceVersion: serviceVersion,
		config:         configuration,
	}
	r.GET(common.ApiPingRoute, c.Ping) // Health check is always unauthenticated
	r.GET(common.ApiVersionRoute, c.Version, authenticationHook)
	r.GET(common.ApiConfigRoute, c.Config, authenticationHook)
	r.POST(common.ApiSecretRoute, c.AddSecret, authenticationHook)

	return &c
}

func (c *CommonController) AddRoute(routePath string, handler echo.HandlerFunc, methods []string, authentication bool, middlewareFunc ...echo.MiddlewareFunc) {
	if authentication {
		authenticationHook := handlers.AutoConfigAuthenticationFunc(c.dic)
		middlewareFunc = append(middlewareFunc, authenticationHook)
	}
	c.router.Match(methods, routePath, handler, middlewareFunc...)
	c.logger.Debugf("Added route %s with methods %v ", routePath, methods)
}

// Ping handles the request to /ping endpoint. Is used to test if the service is working
// It returns a response as specified by the API swagger in the openapi directory
func (c *CommonController) Ping(e echo.Context) error {
	request := e.Request()
	writer := e.Response()
	response := models.NewPingResponse(c.serviceName)

	return utils.SendJsonResp(c.logger, writer, request, response, http.StatusOK)
}

// Version handles the request to /version endpoint. Is used to request the service's versions
// It returns a response as specified by the API swagger in the openapi directory
func (c *CommonController) Version(e echo.Context) error {
	request := e.Request()
	writer := e.Response()
	response := models.NewVersionResponse(c.serviceVersion, c.serviceName)

	return utils.SendJsonResp(c.logger, writer, request, response, http.StatusOK)
}

// Config handles the request to /config endpoint. Is used to request the service's configuration
// It returns a response as specified by the swagger in openapi/common
func (c *CommonController) Config(e echo.Context) error {
	request := e.Request()
	writer := e.Response()

	config := make(map[string]any)
	err := mapstructure.Decode(c.config, &config)
	if err != nil {
		c.logger.Errorf("%v", err.Error())
		return utils.SendJsonErrResp(c.logger, writer, request, errors.KindServerError, "config can not convert to map", err, "")
	}

	response := models.NewConfigResponse(config, c.serviceName)
	return utils.SendJsonResp(c.logger, writer, request, response, http.StatusOK)
}

// AddSecret handles the request to the /secret endpoint. Is used to add Service exclusive secret to the Secret Store
// It returns a response as specified by the API swagger in the openapi directory
func (c *CommonController) AddSecret(e echo.Context) error {
	request := e.Request()
	writer := e.Response()

	defer func() {
		_ = request.Body.Close()
	}()

	secretRequest := models.SecretRequest{}
	err := json.NewDecoder(request.Body).Decode(&secretRequest)
	if err != nil {
		c.logger.Errorf("%v", err.Error())
		return utils.SendJsonErrResp(c.logger, writer, request, errors.KindContractInvalid, "JSON decode failed", err, "")
	}

	err = addSecret(c.dic, secretRequest)
	if err != nil {
		return utils.SendJsonErrResp(c.logger, writer, request, errors.Kind(err), err.Error(), err, secretRequest.RequestId)
	}

	response := models.NewBaseResponse(secretRequest.RequestId, "", http.StatusCreated)

	return utils.SendJsonResp(c.logger, writer, request, response, http.StatusCreated)
}

// addSecret adds Service exclusive secret to the Secret Store
func addSecret(dic *di.Container, request models.SecretRequest) errors.Error {
	secretName, secret := prepareSecret(request)

	secretProvider := container.SecretProviderFrom(dic.Get)
	if secretProvider == nil {
		return errors.NewBaseError(errors.KindServerError, "secret provider is missing. Make sure it is specified to be used in bootstrap.Run()", nil, nil)
	}

	if err := secretProvider.StoreSecret(secretName, secret); err != nil {
		return errors.NewBaseError(errors.Kind(err), "adding secret failed", err, nil)
	}
	return nil
}

func prepareSecret(request models.SecretRequest) (string, map[string]string) {
	var secretsKV = make(map[string]string)
	for _, secret := range request.SecretData {
		secretsKV[secret.Key] = secret.Value
	}

	secretName := strings.TrimSpace(request.SecretName)

	return secretName, secretsKV
}
