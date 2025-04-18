//
// Copyright (c) 2022 Intel Corporation
// Copyright (C) 2024-2025 IOTech Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package container

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/rest/interfaces"
)

// SecurityProxyAuthClientName contains the name of the AuthClient's implementation in the DIC.
var SecurityProxyAuthClientName = di.TypeInstanceToName((*interfaces.AuthClient)(nil))

// SecurityProxyAuthClientFrom helper function queries the DIC and returns the AuthClient's implementation.
func SecurityProxyAuthClientFrom(get di.Get) interfaces.AuthClient {
	if get(SecurityProxyAuthClientName) == nil {
		return nil
	}

	return get(SecurityProxyAuthClientName).(interfaces.AuthClient)
}
