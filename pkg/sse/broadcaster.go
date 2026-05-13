//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// subscriberCh carries broadcast payloads to a single subscriber. It also
// serves as the identity key in broadcaster.subscribers, so the subscriber
// struct does not need to carry its own channel reference.
type subscriberCh chan any

type subscriber struct {
	// isNew flags a subscriber that has not yet received any payload. New
	// subscribers always receive the next published payload regardless of
	// the dedup hash, otherwise a subscriber that joins after the cache is
	// warm could wait an arbitrary time before observing the current value.
	// atomic so concurrent Publish goroutines (which hold mu.RLock, allowing
	// parallel readers) can safely clear the flag without escalating to a
	// write lock.
	isNew atomic.Bool
}

// broadcaster is the per-topic fan-out unit. It is package-internal: only the
// Manager creates, holds, and tears down broadcasters. External code reaches
// this type only through Manager.Publish and sse.Handler, which prevents
// reference escape (the source of every race condition in earlier revisions).
type broadcaster struct {
	lc          log.Logger
	subscribers map[subscriberCh]*subscriber
	mu          sync.RWMutex
	lastHash    common.AtomicString

	pollingService PollingService
}

// newBroadcaster constructs a broadcaster. pollingService is bound for the
// broadcaster's lifetime; pass nil when no polling source is configured.
func newBroadcaster(lc log.Logger, pollingService PollingService) *broadcaster {
	b := &broadcaster{
		lc:             lc,
		subscribers:    make(map[subscriberCh]*subscriber),
		pollingService: pollingService,
	}
	b.lastHash.Set("")
	return b
}

// subscribe adds a new subscriber channel and returns it. Callers must
// arrange for unsubscribe so the channel is closed.
func (b *broadcaster) subscribe() subscriberCh {
	ch := make(subscriberCh, 64)
	s := &subscriber{}
	s.isNew.Store(true)
	b.mu.Lock()
	b.subscribers[ch] = s
	total := len(b.subscribers)
	b.mu.Unlock()

	b.lc.Debugf("sse: subscriber added, total=%d", total)
	return ch
}

// unsubscribe removes ch and closes it. Returns true when the broadcaster has
// no remaining subscribers, so the caller (Manager) can decide whether to
// tear it down.
func (b *broadcaster) unsubscribe(ch subscriberCh) (empty bool) {
	b.mu.Lock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
	total := len(b.subscribers)
	empty = total == 0
	b.mu.Unlock()

	b.lc.Debugf("sse: subscriber removed, total=%d", total)
	return empty
}

// Publish sends data to every current subscriber. Method is exported so that
// *broadcaster satisfies the Publisher interface used by PollingService —
// the type itself is unexported, so no external caller can obtain or invoke
// this directly.
func (b *broadcaster) Publish(data any) {
	shouldSend := b.shouldSendUpdate(data)

	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch, s := range b.subscribers {
		isNew := s.isNew.Load()
		// Only forward to subscribers that are new or when the payload changed.
		if isNew || shouldSend {
			select {
			case ch <- data:
				if isNew {
					// CAS rather than plain Store so concurrent Publishes
					// don't clobber the "first-event observed" transition.
					s.isNew.CompareAndSwap(true, false)
				}
			default:
				// Subscriber buffer is full. Skipping this subscriber keeps the
				// fan-out from blocking on a slow consumer.
				b.lc.Warn("sse: subscriber channel is full, skipping this subscriber for this event")
			}
		}
	}
}

// startPolling kicks off the configured polling service. Manager calls this
// exactly once per broadcaster lifetime — on the first subscribe.
func (b *broadcaster) startPolling() {
	if b.pollingService == nil {
		return
	}
	b.pollingService.Start(b)
}

// stopPolling stops the polling service. Safe to call when polling was never
// started: PollingService implementations treat repeated/early Stop as a
// no-op.
func (b *broadcaster) stopPolling() error {
	if b.pollingService == nil {
		return nil
	}
	return b.pollingService.Stop()
}

func (b *broadcaster) shouldSendUpdate(data any) bool {
	bytes, err := json.Marshal(data)
	if err != nil {
		b.lc.Errorf("sse: failed to marshal data for hash comparison: %v", err)
		return false
	}

	hashBytes := sha256.Sum256(bytes)
	newHashStr := hex.EncodeToString(hashBytes[:])

	return b.lastHash.CompareAndSwap(newHashStr)
}
