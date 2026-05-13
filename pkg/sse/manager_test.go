//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(context.Background(), newTestLogger(t), 30*time.Second)
}

// TestManager_PublishUnknownTopic_NoOp verifies that publishing to a topic
// with no subscribers (no broadcaster in the map) is a safe no-op.
func TestManager_PublishUnknownTopic_NoOp(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	// Must not panic and must return promptly.
	done := make(chan struct{})
	go func() {
		m.Publish("never-subscribed", "payload")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Publish to unknown topic blocked unexpectedly")
	}

	// Broadcaster must not have been auto-created.
	m.mu.RLock()
	_, exists := m.broadcasters["never-subscribed"]
	m.mu.RUnlock()
	assert.False(t, exists, "Publish must not auto-create a broadcaster")
}

// TestManager_SubscribeCreatesBroadcaster verifies that the first subscribe
// to a topic creates its broadcaster atomically.
func TestManager_SubscribeCreatesBroadcaster(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b, ch, isNew := m.subscribe("topic-a", nil)
	defer m.unsubscribe("topic-a", ch)

	require.NotNil(t, b)
	require.NotNil(t, ch)
	assert.True(t, isNew, "first subscriber to a topic should report isNew=true")

	m.mu.RLock()
	stored, ok := m.broadcasters["topic-a"]
	m.mu.RUnlock()
	require.True(t, ok, "broadcaster must be registered in the map")
	assert.Same(t, b, stored, "returned broadcaster should be the one stored in the map")
}

// TestManager_PublishReachesSubscriber verifies the basic happy path:
// subscribe, publish, the subscriber receives the payload.
func TestManager_PublishReachesSubscriber(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	_, ch, _ := m.subscribe("topic-a", nil)
	defer m.unsubscribe("topic-a", ch)

	m.Publish("topic-a", "hello")

	select {
	case got := <-ch:
		assert.Equal(t, "hello", got)
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive published payload")
	}
}

// TestManager_UnsubscribeCleansUpBroadcaster verifies that the last
// unsubscribe synchronously removes the broadcaster from the map, so any
// later Publish sees the topic as unknown.
func TestManager_UnsubscribeCleansUpBroadcaster(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	_, ch, _ := m.subscribe("topic-a", nil)

	m.unsubscribe("topic-a", ch)

	m.mu.RLock()
	_, exists := m.broadcasters["topic-a"]
	m.mu.RUnlock()
	assert.False(t, exists, "broadcaster must be removed from the map synchronously")
}

// TestManager_MultiSubscriberSharesBroadcaster verifies that two
// subscriptions to the same topic share one broadcaster and that the
// broadcaster lives until the last subscriber leaves.
func TestManager_MultiSubscriberSharesBroadcaster(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	b1, ch1, isNew1 := m.subscribe("topic-a", nil)
	b2, ch2, isNew2 := m.subscribe("topic-a", nil)

	assert.True(t, isNew1)
	assert.False(t, isNew2, "second subscriber must not be reported as new")
	assert.Same(t, b1, b2, "both subscribers must share the broadcaster")

	m.Publish("topic-a", "broadcast")

	// Both subscribers receive the same payload.
	for i, ch := range []subscriberCh{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, "broadcast", got, "subscriber %d", i)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive payload", i)
		}
	}

	// Unsubscribing one leaves the broadcaster alive for the other.
	m.unsubscribe("topic-a", ch1)
	m.mu.RLock()
	_, exists := m.broadcasters["topic-a"]
	m.mu.RUnlock()
	assert.True(t, exists, "broadcaster must remain while at least one subscriber is connected")

	m.unsubscribe("topic-a", ch2)
	m.mu.RLock()
	_, exists = m.broadcasters["topic-a"]
	m.mu.RUnlock()
	assert.False(t, exists, "broadcaster must be removed once the last subscriber leaves")
}

// TestManager_FreshBroadcasterAfterCleanup is the regression test for the
// race that motivated this redesign. Sequence:
//  1. Subscriber A subscribes, then unsubscribes (cleanup removes broadcaster).
//  2. Subscriber B subscribes — must get a fresh broadcaster, not a stale one.
//  3. Publish reaches B.
func TestManager_FreshBroadcasterAfterCleanup(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	_, chA, _ := m.subscribe("topic-a", nil)
	m.unsubscribe("topic-a", chA)

	_, chB, isNewB := m.subscribe("topic-a", nil)
	defer m.unsubscribe("topic-a", chB)

	assert.True(t, isNewB, "after the previous broadcaster was cleaned up, the next subscribe must create a fresh one")

	m.Publish("topic-a", "after-recreate")

	select {
	case got := <-chB:
		assert.Equal(t, "after-recreate", got)
	case <-time.After(time.Second):
		t.Fatal("subscriber B did not receive payload after broadcaster was recreated")
	}
}

// TestManager_ConcurrentSubscribersSameTopic stresses lookup-or-create under
// concurrent subscribe. All callers must end up on the same broadcaster, and
// the map must hold exactly one entry. Run under -race to catch any data
// race exposed by the redesign.
func TestManager_ConcurrentSubscribersSameTopic(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	const n = 64
	var wg sync.WaitGroup
	bs := make([]*broadcaster, n)
	chs := make([]subscriberCh, n)

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			b, ch, _ := m.subscribe("topic-a", nil)
			bs[i] = b
			chs[i] = ch
		}(i)
	}
	wg.Wait()

	// Every concurrent subscriber must land on the same broadcaster.
	first := bs[0]
	for i := 1; i < n; i++ {
		assert.Same(t, first, bs[i], "subscriber %d landed on a different broadcaster", i)
	}

	m.mu.RLock()
	mapSize := len(m.broadcasters)
	m.mu.RUnlock()
	assert.Equal(t, 1, mapSize, "map should contain exactly one broadcaster for topic-a")

	// Cleanup.
	for i := 0; i < n; i++ {
		m.unsubscribe("topic-a", chs[i])
	}
}

// TestManager_ConcurrentPublishAndSubscribe stresses the lock interaction
// between Publish and subscribe to surface any data race introduced by the
// new Manager-owned lifecycle. Should be run with `go test -race`.
func TestManager_ConcurrentPublishAndSubscribe(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	const (
		subscribers   = 32
		publishesEach = 100
		topic         = "topic-stress"
	)

	// Pre-create a long-lived subscriber so the broadcaster stays alive.
	_, anchorCh, _ := m.subscribe(topic, nil)
	defer m.unsubscribe(topic, anchorCh)

	// Drain the anchor to keep its channel free.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range anchorCh {
		}
	}()

	var wg sync.WaitGroup
	wg.Add(subscribers * 2)
	for i := 0; i < subscribers; i++ {
		go func() {
			defer wg.Done()
			_, ch, _ := m.subscribe(topic, nil)
			// Drain a few events then leave.
			go func() {
				for j := 0; j < 10; j++ {
					select {
					case <-ch:
					case <-time.After(50 * time.Millisecond):
						return
					}
				}
			}()
			time.Sleep(10 * time.Millisecond)
			m.unsubscribe(topic, ch)
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < publishesEach; j++ {
				m.Publish(topic, j)
			}
		}()
	}
	wg.Wait()

	// We mainly rely on -race for the assertion; sanity-check the broadcaster
	// for the anchor topic is still alive.
	m.mu.RLock()
	_, exists := m.broadcasters[topic]
	m.mu.RUnlock()
	assert.True(t, exists, "anchor subscriber should keep the broadcaster alive")
}

// TestManager_PublishDuringFinalUnsubscribe stresses the exact pattern that
// motivated the redesign: a publisher running while the last subscriber leaves
// and the broadcaster is torn down. Under the new architecture every Publish
// is a fresh map lookup, so a publish racing with the final unsubscribe must
// be a safe no-op once the broadcaster is gone.
//
// Run under `go test -race` (ideally `-count=100`) to surface any regression
// — a send on a closed channel, a stale broadcaster reference, or a torn
// subscribers map would all fail here.
func TestManager_PublishDuringFinalUnsubscribe(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	const topic = "topic-race"
	_, ch, _ := m.subscribe(topic, nil)

	// Drain the subscriber channel. The loop exits when unsubscribe closes ch.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range ch {
		}
	}()

	stop := make(chan struct{})
	pubDone := make(chan struct{})
	var publishCount atomic.Int64
	go func() {
		defer close(pubDone)
		for {
			select {
			case <-stop:
				return
			default:
				m.Publish(topic, publishCount.Add(1))
			}
		}
	}()

	// Let the publisher get hot before we yank the rug out.
	time.Sleep(20 * time.Millisecond)

	// Unsubscribe while the publisher is still hammering. The broadcaster is
	// removed from the map; subsequent publishes must be no-op lookups.
	m.unsubscribe(topic, ch)

	// Let the publisher run past the unsubscribe to expose any race.
	time.Sleep(20 * time.Millisecond)

	close(stop)
	<-pubDone
	<-drainDone

	m.mu.RLock()
	_, exists := m.broadcasters[topic]
	m.mu.RUnlock()
	assert.False(t, exists, "broadcaster must be removed after last unsubscribe")
	assert.Greater(t, publishCount.Load(), int64(0), "publisher should have run at least once")
}

// TestManager_Shutdown_CancelsContext verifies that Shutdown cancels the
// manager's context, which is the signal handlers use to drop their streams.
func TestManager_Shutdown_CancelsContext(t *testing.T) {
	m := newTestManager(t)

	m.Shutdown()

	select {
	case <-m.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Manager context was not cancelled by Shutdown")
	}
}

// --- PollingService lifecycle ---

// trackingPollingService records Start/Stop calls so tests can assert the
// service is wired into a broadcaster's lifetime correctly.
type trackingPollingService struct {
	starts atomic.Int32
	stops  atomic.Int32

	pubMu sync.Mutex
	pub   Publisher
}

func (t *trackingPollingService) Start(pub Publisher) {
	t.starts.Add(1)
	t.pubMu.Lock()
	t.pub = pub
	t.pubMu.Unlock()
}

func (t *trackingPollingService) Stop() error {
	t.stops.Add(1)
	return nil
}

func (t *trackingPollingService) publish(data any) {
	t.pubMu.Lock()
	defer t.pubMu.Unlock()
	if t.pub != nil {
		t.pub.Publish(data)
	}
}

// TestManager_PollingService_StartsOnceOnFirstSubscribe verifies that the
// polling service attached via the first subscribe is started exactly once,
// and the broadcaster receives the published payload it pushes.
func TestManager_PollingService_StartsOnceOnFirstSubscribe(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	svc := &trackingPollingService{}

	b, ch, isNew := m.subscribe("topic-poll", svc)
	defer m.unsubscribe("topic-poll", ch)

	require.True(t, isNew)
	require.Equal(t, svc, b.pollingService, "polling service must be wired into the broadcaster on first subscribe")

	// Mirror Handler's behavior: caller drives startPolling when isNew.
	b.startPolling()

	// Second subscribe should NOT start polling again.
	_, ch2, isNew2 := m.subscribe("topic-poll", svc)
	defer m.unsubscribe("topic-poll", ch2)
	assert.False(t, isNew2)

	// Simulate the polling service pushing a payload — both subscribers see it.
	svc.publish("polled")

	for i, c := range []subscriberCh{ch, ch2} {
		select {
		case got := <-c:
			assert.Equal(t, "polled", got)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive polled payload", i)
		}
	}

	assert.Equal(t, int32(1), svc.starts.Load(), "polling should start exactly once per broadcaster lifetime")
}

// TestManager_PollingService_StopsOnLastUnsubscribe verifies that the
// polling service is asked to Stop when the last subscriber leaves and the
// broadcaster is torn down.
func TestManager_PollingService_StopsOnLastUnsubscribe(t *testing.T) {
	m := newTestManager(t)
	defer m.Shutdown()

	svc := &trackingPollingService{}

	b, ch, _ := m.subscribe("topic-poll", svc)
	b.startPolling()

	m.unsubscribe("topic-poll", ch)

	// Stop is dispatched in a goroutine; give it a moment.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if svc.stops.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	assert.GreaterOrEqual(t, svc.stops.Load(), int32(1), "polling service should be stopped after the last unsubscribe")
}
