//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import (
	"context"
	"sync"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

const (
	// subscriberChanBuffer is the per-subscriber channel size. Sized to 1
	// because Publish uses latest-wins semantics: a slow subscriber's
	// pending stale payload is drained and replaced by the newer one, so
	// the channel always carries the most recent state. Larger buffers
	// would queue intermediate snapshots that callers should treat as
	// already superseded (see README "Scope").
	subscriberChanBuffer = 1

	defaultRetention = 30 * time.Second
	defaultHeartbeat = 30 * time.Second
)

// Tracker is a per-service registry of in-flight and recently-finished
// Jobs, keyed by topic. See README for the API and the Start vs
// StartOrJoin selection rule.
//
// Tracker holds the caller-supplied ctx directly (no WithCancel wrap):
// its lifecycle matches the parent, so cancelling the parent wakes
// every watchAndCleanup goroutine and every in-flight Subscribe loop.
// If a future use case needs to shut down a Tracker independent of its
// parent, reintroduce a cancel field + Close() — it isn't needed today.
type Tracker struct {
	mu        sync.RWMutex
	jobs      map[string]*Job
	retention time.Duration
	heartbeat time.Duration
	ctx       context.Context
	lc        log.Logger

	// attachHookForTest is invoked by Subscribe between registry lookup
	// and attach, while Tracker.mu (read) is still held. Tests only.
	attachHookForTest func()
}

// New constructs a Tracker bound to ctx. Zero or negative durations use
// 30s defaults. Cancelling ctx wakes all cleanup goroutines.
func New(ctx context.Context, retention, heartbeat time.Duration, lc log.Logger) *Tracker {
	if retention <= 0 {
		retention = defaultRetention
	}
	if heartbeat <= 0 {
		heartbeat = defaultHeartbeat
	}

	return &Tracker{
		jobs:      make(map[string]*Job),
		retention: retention,
		heartbeat: heartbeat,
		ctx:       ctx,
		lc:        lc,
	}
}

// Start is the trigger-driven entry. Running entry → join; retained
// terminal → discard and create fresh; absent → create.
//
// The retained-discard case is the load-bearing difference vs
// StartOrJoin: a new trigger after the previous run finished must
// produce a fresh run, not silently observe the prior terminal.
func (t *Tracker) Start(topic string) (job *Job, isNew bool) {
	t.mu.Lock()
	if existing, ok := t.jobs[topic]; ok {
		if !existing.isTerminal() {
			t.mu.Unlock()
			return existing, false
		}
		// Retained — replace. The previous Job's cleanup goroutine sees
		// jobs[topic] != itself in deleteIfIdentity and leaves us alone.
	}
	j := newJob()
	t.jobs[topic] = j
	t.mu.Unlock()

	go t.watchAndCleanup(topic, j)
	return j, true
}

// StartOrJoin is the subscriber-driven entry. Any existing entry —
// running or retained terminal — is reused; absent → create.
//
// The retained-reuse case is the load-bearing difference vs Start: late
// subscribers on a compute-once flow observe the cached terminal
// instead of retriggering work.
func (t *Tracker) StartOrJoin(topic string) (job *Job, isNew bool) {
	t.mu.Lock()
	if existing, ok := t.jobs[topic]; ok {
		t.mu.Unlock()
		return existing, false
	}

	j := newJob()
	t.jobs[topic] = j
	t.mu.Unlock()

	go t.watchAndCleanup(topic, j)
	return j, true
}

// watchAndCleanup removes jobs[topic] after Finish + retention, or
// immediately on Tracker shutdown. Both paths gate on an identity check
// so that a Start replacement (retained → fresh) survives the old Job's
// cleanup pass.
func (t *Tracker) watchAndCleanup(topic string, j *Job) {
	select {
	case <-j.finished:
	case <-t.ctx.Done():
		t.deleteIfIdentity(topic, j)
		return
	}

	timer := time.NewTimer(t.retention)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-t.ctx.Done():
	}

	t.deleteIfIdentity(topic, j)
}

func (t *Tracker) deleteIfIdentity(topic string, j *Job) {
	t.mu.Lock()
	if t.jobs[topic] == j {
		delete(t.jobs, topic)
	}
	t.mu.Unlock()
}
