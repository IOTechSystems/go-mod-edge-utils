/*******************************************************************************
 * Copyright 2019 Dell Inc.
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

package container

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

// LoggerInterfaceName contains the name of the log.Logger implementation in the DIC.
var LoggerInterfaceName = di.TypeInstanceToName((*log.Logger)(nil))

// LoggerFrom helper function queries the DIC and returns the log.logger implementation.
func LoggerFrom(get di.Get) log.Logger {
	logger, ok := get(LoggerInterfaceName).(log.Logger)
	if !ok {
		return nil
	}

	return logger
}
