//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// SubscriberCh is a channel type used for broadcasting messages.
type SubscriberCh chan any

type Subscriber struct {
	ch    SubscriberCh
	isNew bool
}

// Broadcaster manages a set of subscribers and broadcasts messages to them.
type Broadcaster struct {
	lc log.Logger
	// subscribers hold the active subscribers.
	subscribers map[SubscriberCh]*Subscriber
	mu          sync.RWMutex
	lastHash    common.AtomicString

	pollingService PollingService
	onEmptyCb      func()
	once           sync.Once
}

// NewBroadcaster creates a new instance of Broadcaster.
func NewBroadcaster(lc log.Logger) *Broadcaster {
	b := &Broadcaster{
		lc:          lc,
		subscribers: make(map[SubscriberCh]*Subscriber),
	}
	b.lastHash.Set("")
	return b
}

// SetPollingService sets the polling service for the broadcaster if auto-polling is required.
func (b *Broadcaster) SetPollingService(service PollingService) {
	b.pollingService = service
}

// SetOnEmptyCallback sets a callback function that will be called when there are no subscribers left.
func (b *Broadcaster) SetOnEmptyCallback(f func()) {
	b.onEmptyCb = f
}

// Subscribe adds a new subscriber and returns a channel to receive messages.
func (b *Broadcaster) Subscribe() SubscriberCh {
	ch := make(SubscriberCh, 64)
	b.mu.Lock()
	b.subscribers[ch] = &Subscriber{
		ch:    ch,
		isNew: true,
	}
	b.mu.Unlock()

	b.lc.Debugf("sse: Subscriber added, total=%d", len(b.subscribers))
	return ch
}

// Unsubscribe should only be deferred after the subscription to ensure the channel will be closed properly.
func (b *Broadcaster) Unsubscribe(ch SubscriberCh) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)

	b.lc.Debugf("sse: Subscriber removed, total=%d", len(b.subscribers))

	if len(b.subscribers) == 0 {
		go b.handleNoSubscribers()
	}
	b.mu.Unlock()
}

func (b *Broadcaster) handleNoSubscribers() {
	// Re-check the subscriber count under the lock: a new subscriber may have
	// arrived between the Unsubscribe that scheduled this goroutine and now.
	// Without this re-check the auto-remove callback fires against a
	// broadcaster that has just been re-subscribed, breaking GetBroadcaster
	// for the new subscriber.
	b.mu.RLock()
	if len(b.subscribers) > 0 {
		b.mu.RUnlock()
		return
	}
	cb := b.onEmptyCb
	pollingService := b.pollingService
	b.mu.RUnlock()

	// When an onEmpty callback is present (e.g. the manager), delegate both
	// polling teardown and removal to the callback. The callback re-checks
	// emptiness and identity under its own lock, so StopPolling only fires
	// after we are certain no new subscriber slipped in between our RUnlock
	// above and the callback running. Without this guard, StopPolling could
	// fire on a broadcaster that just gained a subscriber; because StartPolling
	// uses sync.Once, polling would never restart on that instance.
	if pollingService != nil && cb == nil {
		if err := b.StopPolling(); err != nil {
			b.lc.Errorf("sse: Failed to stop polling: %v", err)
		}
	}
	if cb != nil {
		b.lc.Debug("sse: No subscribers left, calling onEmpty callback")
		cb()
	}
}

// Publish sends data to all subscribers.
func (b *Broadcaster) Publish(data any) {
	shouldSend := b.shouldSendUpdate(data)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch, s := range b.subscribers {
		// Only send data to subscribers that are new or if the data has changed
		if s.isNew || shouldSend {
			select {
			case ch <- data:
				if s.isNew {
					s.isNew = false // Mark the subscriber as no longer new after the first message
				}
			default: // if the channel is full, dropping to avoid blocking
				b.lc.Warn("sse: Subscriber channel is full, dropping data")
			}
		}
	}
}

// StartPolling starts the polling service if it is set.
func (b *Broadcaster) StartPolling() {
	if b.pollingService == nil {
		b.lc.Debug("sse: StartPolling: no polling service defined")
	}
	// Use sync.Once to ensure the polling service is started only once for the same broadcaster instance.
	b.once.Do(func() {
		b.pollingService.Start(b)
	})
}

// StopPolling stops the polling service if it is running. It cancels the polling context and stops the service.
func (b *Broadcaster) StopPolling() error {
	if b.pollingService == nil {
		b.lc.Debug("sse: StopPolling: no polling service defined")
		return nil
	}
	return b.pollingService.Stop()
}

func (b *Broadcaster) shouldSendUpdate(data any) bool {
	bytes, err := json.Marshal(data)
	if err != nil {
		b.lc.Errorf("sse: Failed to marshal data for hash comparison: %v", err)
		return false
	}

	hashBytes := sha256.Sum256(bytes)
	newHashStr := hex.EncodeToString(hashBytes[:])

	return b.lastHash.CompareAndSwap(newHashStr)
}
