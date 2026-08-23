//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// recordingLogger captures formatted log lines into a single string for
// assertion. Satisfies log.Logger with no-ops for non-formatted methods we
// don't exercise.
type recordingLogger struct {
	log.Logger
	out strings.Builder
}

func (r *recordingLogger) Infof(msg string, args ...any)  { r.write("INFO", msg, args) }
func (r *recordingLogger) Errorf(msg string, args ...any) { r.write("ERROR", msg, args) }
func (r *recordingLogger) Debugf(msg string, args ...any) { r.write("DEBUG", msg, args) }
func (r *recordingLogger) Warnf(_ string, _ ...any)       {}
func (r *recordingLogger) Tracef(_ string, _ ...any)      {}
func (r *recordingLogger) LogLevel() string               { return log.DebugLog }
func (r *recordingLogger) write(level, msg string, args []any) {
	r.out.WriteString(level + " " + fmt.Sprintf(msg, args...) + "\n")
}

func TestLogging_PassesThrough(t *testing.T) {
	lc := &recordingLogger{}

	mw := Logging(lc)

	called := false
	inner := sdkmcp.MethodHandler(func(_ context.Context, method string, _ sdkmcp.Request) (sdkmcp.Result, error) {
		called = true
		assert.Equal(t, "tools/call", method)
		return nil, nil
	})

	wrapped := mw(inner)
	_, err := wrapped(context.Background(), "tools/call", nil)

	require.NoError(t, err)
	assert.True(t, called, "middleware must invoke wrapped handler")
	assert.Contains(t, lc.out.String(), "tools/call", "log line must include method name")
}
