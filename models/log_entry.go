//
// Copyright (C) 2023 IOTech Ltd
//

package models

// These constants identify the log levels in order of increasing severity.
const (
	TraceLog = "TRACE"
	DebugLog = "DEBUG"
	InfoLog  = "INFO"
	WarnLog  = "WARN"
	ErrorLog = "ERROR"
)

type LogEntry struct {
	Level         string        `json:"logLevel"`
	Args          []interface{} `json:"args"`
	OriginService string        `json:"originService"`
	Message       string        `json:"message"`
	Created       int64         `json:"created"`
}
