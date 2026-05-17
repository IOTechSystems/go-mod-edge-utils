//
// Copyright (C) 2026 IOTech Ltd
//

package progress

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestJob(_ *testing.T) *Job {
	return newJob()
}

func TestJob_Publish_UpdatesLastCache(t *testing.T) {
	j := newTestJob(t)

	j.Publish("e1")
	j.Publish("e2")
	j.Publish("e3")

	j.mu.Lock()
	defer j.mu.Unlock()
	assert.True(t, j.hasLast)
	assert.Equal(t, "e3", j.last)
	assert.False(t, j.done)
}

func TestJob_Publish_BroadcastsToSubscribers(t *testing.T) {
	j := newTestJob(t)

	ch1 := make(chan any, subscriberChanBuffer)
	ch2 := make(chan any, subscriberChanBuffer)
	j.mu.Lock()
	j.subscribers[ch1] = struct{}{}
	j.subscribers[ch2] = struct{}{}
	j.mu.Unlock()

	j.Publish("event-a")

	for i, ch := range []chan any{ch1, ch2} {
		select {
		case got := <-ch:
			assert.Equal(t, "event-a", got, "subscriber %d", i)
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive event", i)
		}
	}
}

func TestJob_Publish_LatestWinsOnSlowSubscriber(t *testing.T) {
	// Load-bearing: when the subscriber has not drained its previous
	// payload, the next Publish replaces it (not queues, not drops the
	// new one). Channel buffer of 1 + drain-then-push guarantees the
	// most recent payload is always the one waiting.
	j := newTestJob(t)
	ch := make(chan any, subscriberChanBuffer)
	j.mu.Lock()
	j.subscribers[ch] = struct{}{}
	j.mu.Unlock()

	j.Publish("stale")
	j.Publish("fresh")

	select {
	case got := <-ch:
		assert.Equal(t, "fresh", got, "expected latest-wins to replace stale payload")
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive any payload")
	}

	// Channel must now be empty — only one (latest) value remained.
	select {
	case got := <-ch:
		t.Fatalf("expected empty channel, got %v", got)
	default:
	}
}

func TestJob_Publish_NilPayloadIsValid(t *testing.T) {
	j := newTestJob(t)

	j.Publish(nil)

	j.mu.Lock()
	defer j.mu.Unlock()
	assert.True(t, j.hasLast)
	assert.Nil(t, j.last)
}

func TestJob_Publish_ConcurrentPublishers_NoRace(t *testing.T) {
	// Race-detector smoke test. Correctness of latest-wins coalescing is
	// covered by TestJob_Publish_LatestWinsOnSlowSubscriber; here we only
	// assert no race and that hasLast ends true.
	j := newTestJob(t)
	ch := make(chan any, subscriberChanBuffer)
	j.mu.Lock()
	j.subscribers[ch] = struct{}{}
	j.mu.Unlock()

	// Drain in the background so Publish never blocks structurally
	// (it shouldn't anyway — drain-then-push is non-blocking — but a
	// reader keeps the channel from staying saturated).
	doneDrain := make(chan struct{})
	go func() {
		defer close(doneDrain)
		for {
			select {
			case <-ch:
			case <-time.After(50 * time.Millisecond):
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for k := 0; k < 32; k++ {
				j.Publish(i*32 + k)
			}
		}(i)
	}
	wg.Wait()
	<-doneDrain

	j.mu.Lock()
	assert.True(t, j.hasLast)
	// Some publish must have landed; nil would mean Publish silently
	// dropped every payload (e.g. the drain-then-push pair regressed).
	assert.NotNil(t, j.last)
	j.mu.Unlock()
}

// ---------- Finish ----------

func TestJob_Finish_CachesTerminalAndClosesSubscribers(t *testing.T) {
	j := newTestJob(t)
	ch := make(chan any, subscriberChanBuffer)
	j.mu.Lock()
	j.subscribers[ch] = struct{}{}
	j.mu.Unlock()

	j.Publish("mid")
	j.Finish("ok")

	// Drain any buffered mid event, then expect a closed channel — the
	// terminal arrives via j.last, not the channel.
	for {
		select {
		case got, open := <-ch:
			if !open {
				goto closed
			}
			assert.Equal(t, "mid", got)
		case <-time.After(time.Second):
			t.Fatal("subscriber channel was not closed after Finish")
		}
	}
closed:

	j.mu.Lock()
	defer j.mu.Unlock()
	assert.True(t, j.done)
	assert.True(t, j.hasLast)
	assert.Equal(t, "ok", j.last)
	assert.Empty(t, j.subscribers)

	select {
	case <-j.finished:
	default:
		t.Fatal("finished channel was not closed after Finish")
	}
}

func TestJob_Finish_Idempotent(t *testing.T) {
	j := newTestJob(t)
	j.Finish("first")
	j.Finish("second") // ignored

	j.mu.Lock()
	defer j.mu.Unlock()
	assert.True(t, j.done)
	assert.True(t, j.hasLast)
	assert.Equal(t, "first", j.last)
}

func TestJob_Publish_AfterFinishIsNoOp(t *testing.T) {
	j := newTestJob(t)
	j.Finish("terminal")
	j.Publish("late") // ignored

	j.mu.Lock()
	defer j.mu.Unlock()
	assert.True(t, j.hasLast)
	assert.Equal(t, "terminal", j.last)
}

func TestJob_TerminalSurvivesWithoutChannelRead(t *testing.T) {
	// Regression: the terminal is stored in j.last, not sent via the
	// fan-out channel, so it survives even when no subscriber drains the
	// channel buffer first.
	j := newTestJob(t)
	ch := make(chan any, subscriberChanBuffer)
	j.mu.Lock()
	j.subscribers[ch] = struct{}{}
	j.mu.Unlock()

	j.Publish("progress")
	j.Finish("terminal")

	// Without draining the buffered progress event, observe that:
	//  - the channel eventually closes (Finish's signal)
	//  - j.last carries the terminal
	for {
		select {
		case _, open := <-ch:
			if !open {
				goto done
			}
		case <-time.After(time.Second):
			t.Fatal("channel was not closed after Finish")
		}
	}
done:

	j.mu.Lock()
	defer j.mu.Unlock()
	assert.True(t, j.done)
	assert.Equal(t, "terminal", j.last)
}
