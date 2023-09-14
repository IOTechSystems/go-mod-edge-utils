/*******************************************************************************
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

package flags

import (
	"flag"
	"fmt"
	"os"
)

// Common is an interface that defines AP for the common command-line flags used by most Edge services
type Common interface {
	ConfigDirectory() string
	ConfigFileName() string
	Parse([]string)
	Help()
}

// Default is the Default implementation of Common used by most Edge services
type Default struct {
	FlagSet         *flag.FlagSet
	additionalUsage string
	configDir       string
	configFileName  string
}

// NewWithUsage returns a Default struct.
func NewWithUsage(additionalUsage string) *Default {
	return &Default{
		FlagSet:         flag.NewFlagSet("", flag.ExitOnError),
		additionalUsage: additionalUsage,
	}
}

// New returns a Default struct with an empty additional usage string.
func New() *Default {
	return NewWithUsage("")
}

// Parse parses the passed in command-lie arguments looking to the default set of common flags
func (d *Default) Parse(arguments []string) {

	// Usage is provided by caller, so leaving individual usage blank here so not confusing where it comes from. )
	d.FlagSet.StringVar(&d.configFileName, "cf", "", "")
	d.FlagSet.StringVar(&d.configFileName, "configFile", "", "")
	d.FlagSet.StringVar(&d.configDir, "configDir", "", "")
	d.FlagSet.StringVar(&d.configDir, "cd", "", "")

	d.FlagSet.Usage = d.helpCallback

	err := d.FlagSet.Parse(arguments)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
}

// ConfigDirectory returns the directory where the config file(s) are located, if it was specified.
func (d *Default) ConfigDirectory() string {
	return d.configDir
}

// ConfigFileName returns the name of the local configuration file
func (d *Default) ConfigFileName() string {
	return d.configFileName
}

// Help displays the usage help message and exit.
func (d *Default) Help() {
	d.helpCallback()
}

// commonHelpCallback displays the help usage message and exits
func (d *Default) helpCallback() {
	fmt.Printf(
		"Usage: %s [options]\n"+
			"Server Options:\n"+
			"    -cf, --configFile <name>        Indicates name of the local configuration file. Defaults to configuration.json\n"+
			"    -cd, --configDir                Specify local configuration directory\n"+
			"%s\n"+
			"Common Options:\n"+
			"	-h, --help                      Show this message\n",
		os.Args[0], d.additionalUsage,
	)
	os.Exit(0)
}
