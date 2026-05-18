//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConcurrency_StartOrJoinPublishSubscribe stresses the hot paths
// under -race: many goroutines on shared topics, all expected to drain
// without races, deadlocks, or stale-map entries.
func TestConcurrency_StartOrJoinPublishSubscribe(t *testing.T) {
	tr := New(context.Background(), 50*time.Millisecond, time.Second, newTestLogger(t))

	const (
		topics     = 8
		publishers = 4
		each       = 200
	)
	var wg sync.WaitGroup
	for i := 0; i < topics; i++ {
		topic := topicName(i)
		for p := 0; p < publishers; p++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				j, _ := tr.StartOrJoin(topic)
				for k := 0; k < each; k++ {
					j.Publish(k)
				}
				j.Finish("done")
			}()
		}
	}
	wg.Wait()

	require.Eventually(t, func() bool {
		tr.mu.RLock()
		empty := len(tr.jobs) == 0
		tr.mu.RUnlock()
		return empty
	}, 5*time.Second, 10*time.Millisecond)
}

// TestConcurrency_StartReplaceRetainedDuringPublish — Start replaces a
// retained Job with a fresh one; concurrent (no-op) Publish on the
// orphaned Job + (live) Publish on the replacement must not race, and
// the old Job's cleanup must not evict the replacement.
func TestConcurrency_StartReplaceRetainedDuringPublish(t *testing.T) {
	tr := New(context.Background(), 30*time.Millisecond, time.Second, newTestLogger(t))

	old, _ := tr.Start("topic-orphan")
	old.Finish("orphan terminal")

	replacement, isNew := tr.Start("topic-orphan")
	require.True(t, isNew)
	assert.NotSame(t, old, replacement)

	// Publish on both; the old (terminal) is a no-op.
	var publishCount atomic.Int64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				old.Publish(publishCount.Add(1))
			}
		}
	}()
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				replacement.Publish(publishCount.Add(1))
			}
		}
	}()

	time.Sleep(80 * time.Millisecond) // > retention; let old's cleanup fire
	close(stop)
	wg.Wait()

	tr.mu.RLock()
	stored, ok := tr.jobs["topic-orphan"]
	tr.mu.RUnlock()
	require.True(t, ok)
	assert.Same(t, replacement, stored)

	replacement.Finish("replacement terminal")
	require.Eventually(t, func() bool {
		tr.mu.RLock()
		_, ok := tr.jobs["topic-orphan"]
		tr.mu.RUnlock()
		return !ok
	}, time.Second, 5*time.Millisecond)
}

// TestConcurrency_SubscribeHoldsLockThroughAttach — regression: Subscribe
// must hold Tracker.mu (read) across both lookup AND j.mu acquisition
// inside attach. Otherwise a concurrent Start replacing a retained
// entry would slip into the gap and the subscriber would attach to the
// stale Job.
func TestConcurrency_SubscribeHoldsLockThroughAttach(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tr := New(ctx, 10*time.Second, time.Second, newTestLogger(t))

	jobA, _ := tr.Start("topic")
	jobA.Finish("a") // retained → Subscribe returns immediately after attach

	startAttempted := make(chan struct{})
	startReturned := make(chan struct{})
	var observedDuringHook *Job

	tr.attachHookForTest = func() {
		// Drive a concurrent Start while Subscribe is between lookup
		// and attach. With the lock held, this Start (needs
		// Tracker.mu.Lock to replace the retained entry) must block.
		go func() {
			close(startAttempted)
			tr.Start("topic")
			close(startReturned)
		}()
		<-startAttempted

		select {
		case <-startReturned:
			t.Errorf("Start replacement landed while Subscribe held the read lock")
		case <-time.After(50 * time.Millisecond):
			// Expected: blocked.
		}

		observedDuringHook = tr.jobs["topic"]
	}

	_, c, cancelReq := newSubscribeFixture(t)
	defer cancelReq()
	require.NoError(t, tr.Subscribe(c, "topic"))

	<-startReturned // Subscribe released the read lock; replacement lands

	assert.Same(t, jobA, observedDuringHook,
		"hook must observe original Job under Subscribe's read lock")

	tr.mu.RLock()
	stored := tr.jobs["topic"]
	tr.mu.RUnlock()
	assert.NotSame(t, jobA, stored)
}

func topicName(i int) string {
	return "topic-" + string(rune('a'+i))
}
