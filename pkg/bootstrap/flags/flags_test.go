/*******************************************************************************
 * Copyright 2020 Intel Corp.
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

package flags

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// newSUT creates and returns a new "system under test" instance.
func newSUT(args []string) *Default {
	actual := New()
	actual.Parse(args)
	return actual
}

func TestNewAllFlags(t *testing.T) {
	expectedConfigDirectory := "/res"
	expectedFileName := "config.yml"

	actual := newSUT(
		[]string{
			"-cd=" + expectedConfigDirectory,
			"-cf=" + expectedFileName,
		},
	)

	assert.Equal(t, expectedConfigDirectory, actual.ConfigDirectory())
	assert.Equal(t, expectedFileName, actual.ConfigFileName())
}

func TestNewDefaultsNoFlags(t *testing.T) {
	actual := newSUT([]string{})

	assert.Equal(t, "", actual.ConfigDirectory())
	assert.Equal(t, "", actual.ConfigFileName())
}

func TestDashR(t *testing.T) {
	expectedConfigDirectory := "/foo/ba-r/"
	actual := newSUT([]string{"-configDir", "/foo/ba-r/"})

	assert.Equal(t, expectedConfigDirectory, actual.ConfigDirectory())
}

func TestConfigDirEquals(t *testing.T) {
	expectedConfigDirectory := "/foo/ba-r/"
	actual := newSUT([]string{"-configDir=/foo/ba-r/"})

	assert.Equal(t, expectedConfigDirectory, actual.ConfigDirectory())
}
