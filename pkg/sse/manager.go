//
// Copyright (C) 2025-2026 IOTech Ltd
//

package sse

import (
	"context"
	"sync"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// Manager manages multiple broadcasters for different topics.
type Manager struct {
	// broadcasters hold a map of topic names to their corresponding broadcasters.
	broadcasters      map[string]*Broadcaster
	mu                sync.RWMutex
	lc                log.Logger
	heartbeatInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager creates a new SSE Manager instance.
func NewManager(ctx context.Context, lc log.Logger, heartbeatInterval time.Duration) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	manager := &Manager{
		broadcasters:      make(map[string]*Broadcaster),
		lc:                lc,
		ctx:               ctx,
		cancel:            cancel,
		heartbeatInterval: heartbeatInterval,
	}

	// Gracefully shutdown the SSE manager when the main context is done
	go func() {
		<-ctx.Done()
		manager.Shutdown()
	}()

	return manager
}

// GetBroadcaster retrieves a broadcaster for the specified topic.
func (m *Manager) GetBroadcaster(topic string) (b *Broadcaster, ok bool) {
	m.mu.RLock()
	b, ok = m.broadcasters[topic]
	m.mu.RUnlock()

	if ok {
		m.lc.Debugf("sse: Broadcaster with topic '%s' found", topic)
		return b, ok
	}

	return nil, false
}

// CreateOrGetBroadcaster retrieves a broadcaster for the specified topic or
// creates a new one if it doesn't exist. The returned broadcaster auto-removes
// itself from the manager when its last subscriber unsubscribes — fitting the
// "lives only while observed" model used by live dashboards and polling-driven
// streams. For publisher-driven flows (e.g. async job progress) where the
// stream lifetime is owned by the publisher rather than its subscribers, use
// pkg/sse/jobtracker instead.
func (m *Manager) CreateOrGetBroadcaster(topic string) (b *Broadcaster, isNew bool) {
	if b, ok := m.GetBroadcaster(topic); ok {
		return b, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check after acquiring the write lock. Two callers can both pass the
	// fast-path GetBroadcaster check above and serialize on Lock here; without
	// this re-check the second caller would silently overwrite the first
	// caller's broadcaster and orphan any subscribers the first caller already
	// attached.
	if b, ok := m.broadcasters[topic]; ok {
		return b, false
	}

	m.lc.Debugf("sse: Creating new broadcaster for topic '%s'", topic)
	b = NewBroadcaster(m.lc)
	// Capture the broadcaster pointer in the callback so the auto-remove path
	// can compare-and-delete: a new subscriber that arrives between
	// handleNoSubscribers's RLock check and this callback firing must not have
	// its broadcaster yanked out from under it. removeBroadcasterIfEmpty
	// re-verifies both identity and emptiness before deleting.
	b.SetOnEmptyCallback(func() {
		m.removeBroadcasterIfEmpty(topic, b)
	})
	m.broadcasters[topic] = b
	return b, true
}

// RemoveBroadcaster removes a broadcaster for the specified topic
// unconditionally. Most callers should not need this — the auto-remove
// callback installed by CreateOrGetBroadcaster handles cleanup safely.
func (m *Manager) RemoveBroadcaster(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.broadcasters, topic)
	m.lc.Debugf("sse: Broadcaster of topic '%s' has been removed", topic)
}

// removeBroadcasterIfEmpty deletes the broadcaster for topic only if (a) the
// entry currently in the map is still b (not a replacement), and (b) b has no
// subscribers. This closes the race window between the broadcaster-level
// "subscribers == 0" re-check in handleNoSubscribers and this callback
// running: a new subscriber can attach after the broadcaster releases its
// RLock but before cb() fires; without this manager-level guard, that
// subscriber's broadcaster would be silently yanked from the map.
//
// Lock order is m.mu → b.mu, consistent with every other path (no caller
// holds b.mu while taking m.mu).
func (m *Manager) removeBroadcasterIfEmpty(topic string, b *Broadcaster) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cur, ok := m.broadcasters[topic]
	if !ok || cur != b {
		return
	}

	b.mu.RLock()
	empty := len(b.subscribers) == 0
	b.mu.RUnlock()
	if !empty {
		return
	}

	delete(m.broadcasters, topic)
	m.lc.Debugf("sse: Broadcaster of topic '%s' has been removed", topic)
	// Stop polling here, after confirmed removal, so we never call StopPolling
	// on a broadcaster that still has subscribers. handleNoSubscribers defers
	// to this path (skips StopPolling when an onEmpty callback is set) to close
	// the race where a new subscriber arrives between the emptiness check and
	// StopPolling firing — which would permanently break polling since
	// StartPolling is guarded by sync.Once.
	if err := b.StopPolling(); err != nil {
		m.lc.Errorf("sse: Failed to stop polling for topic '%s': %v", topic, err)
	}
}

func (m *Manager) Shutdown() {
	m.cancel()
}
