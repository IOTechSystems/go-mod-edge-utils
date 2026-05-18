//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(_ *testing.T) log.Logger {
	return log.NewNopeLogger()
}

func TestNew_DefaultsAppliedForZeroOrNegative(t *testing.T) {
	tr := New(context.Background(), 0, -1, newTestLogger(t))
	require.NotNil(t, tr)
	assert.Equal(t, defaultRetention, tr.retention)
	assert.Equal(t, defaultHeartbeat, tr.heartbeat)
	assert.NotNil(t, tr.jobs)
}

func TestNew_HonoursPositiveDurations(t *testing.T) {
	tr := New(context.Background(), 7*time.Second, 3*time.Second, newTestLogger(t))
	assert.Equal(t, 7*time.Second, tr.retention)
	assert.Equal(t, 3*time.Second, tr.heartbeat)
}

// ---------- Start ----------

func TestTracker_Start_RegistersJobWhenAbsent(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))

	job, isNew := tr.Start("topic-a")

	require.NotNil(t, job)
	assert.True(t, isNew)
	tr.mu.RLock()
	stored, ok := tr.jobs["topic-a"]
	tr.mu.RUnlock()
	require.True(t, ok)
	assert.Same(t, job, stored)
}

func TestTracker_Start_JoinsRunningEntry(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))

	first, isNewFirst := tr.Start("topic-a")
	second, isNewSecond := tr.Start("topic-a") // first still running

	assert.True(t, isNewFirst)
	assert.False(t, isNewSecond)
	assert.Same(t, first, second, "running entry must be joined, not replaced")
}

func TestTracker_Start_ReplacesRetainedEntryWithFresh(t *testing.T) {
	// Load-bearing vs StartOrJoin: a trigger after retention must run
	// fresh, not observe the prior terminal.
	tr := New(context.Background(), 30*time.Second, time.Second, newTestLogger(t))

	first, _ := tr.Start("topic-a")
	first.Finish("ok")

	second, isNew := tr.Start("topic-a")
	assert.NotSame(t, first, second)
	assert.True(t, isNew)

	tr.mu.RLock()
	stored := tr.jobs["topic-a"]
	tr.mu.RUnlock()
	assert.Same(t, second, stored)
}

// ---------- StartOrJoin ----------

func TestTracker_StartOrJoin_CreatesWhenAbsent(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))

	job, isNew := tr.StartOrJoin("topic-a")
	require.NotNil(t, job)
	assert.True(t, isNew)
}

func TestTracker_StartOrJoin_ReusesRunning(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))

	first, isNewFirst := tr.StartOrJoin("topic-a")
	second, isNewSecond := tr.StartOrJoin("topic-a")

	assert.True(t, isNewFirst)
	assert.False(t, isNewSecond)
	assert.Same(t, first, second)
}

func TestTracker_StartOrJoin_ReusesRetained(t *testing.T) {
	// Load-bearing vs Start: late subscribers on a compute-once flow
	// must observe the cached terminal, not retrigger.
	tr := New(context.Background(), 30*time.Second, time.Second, newTestLogger(t))

	first, _ := tr.StartOrJoin("topic-a")
	first.Finish("ok")

	second, isNew := tr.StartOrJoin("topic-a")
	assert.Same(t, first, second)
	assert.False(t, isNew)
}

func TestTracker_StartOrJoin_ConcurrentRaceLandsOnSingleJob(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))

	const n = 32
	var (
		wg      sync.WaitGroup
		results = make([]*Job, n)
		newFlag = make([]bool, n)
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			j, isNew := tr.StartOrJoin("topic-race")
			results[i] = j
			newFlag[i] = isNew
		}(i)
	}
	wg.Wait()

	newCount := 0
	for _, b := range newFlag {
		if b {
			newCount++
		}
	}
	assert.Equal(t, 1, newCount)
	for i := 1; i < n; i++ {
		assert.Same(t, results[0], results[i])
	}
}

func TestTracker_Start_ConcurrentRaceLandsOnSingleJob(t *testing.T) {
	tr := New(context.Background(), time.Second, time.Second, newTestLogger(t))

	const n = 32
	var (
		wg      sync.WaitGroup
		results = make([]*Job, n)
		newFlag = make([]bool, n)
	)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			j, isNew := tr.Start("topic-race")
			results[i] = j
			newFlag[i] = isNew
		}(i)
	}
	wg.Wait()

	newCount := 0
	for _, b := range newFlag {
		if b {
			newCount++
		}
	}
	assert.Equal(t, 1, newCount)
	for i := 1; i < n; i++ {
		assert.Same(t, results[0], results[i])
	}
}

// ---------- cleanup ----------

func TestTracker_Cleanup_RemovesEntryAfterRetention(t *testing.T) {
	tr := New(context.Background(), 30*time.Millisecond, time.Second, newTestLogger(t))

	job, _ := tr.Start("topic-a")
	job.Finish("ok")

	require.Eventually(t, func() bool {
		tr.mu.RLock()
		_, ok := tr.jobs["topic-a"]
		tr.mu.RUnlock()
		return !ok
	}, time.Second, 5*time.Millisecond)
}

func TestTracker_Cleanup_RespectsTrackerContext(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	tr := New(parentCtx, 10*time.Second, time.Second, newTestLogger(t))

	job, _ := tr.Start("topic-a")
	job.Finish("ok")

	cancel()

	require.Eventually(t, func() bool {
		tr.mu.RLock()
		_, ok := tr.jobs["topic-a"]
		tr.mu.RUnlock()
		return !ok
	}, time.Second, 5*time.Millisecond)
}

func TestTracker_Cleanup_IdentityCheckPreservesReplacement(t *testing.T) {
	// Regression: Start's retained→fresh replacement must not be evicted
	// by the old Job's cleanup goroutine.
	tr := New(context.Background(), 20*time.Millisecond, time.Second, newTestLogger(t))

	old, _ := tr.Start("topic-a")
	old.Finish("old terminal")

	replacement, isNew := tr.Start("topic-a")
	require.True(t, isNew)
	assert.NotSame(t, old, replacement)

	time.Sleep(80 * time.Millisecond) // > retention; let old's cleanup fire

	tr.mu.RLock()
	stored, ok := tr.jobs["topic-a"]
	tr.mu.RUnlock()
	require.True(t, ok)
	assert.Same(t, replacement, stored)
}

func TestTracker_Cleanup_TrackerCtxCancelBeforeTerminal(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	tr := New(parentCtx, 10*time.Second, time.Second, newTestLogger(t))

	_, _ = tr.Start("topic-a") // never terminated
	cancel()

	require.Eventually(t, func() bool {
		tr.mu.RLock()
		_, ok := tr.jobs["topic-a"]
		tr.mu.RUnlock()
		return !ok
	}, time.Second, 5*time.Millisecond)
}
