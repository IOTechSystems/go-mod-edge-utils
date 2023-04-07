//
// Copyright (C) 2023 IOTech Ltd
//

package logger

import (
	"fmt"
	"io"
	stdLog "log"
	"os"

	"github.com/IOTechSystems/go-mod-gui-utils/models"

	"github.com/go-kit/log"
)

type Logger struct {
	owningServiceName string
	logLevel          *string
	rootLogger        log.Logger
	levelLoggers      map[string]log.Logger
}

// InitLogger creates an instance of Logger
func InitLogger(owningServiceName string, logLevel string, logWriter io.Writer) Logger {
	if !isValidLogLevel(logLevel) {
		logLevel = models.InfoLog
	}

	// Set up logging client
	lc := Logger{
		owningServiceName: owningServiceName,
		logLevel:          &logLevel,
	}

	if logWriter == nil {
		logWriter = os.Stdout
	}
	lc.rootLogger = log.NewLogfmtLogger(logWriter)
	lc.rootLogger = log.WithPrefix(
		lc.rootLogger,
		"ts",
		log.DefaultTimestamp,
		"app",
		owningServiceName,
		"source",
		log.Caller(5))

	// Set up the loggers
	lc.levelLoggers = map[string]log.Logger{}

	for _, logLevel := range logLevels() {
		lc.levelLoggers[logLevel] = log.WithPrefix(lc.rootLogger, "level", logLevel)
	}

	return lc
}

// LogLevels returns an array of the possible log levels in order from most to least verbose.
func logLevels() []string {
	return []string{
		models.TraceLog,
		models.DebugLog,
		models.InfoLog,
		models.WarnLog,
		models.ErrorLog}
}

func isValidLogLevel(l string) bool {
	for _, name := range logLevels() {
		if name == l {
			return true
		}
	}
	return false
}

func (lc Logger) log(logLevel string, formatted bool, msg string, args ...interface{}) {
	// Check minimum log level
	for _, name := range logLevels() {
		if name == *lc.logLevel {
			break
		}
		if name == logLevel {
			return
		}
	}

	if args == nil {
		args = []interface{}{"msg", msg}
	} else if formatted {
		args = []interface{}{"msg", fmt.Sprintf(msg, args...)}
	} else {
		if len(args)%2 == 1 {
			// add an empty string to keep k/v pairs correct
			args = append(args, "")
		}
		if len(msg) > 0 {
			args = append(args, "msg", msg)
		}
	}

	err := lc.levelLoggers[logLevel].Log(args...)
	if err != nil {
		stdLog.Fatal(err.Error())
		return
	}

}

func (lc Logger) SetLogLevel(logLevel string) error {
	if isValidLogLevel(logLevel) {
		*lc.logLevel = logLevel

		return nil
	}

	return fmt.Errorf("invalid log level `%s`", logLevel)
}

func (lc Logger) LogLevel() string {
	if lc.logLevel == nil {
		return ""
	}
	return *lc.logLevel
}

func (lc Logger) Info(msg string, args ...interface{}) {
	lc.log(models.InfoLog, false, msg, args...)
}

func (lc Logger) Trace(msg string, args ...interface{}) {
	lc.log(models.TraceLog, false, msg, args...)
}

func (lc Logger) Debug(msg string, args ...interface{}) {
	lc.log(models.DebugLog, false, msg, args...)
}

func (lc Logger) Warn(msg string, args ...interface{}) {
	lc.log(models.WarnLog, false, msg, args...)
}

func (lc Logger) Error(msg string, args ...interface{}) {
	lc.log(models.ErrorLog, false, msg, args...)
}

func (lc Logger) Infof(msg string, args ...interface{}) {
	lc.log(models.InfoLog, true, msg, args...)
}

func (lc Logger) Tracef(msg string, args ...interface{}) {
	lc.log(models.TraceLog, true, msg, args...)
}

func (lc Logger) Debugf(msg string, args ...interface{}) {
	lc.log(models.DebugLog, true, msg, args...)
}

func (lc Logger) Warnf(msg string, args ...interface{}) {
	lc.log(models.WarnLog, true, msg, args...)
}

func (lc Logger) Errorf(msg string, args ...interface{}) {
	lc.log(models.ErrorLog, true, msg, args...)
}
