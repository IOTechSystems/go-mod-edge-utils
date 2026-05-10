//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	lc := log.InitLogger("test", "INFO", nil)
	return NewManager(context.Background(), lc, time.Second)
}

func TestCreateOrGetBroadcaster_ReturnsSameInstanceForSameTopic(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b1, isNew1 := m.CreateOrGetBroadcaster("topic1")
	require.True(t, isNew1)
	require.NotNil(t, b1)

	b2, isNew2 := m.CreateOrGetBroadcaster("topic1")
	require.False(t, isNew2)
	require.Same(t, b1, b2)
}

// When the last subscriber leaves, the broadcaster is removed from the manager.
func TestCreateOrGetBroadcaster_AutoRemovesOnLastUnsubscribe(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b, _ := m.CreateOrGetBroadcaster("topic1")
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	require.Eventually(t, func() bool {
		_, ok := m.GetBroadcaster("topic1")
		return !ok
	}, time.Second, 5*time.Millisecond, "broadcaster should be gone after last unsubscribe")
}

// Concurrent CreateOrGet calls on the same topic must converge on a single
// broadcaster — no caller should silently lose its instance to another's
// overwrite. Run with -race to validate.
func TestCreateOrGetBroadcaster_ConcurrentSameTopic(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	const goroutines = 32
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		seen   = make(map[*Broadcaster]struct{})
		newCnt int
	)
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			b, isNew := m.CreateOrGetBroadcaster("topic1")
			mu.Lock()
			seen[b] = struct{}{}
			if isNew {
				newCnt++
			}
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()

	require.Len(t, seen, 1, "all callers must converge on a single broadcaster instance")
	require.Equal(t, 1, newCnt, "exactly one caller must observe isNew=true")
}

// A subscriber that arrives between the Unsubscribe that drops the count to
// zero and the spawned handleNoSubscribers goroutine running must NOT cause
// the broadcaster to be removed: handleNoSubscribers re-checks the subscriber
// count under the lock before firing the auto-remove callback.
func TestAutoRemove_ReChecksSubscriberCountBeforeFiring(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b, _ := m.CreateOrGetBroadcaster("topic1")
	ch1 := b.Subscribe()

	// Resubscribe BEFORE handleNoSubscribers' goroutine has had a chance to
	// run. The race window is tiny but reliable in practice because
	// Unsubscribe spawns the cleanup goroutine and we Subscribe synchronously
	// in the same goroutine right after.
	b.Unsubscribe(ch1)
	ch2 := b.Subscribe()

	// Give handleNoSubscribers plenty of time to (incorrectly) fire if the
	// re-check is missing.
	require.Never(t, func() bool {
		_, ok := m.GetBroadcaster("topic1")
		return !ok
	}, 200*time.Millisecond, 10*time.Millisecond,
		"new subscriber arriving before handleNoSubscribers fires must keep the broadcaster alive")

	b.Unsubscribe(ch2)
}

func TestRemoveBroadcaster_RemovesEntry(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	m.CreateOrGetBroadcaster("topic1")
	m.RemoveBroadcaster("topic1")
	_, ok := m.GetBroadcaster("topic1")
	require.False(t, ok)
}

// removeBroadcasterIfEmpty must NOT remove a broadcaster that has live
// subscribers. This is the manager-level guard that backstops the
// broadcaster-level RLock re-check: if a new Subscribe slips in between the
// RLock release and the callback running, this method must see the subscriber
// and refuse to delete.
func TestRemoveBroadcasterIfEmpty_KeepsBroadcasterWithSubscribers(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b, _ := m.CreateOrGetBroadcaster("topic1")
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	m.removeBroadcasterIfEmpty("topic1", b)

	got, ok := m.GetBroadcaster("topic1")
	require.True(t, ok, "broadcaster with subscribers must not be removed")
	require.Same(t, b, got)
}

// removeBroadcasterIfEmpty must NOT delete a fresh broadcaster when called
// with a stale pointer — e.g. an old auto-remove callback firing after the
// topic's broadcaster has been replaced.
func TestRemoveBroadcasterIfEmpty_IgnoresStalePointer(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	stale, _ := m.CreateOrGetBroadcaster("topic1")
	m.RemoveBroadcaster("topic1")
	fresh, _ := m.CreateOrGetBroadcaster("topic1")
	require.NotSame(t, stale, fresh)

	m.removeBroadcasterIfEmpty("topic1", stale)

	got, ok := m.GetBroadcaster("topic1")
	require.True(t, ok, "stale auto-remove must not delete a replacement broadcaster")
	require.Same(t, fresh, got)
}

// removeBroadcasterIfEmpty deletes when both conditions are met (current
// pointer matches AND no subscribers).
func TestRemoveBroadcasterIfEmpty_DeletesWhenEmpty(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b, _ := m.CreateOrGetBroadcaster("topic1")
	m.removeBroadcasterIfEmpty("topic1", b)

	_, ok := m.GetBroadcaster("topic1")
	require.False(t, ok)
}
