//
// Copyright (C) 2024 IOTech Ltd
//

package certificates

import (
	"os"
)

// FileWriter is an interface that provides abstraction for writing certificate files to disk. This abstraction is
// necessary for appropriate mocking of this functionality for unit tests
type FileWriter interface {
	Write(fileName string, contents []byte, permissions os.FileMode) error
}

// NewFileWriter will instantiate an instance of FileWriter for persisting the given file to disk.
func NewFileWriter() FileWriter {
	return fileWriter{}
}

type fileWriter struct{}

// Write performs the actual write operation based on the parameters supplied
func (w fileWriter) Write(fileName string, contents []byte, permissions os.FileMode) (err error) {
	return os.WriteFile(fileName, contents, permissions)
}
