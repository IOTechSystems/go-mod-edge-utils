//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// captureLogger records Errorf calls so the diagnostic log emitted by
// applyWriteDeadline (which WriteSSEHeaders delegates to) can be
// asserted. Other levels are inherited from NopeLogger.
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
// exercising the "writer supports deadlines" branch.
type deadlineCapableRecorder struct {
	*httptest.ResponseRecorder
}

func (d *deadlineCapableRecorder) SetWriteDeadline(_ time.Time) error { return nil }

func newSSEContext(rw http.ResponseWriter) echo.Context {
	req := httptest.NewRequest(http.MethodGet, "/sse", nil)
	return echo.New().NewContext(req, rw)
}

func TestWriteSSEHeaders_SucceedsWhenDeadlineSupported(t *testing.T) {
	rec := &deadlineCapableRecorder{ResponseRecorder: httptest.NewRecorder()}
	lc := &captureLogger{}

	err := WriteSSEHeaders(newSSEContext(rec), time.Second, lc)

	require.NoError(t, err, "writer supports SetWriteDeadline; WriteSSEHeaders must succeed")
	assert.Empty(t, lc.snapshotErrorfs(), "successful path must not log Errorf")
	assert.Equal(t, "text/event-stream", rec.Header().Get(echo.HeaderContentType))
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", rec.Header().Get("Connection"))
}

func TestWriteSSEHeaders_FailsWhenDeadlineUnsupported(t *testing.T) {
	// Plain httptest.ResponseRecorder supports Flush but does NOT support
	// SetWriteDeadline — http.ResponseController returns ErrNotSupported.
	rec := httptest.NewRecorder()
	lc := &captureLogger{}

	err := WriteSSEHeaders(newSSEContext(rec), time.Second, lc)

	require.Error(t, err, "missing SetWriteDeadline must be surfaced as a returned error")
	errs := lc.snapshotErrorfs()
	if assert.Len(t, errs, 1, "exactly one diagnostic Errorf is expected when SetWriteDeadline is unsupported") {
		assert.Contains(t, errs[0], "write deadline",
			"diagnostic must name the failed capability so ops can diagnose")
	}
}
