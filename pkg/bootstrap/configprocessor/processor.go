/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2023 Intel Corporation
 * Copyright 2023 IOTech Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package configprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/environment"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/flags"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/utils"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

const (
	DefaultConfigJsonFile = "configuration.json"
	yamlExt               = ".yaml"
	ymlExt                = ".yml"
	jsonExt               = ".json"
)

// UpdatedStream defines the stream type that is notified by ListenForChanges when a configuration update is received.
//type UpdatedStream chan struct{}

type Processor struct {
	logger       log.Logger
	flags        flags.Common
	envVars      *environment.Variables
	startupTimer startup.Timer
	ctx          context.Context
	wg           *sync.WaitGroup
	dic          *di.Container
}

// NewProcessor creates a new configuration Processor
func NewProcessor(
	flags flags.Common,
	envVars *environment.Variables,
	startupTimer startup.Timer,
	ctx context.Context,
	wg *sync.WaitGroup,
	dic *di.Container,
) *Processor {
	return &Processor{
		logger:       container.LoggerFrom(dic.Get),
		flags:        flags,
		envVars:      envVars,
		startupTimer: startupTimer,
		ctx:          ctx,
		wg:           wg,
		dic:          dic,
	}
}

func (cp *Processor) Process(
	serviceType string,
	serviceConfig interfaces.Configuration,
	secretProvider interfaces.SecretProvider) error {

	if err := cp.loadFromFile(serviceConfig, "service"); err != nil {
		return err
	}

	// Override file-based configuration with envVars variables.
	// Variables variable overrides have precedence over all others,
	// so make sure they are applied before config is used for anything.
	overrideCount, err := cp.envVars.OverrideConfiguration(serviceConfig)
	if err != nil {
		return err
	}
	cp.logger.Infof("Configuration loaded from file with %d overrides applied", overrideCount)

	// Now that configuration has been loaded and overrides applied the log level can be set as configured.
	err = cp.logger.SetLogLevel(serviceConfig.GetLogLevel())

	return err
}

// LoadFromFile attempts to read and unmarshal toml-based configuration into a configuration struct.
func (cp *Processor) loadFromFile(config any, configType string) error {
	configDir := environment.GetConfigDir(cp.logger, cp.flags.ConfigDirectory())
	configFileName := environment.GetConfigFileName(cp.logger, cp.flags.ConfigFileName())

	filePath := filepath.Join(configDir, configFileName)

	contents, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not load %s configuration file (%s): %s", configType, filePath, err.Error())
	}

	switch filepath.Ext(filePath) {
	case yamlExt, ymlExt:
		data := make(map[string]any)
		err = yaml.Unmarshal(contents, &data)
		if err != nil {
			return fmt.Errorf("failed to unmarshall configuration file %s: %s", filePath, err.Error())
		}
		err = utils.ConvertFromMap(data, config)
	case jsonExt:
		err = json.Unmarshal(contents, config)
	default:
		return fmt.Errorf("configuration file format isn't support, only support %s, %s, %s", yamlExt, ymlExt, jsonExt)
	}

	if err != nil {
		return fmt.Errorf("failed to unmarshall configuration file %s: %s", filePath, err.Error())
	}

	cp.logger.Infof("Loaded %s configuration from %s", configType, filePath)

	return nil
}
