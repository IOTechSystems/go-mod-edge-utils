/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2022 Intel Inc.
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

package interfaces

import "github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/config"

// Configuration interface provides an abstraction around a configuration struct.
type Configuration interface {

	// GetBootstrap returns the configuration elements required by the bootstrap.
	GetBootstrap() config.BootstrapConfiguration

	// GetLogLevel returns the current ConfigurationStruct's log level.
	GetLogLevel() string

	// GetInsecureSecrets gets the config.InsecureSecrets field from the configuration struct.
	GetInsecureSecrets() config.InsecureSecrets
}
