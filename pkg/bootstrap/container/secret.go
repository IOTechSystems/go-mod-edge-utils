/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2020 Intel Corp.
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
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
)

// SecretProviderName contains the name of the interfaces.SecretProvider implementation in the DIC.
var SecretProviderName = di.TypeInstanceToName((*interfaces.SecretProvider)(nil))

// SecretProviderFrom helper function queries the DIC and returns the interfaces.SecretProvider
// implementation.
func SecretProviderFrom(get di.Get) interfaces.SecretProvider {
	provider, ok := get(SecretProviderName).(interfaces.SecretProvider)
	if !ok {
		return nil
	}

	return provider
}
