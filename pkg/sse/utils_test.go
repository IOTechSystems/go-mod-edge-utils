//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// captureLogger records Errorf calls so the probe's diagnostic log can
// be asserted. Other levels are inherited from NopeLogger.
type captureLogger struct {
	log.NopeLogger
	mu      sync.Mutex
	errorfs []string
}

func (c *captureLogger) Errorf(msg string, args ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorfs = append(c.errorfs, fmt.Sprintf(msg, args...))
}

func (c *captureLogger) snapshotErrorfs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.errorfs))
	copy(out, c.errorfs)
	return out
}

// deadlineCapableRecorder is a httptest.ResponseRecorder that also
// satisfies http.ResponseController's SetWriteDeadline (as a no-op),
// exercising the "writer supports deadlines" probe branch.
type deadlineCapableRecorder struct {
	*httptest.ResponseRecorder
}

func (d *deadlineCapableRecorder) SetWriteDeadline(_ time.Time) error { return nil }

func newProbeContext(rw http.ResponseWriter) echo.Context {
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	return echo.New().NewContext(req, rw)
}

func TestProbeWriteDeadline_SilentWhenSupported(t *testing.T) {
	rec := &deadlineCapableRecorder{ResponseRecorder: httptest.NewRecorder()}
	lc := &captureLogger{}

	probeWriteDeadline(newProbeContext(rec), lc)

	assert.Empty(t, lc.snapshotErrorfs(),
		"a writer that supports SetWriteDeadline must not trigger the probe log")
}

func TestProbeWriteDeadline_LogsWhenUnsupported(t *testing.T) {
	// Plain httptest.ResponseRecorder supports Flush but does NOT support
	// SetWriteDeadline — http.ResponseController returns ErrNotSupported.
	rec := httptest.NewRecorder()
	lc := &captureLogger{}

	probeWriteDeadline(newProbeContext(rec), lc)

	errs := lc.snapshotErrorfs()
	if assert.Len(t, errs, 1, "exactly one probe Errorf is expected when SetWriteDeadline is unsupported") {
		assert.Contains(t, errs[0], "SetWriteDeadline",
			"probe log must name the missing capability so ops can diagnose")
	}
}

func TestWriteSSEHeaders_InvokesProbe(t *testing.T) {
	// Smoke test: WriteSSEHeaders must drive the probe so the diagnostic
	// fires at stream start regardless of which SSE entry point is in use.
	rec := httptest.NewRecorder()
	lc := &captureLogger{}

	WriteSSEHeaders(newProbeContext(rec), lc)

	errs := lc.snapshotErrorfs()
	if assert.Len(t, errs, 1, "WriteSSEHeaders must invoke the probe exactly once") {
		assert.Contains(t, strings.ToLower(errs[0]), "setwritedeadline")
	}
}
