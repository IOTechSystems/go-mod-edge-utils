//
// Copyright (C) 2024 IOTech Ltd
//

package auditlog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	TimestampKey   = "ts"
	ActorKey       = "actor"
	ActionKey      = "action"
	DescriptionKey = "desc"
	DetailsKey     = "details"
	SeverityKey    = "severity"
)

type logger struct {
	owningServiceName string
	coverageLevel     *slog.LevelVar
	logger            *slog.Logger
}

//// Entry builds up an audit log entry with the following fields
//type Entry struct {
//	// actor indicates the identity of the user or system that initiated the action
//	actor string
//	// action indicates the type of action that was performed
//	action ActionType
//	// description contains a brief description of the action
//	description string
//	// details contains extra information regarding the action
//	details any
//	// severity indicates the severity of the action
//	severity Severity
//}

// InitLogger creates an instance of Logger
func InitLogger(owningServiceName string, coverageLevel string, logWriter io.Writer, config Configuration) Logger {
	// Initialize the Logger with the given coverage level
	l := logger{
		owningServiceName: owningServiceName,
	}
	coverageLevel = strings.ToUpper(coverageLevel)
	l.SetCoverageLevel(coverageLevel)

	// Set up a default log writer if it is not provided
	if logWriter == nil {
		config.setDefault()
		// Add the service name to the log file name as a prefix
		fileName := owningServiceName + "-" + config.FileName
		if canCreateFileInDir(config.StorageDir, fileName) {
			// Set up the file writer and log rotation configuration
			logWriter = &lumberjack.Logger{
				Filename:   config.StorageDir + "/" + fileName,
				MaxSize:    config.MaxSize, // megabytes
				MaxBackups: config.MaxBackups,
				MaxAge:     config.MaxAge, //days
				LocalTime:  true,
				Compress:   true, // disabled by default
			}
		} else {
			logWriter = os.Stdout
		}
	}

	// Set up the logger
	l.logger = slog.New(slog.NewTextHandler(logWriter, &slog.HandlerOptions{
		Level:       l.coverageLevel,
		ReplaceAttr: replaceAttr,
	}))

	return &l
}

// SetCoverageLevel sets the coverage level for the logger
func (l *logger) SetCoverageLevel(coverageLevel string) {
	// Use BASE coverage level if the given coverage level is invalid
	if !isValidCoverageLevel(coverageLevel) {
		coverageLevel = BaseCoverage
	}
	// Set up the coverage level for this program
	var programLevel = new(slog.LevelVar)
	programLevel.Set(slogLevelFromString(coverageLevel))
	l.coverageLevel = programLevel
}

// LogBase adds an audit log entry to the log writer with base coverage level
func (l *logger) LogBase(severity Severity, actor string, action ActionType, description string, details any) {
	l.log(BaseCoverageLevel, severity, actor, action, description, details)
}

// LogAdvanced adds an audit log entry to the log writer with advanced coverage level
func (l *logger) LogAdvanced(severity Severity, actor string, action ActionType, description string, details any) {
	l.log(AdvancedCoverageLevel, severity, actor, action, description, details)
}

// LogFull adds an audit log entry to the log writer with full coverage level
func (l *logger) LogFull(severity Severity, actor string, action ActionType, description string, details any) {
	l.log(FullCoverageLevel, severity, actor, action, description, details)
}

// log adds an audit log entry to the log writer with the given coverage level
func (l *logger) log(coverageLevel slog.Level, severity Severity, actor string, action ActionType, description string, details any) {
	// Set severity to NORMAL if the given severity is invalid or empty
	if severity != SeverityCritical && severity != SeverityNormal && severity != SeverityMinor {
		severity = SeverityNormal
	}

	// Set action to UNKNOWN if the given action is invalid or empty
	if !isValidActionType(action) {
		action = ActionTypeUnknown
	}

	attrs := []slog.Attr{
		slog.Time(TimestampKey, time.Now()),
		slog.String(appKey, l.owningServiceName),
		slog.String(SeverityKey, string(severity)),
		slog.String(ActorKey, actor),
		slog.String(ActionKey, string(action)),
		slog.String(DescriptionKey, description),
	}

	// Set details to an empty string if it is nil
	if details != nil && details != "" {
		attrs = append(attrs, slog.Any(DetailsKey, details))
	}

	l.logger.LogAttrs(
		context.Background(),
		coverageLevel,
		"", // omit the message as it is not used in the audit log
		attrs...,
	)
}

// canCreateFileInDir is a helper function to check if a file can be created in the given directory
func canCreateFileInDir(dirPath string, fileName string) bool {
	// Check if the file exists
	fullPath := filepath.Join(dirPath, fileName)
	if _, err := os.Stat(fullPath); err == nil {
		// file exists
		return checkFileReadWrite(fullPath)
	} else if !os.IsNotExist(err) {
		// other errors
		return false
	}

	// Create a temporary file in the directory to check if it is writable
	tempFile, err := os.CreateTemp(dirPath, "test-*")
	if err != nil {
		return false
	}
	tempFile.Close()

	// Remove the temporary file
	os.Remove(tempFile.Name())

	return true
}

// checkFileReadWrite is a helper function to check if an exiting file can be read and written
func checkFileReadWrite(fullPath string) bool {
	// Try to open the file in read/write mode
	file, err := os.OpenFile(fullPath, os.O_RDWR, 0666)
	if err != nil {
		return false
	}
	defer file.Close()
	return true
}
