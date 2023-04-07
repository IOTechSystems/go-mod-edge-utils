//
// Copyright (C) 2023 IOTech Ltd
//

package logger

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/IOTechSystems/go-mod-gui-utils/models"

	"github.com/stretchr/testify/assert"
)

func TestIsValidLogLevel(t *testing.T) {
	var tests = []struct {
		level string
		res   bool
	}{
		{models.TraceLog, true},
		{models.DebugLog, true},
		{models.InfoLog, true},
		{models.WarnLog, true},
		{models.ErrorLog, true},
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
	expectedLogLevel := models.InfoLog
	lc := InitLogger("testService", expectedLogLevel, buf)

	expectedLogMsg := "test info log"
	lc.log(expectedLogLevel, false, expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + models.InfoLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestLogFormatted(t *testing.T) {
	buf := &bytes.Buffer{}
	expectedLogLevel := models.TraceLog
	lc := InitLogger("testService", expectedLogLevel, buf)

	expectedLogMsg := "test info log with msg is %s"
	expectedStrVar := "abc123"
	lc.log(expectedLogLevel, true, expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + models.TraceLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestSetLogLevel(t *testing.T) {
	expectedLogLevel := models.TraceLog
	lc := InitLogger("testService", expectedLogLevel, nil)
	assert.Equal(t, expectedLogLevel, lc.LogLevel())
}

func TestLogLevel(t *testing.T) {
	expectedLogLevel := models.DebugLog
	lc := InitLogger("testService", expectedLogLevel, nil)
	assert.Equal(t, expectedLogLevel, lc.LogLevel())
}

func TestInfo(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.InfoLog, buf)

	expectedLogMsg := "test info log"
	lc.Info(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + models.InfoLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestTrace(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.TraceLog, buf)

	expectedLogMsg := "test trace log"
	lc.Trace(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + models.TraceLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestDebug(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.DebugLog, buf)

	expectedLogMsg := "test debug log"
	lc.Debug(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + models.DebugLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestWarn(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.WarnLog, buf)

	expectedLogMsg := "test warn log"
	lc.Warn(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + models.WarnLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestError(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.ErrorLog, buf)

	expectedLogMsg := "test error log"
	lc.Error(expectedLogMsg)

	result := buf.String()
	expectedLevel := "level=" + models.ErrorLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, expectedLogMsg); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLogMsg, result)
	}
}

func TestInfof(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.InfoLog, buf)

	expectedLogMsg := "test info log with msg is %s"
	expectedStrVar := "abc123"
	lc.Infof(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + models.InfoLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestTracef(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.TraceLog, buf)

	expectedLogMsg := "test trace log with msg is %s"
	expectedStrVar := "abc123"
	lc.Tracef(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + models.TraceLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestDebugf(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.DebugLog, buf)

	expectedLogMsg := "test debug log with msg is %s"
	expectedStrVar := "abc123"
	lc.Debugf(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + models.DebugLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestWarnf(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.WarnLog, buf)

	expectedLogMsg := "test warn log with msg is %s"
	expectedStrVar := "abc123"
	lc.Warnf(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + models.WarnLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}

func TestErrorf(t *testing.T) {
	buf := &bytes.Buffer{}
	lc := InitLogger("testService", models.ErrorLog, buf)

	expectedLogMsg := "test error log with msg is %s"
	expectedStrVar := "abc123"
	lc.Errorf(expectedLogMsg, expectedStrVar)

	result := buf.String()
	expectedLevel := "level=" + models.ErrorLog
	if exists := strings.Contains(result, expectedLevel); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", expectedLevel, result)
	}
	if exists := strings.Contains(result, fmt.Sprintf(expectedLogMsg, expectedStrVar)); !exists {
		t.Errorf("Expected %s exists in the writer. Got: %s", fmt.Sprintf(expectedLogMsg, expectedStrVar), result)
	}
}
