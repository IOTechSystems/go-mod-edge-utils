//
// Copyright (C) 2023 IOTech Ltd
//

package log

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidLogLevel(t *testing.T) {
	var tests = []struct {
		level string
		res   bool
	}{
		{TraceLog, true},
		{DebugLog, true},
		{InfoLog, true},
		{WarnLog, true},
		{ErrorLog, true},
		{"EERROR", false},
		{"ERRORR", false},
		{"INF", false},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			r := isValidLogLevel(tt.level)
			if r != tt.res {
				t.Errorf("Level %s labeled as %v and should be %v",
					tt.level, r, tt.res)
			}
		})
	}
}

func TestLogNotFormatted(t *testing.T) {
	buf := &bytes.Buffer{}
	expectedLogLevel := InfoLog
	log := InitLogger("testService", expectedLogLevel, buf)
	l := log.(logger) // convert to logger struct

	expectedLogMsg := "test info log"
	l.log(expectedLogLevel, false, expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + InfoLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestLogFormatted(t *testing.T) {
	buf := &bytes.Buffer{}
	expectedLogLevel := TraceLog
	log := InitLogger("testService", expectedLogLevel, buf)
	l := log.(logger) // convert to logger struct

	expectedLogMsg := "test info log with msg is %s"
	expectedStrVar := "abc123"
	l.log(expectedLogLevel, true, expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + TraceLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestSetLogLevel(t *testing.T) {
	expectedLogLevel := TraceLog
	logger := InitLogger("testService", expectedLogLevel, nil)
	assert.Equal(t, expectedLogLevel, logger.LogLevel())
}

func TestLogLevel(t *testing.T) {
	expectedLogLevel := DebugLog
	logger := InitLogger("testService", expectedLogLevel, nil)
	assert.Equal(t, expectedLogLevel, logger.LogLevel())
}

func TestInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", InfoLog, buf)

	expectedLogMsg := "test info log"
	logger.Info(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + InfoLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestTrace(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", TraceLog, buf)

	expectedLogMsg := "test trace log"
	logger.Trace(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + TraceLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestDebug(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", DebugLog, buf)

	expectedLogMsg := "test debug log"
	logger.Debug(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + DebugLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestWarn(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", WarnLog, buf)

	expectedLogMsg := "test warn log"
	logger.Warn(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + WarnLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestError(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", ErrorLog, buf)

	expectedLogMsg := "test error log"
	logger.Error(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + ErrorLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestInfof(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", InfoLog, buf)

	expectedLogMsg := "test info log with msg is %s"
	expectedStrVar := "abc123"
	logger.Infof(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + InfoLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestTracef(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", TraceLog, buf)

	expectedLogMsg := "test trace log with msg is %s"
	expectedStrVar := "abc123"
	logger.Tracef(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + TraceLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestDebugf(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", DebugLog, buf)

	expectedLogMsg := "test debug log with msg is %s"
	expectedStrVar := "abc123"
	logger.Debugf(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + DebugLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestWarnf(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", WarnLog, buf)

	expectedLogMsg := "test warn log with msg is %s"
	expectedStrVar := "abc123"
	logger.Warnf(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + WarnLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestErrorf(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := InitLogger("testService", ErrorLog, buf)

	expectedLogMsg := "test error log with msg is %s"
	expectedStrVar := "abc123"
	logger.Errorf(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + ErrorLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}
