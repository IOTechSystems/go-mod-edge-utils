//
// Copyright (C) 2025 IOTech Ltd
//

package sse

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"

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
	lastHash    atomic.Value

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
	b.lastHash.Store("")
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
	return ch
}

// Unsubscribe should only be deferred after the subscription to ensure the channel will be closed properly.
func (b *Broadcaster) Unsubscribe(ch SubscriberCh) {
	b.mu.Lock()
	delete(b.subscribers, ch)
	close(ch)

	if len(b.subscribers) == 0 {
		go b.handleNoSubscribers()
	}
	b.mu.Unlock()
}

func (b *Broadcaster) handleNoSubscribers() {
	// Stop the polling service if it is set and there are no subscribers left
	if b.pollingService != nil {
		if err := b.StopPolling(); err != nil {
			b.lc.Errorf("sse: Failed to stop polling: %v", err)
		}
	}
	if b.onEmptyCb != nil {
		b.lc.Debug("sse: No subscribers left, calling onEmpty callback")
		b.onEmptyCb()
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

	// Use for loop with CompareAndSwap to ensure atomicity
	for {
		old := b.lastHash.Load()
		oldStr, _ := old.(string)
		if oldStr == newHashStr {
			return false
		}

		if b.lastHash.CompareAndSwap(oldStr, newHashStr) {
			return true
		}
	}
}
