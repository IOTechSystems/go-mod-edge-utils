//
// Copyright (C) 2023 IOTech Ltd
//

package log

import (
	"fmt"
	"io"
	stdLog "log"
	"os"

	"github.com/go-kit/log"
)

// These constants identify the log levels in order of increasing severity.
const (
	TraceLog = "TRACE"
	DebugLog = "DEBUG"
	InfoLog  = "INFO"
	WarnLog  = "WARN"
	ErrorLog = "ERROR"
)

// Logger defines the interface for logging operations.
type Logger interface {
	// SetLogLevel sets minimum severity log level. If a logging method is called with a lower level of severity than
	// what is set, it will result in no output.
	SetLogLevel(logLevel string) error
	// LogLevel returns the current log level setting
	LogLevel() string
	// Debug logs a message at the DEBUG severity level
	Debug(msg string, args ...any)
	// Error logs a message at the ERROR severity level
	Error(msg string, args ...any)
	// Info logs a message at the INFO severity level
	Info(msg string, args ...any)
	// Trace logs a message at the TRACE severity level
	Trace(msg string, args ...any)
	// Warn logs a message at the WARN severity level
	Warn(msg string, args ...any)
	// Debugf logs a formatted message at the DEBUG severity level
	Debugf(msg string, args ...any)
	// Errorf logs a formatted message at the ERROR severity level
	Errorf(msg string, args ...any)
	// Infof logs a formatted message at the INFO severity level
	Infof(msg string, args ...any)
	// Tracef logs a formatted message at the TRACE severity level
	Tracef(msg string, args ...any)
	// Warnf logs a formatted message at the WARN severity level
	Warnf(msg string, args ...any)
}

type logger struct {
	owningServiceName string
	logLevel          *string
	rootLogger        log.Logger
	levelLoggers      map[string]log.Logger
}

// InitLogger creates an instance of Logger
func InitLogger(owningServiceName string, logLevel string, logWriter io.Writer) Logger {
	if !isValidLogLevel(logLevel) {
		logLevel = InfoLog
	}

	// Set up logger
	l := logger{
		owningServiceName: owningServiceName,
		logLevel:          &logLevel,
	}

	if logWriter == nil {
		logWriter = os.Stdout
	}
	l.rootLogger = log.NewLogfmtLogger(logWriter)
	l.rootLogger = log.WithPrefix(
		l.rootLogger,
		"ts",
		log.DefaultTimestamp,
		"app",
		owningServiceName,
		"source",
		log.Caller(5))

	// Set up the loggers
	l.levelLoggers = map[string]log.Logger{}

	for _, logLevel := range logLevels() {
		l.levelLoggers[logLevel] = log.WithPrefix(l.rootLogger, "level", logLevel)
	}

	return l
}

// LogLevels returns an array of the possible log levels in order from most to least verbose.
func logLevels() []string {
	return []string{
		TraceLog,
		DebugLog,
		InfoLog,
		WarnLog,
		ErrorLog}
}

func isValidLogLevel(l string) bool {
	for _, name := range logLevels() {
		if name == l {
			return true
		}
	}
	return false
}

func (l logger) log(logLevel string, formatted bool, msg string, args ...any) {
	// Check minimum log level
	for _, name := range logLevels() {
		if name == *l.logLevel {
			break
		}
		if name == logLevel {
			return
		}
	}

	if args == nil {
		args = []any{"msg", msg}
	} else if formatted {
		args = []any{"msg", fmt.Sprintf(msg, args...)}
	} else {
		if len(args)%2 == 1 {
			// add an empty string to keep k/v pairs correct
			args = append(args, "")
		}
		if len(msg) > 0 {
			args = append(args, "msg", msg)
		}
	}

	err := l.levelLoggers[logLevel].Log(args...)
	if err != nil {
		stdLog.Fatal(err.Error())
		return
	}

}

func (l logger) SetLogLevel(logLevel string) error {
	if isValidLogLevel(logLevel) {
		*l.logLevel = logLevel

		return nil
	}

	return fmt.Errorf("invalid log level `%s`", logLevel)
}

func (l logger) LogLevel() string {
	if l.logLevel == nil {
		return ""
	}
	return *l.logLevel
}

func (l logger) Info(msg string, args ...any) {
	l.log(InfoLog, false, msg, args...)
}

func (l logger) Trace(msg string, args ...any) {
	l.log(TraceLog, false, msg, args...)
}

func (l logger) Debug(msg string, args ...any) {
	l.log(DebugLog, false, msg, args...)
}

func (l logger) Warn(msg string, args ...any) {
	l.log(WarnLog, false, msg, args...)
}

func (l logger) Error(msg string, args ...any) {
	l.log(ErrorLog, false, msg, args...)
}

func (l logger) Infof(msg string, args ...any) {
	l.log(InfoLog, true, msg, args...)
}

func (l logger) Tracef(msg string, args ...any) {
	l.log(TraceLog, true, msg, args...)
}

func (l logger) Debugf(msg string, args ...any) {
	l.log(DebugLog, true, msg, args...)
}

func (l logger) Warnf(msg string, args ...any) {
	l.log(WarnLog, true, msg, args...)
}

func (l logger) Errorf(msg string, args ...any) {
	l.log(ErrorLog, true, msg, args...)
}
