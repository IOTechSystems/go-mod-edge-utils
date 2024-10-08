//
// Copyright (C) 2024 IOTech Ltd
//

package seed

import (
	"fmt"
	"os"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

type DirectoryHandler struct {
	loggingClient log.Logger
}

func NewDirectoryHandler(lc log.Logger) DirectoryHandler {
	return DirectoryHandler{
		loggingClient: lc,
	}
}

func (h DirectoryHandler) Create(path string) error {
	// Remove eventual previous PKI setup directory
	// Create a new empty PKI setup directory
	h.loggingClient.Debug("New CA creation requested by configuration")
	h.loggingClient.Debug("Cleaning up CA PKI setup directory")

	err := os.RemoveAll(path) // Remove pkiCaDir
	if err != nil {
		return fmt.Errorf("Attempted removal of existing CA PKI config directory: %s (%s)", path, err)
	}

	h.loggingClient.Debug(fmt.Sprintf("Creating CA PKI setup directory: %s", path))
	err = os.MkdirAll(path, 0750) // Create pkiCaDir
	if err != nil {
		return fmt.Errorf("Failed to create the CA PKI configuration directory: %s (%s)", path, err)
	}
	return nil
}

func (h DirectoryHandler) Verify(path string) error {
	h.loggingClient.Debug("No new CA creation requested by configuration")

	// Is the CA there? (if nil then OK... but could be something else than a directory)
	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("CA PKI setup directory does not exist: %s", path)
		}
		return fmt.Errorf("CA PKI setup directory cannot be reached: %s (%s)", path, err)
	}
	if stat.IsDir() {
		h.loggingClient.Debug(fmt.Sprintf("Existing CA PKI setup directory: %s", path))
	} else {
		return fmt.Errorf("Existing CA PKI setup directory is not a directory: %s", path)
	}
	return nil
}
