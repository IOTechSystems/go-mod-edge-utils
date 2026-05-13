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

// Manager owns one fan-out broadcaster per topic. It is the only public
// surface that external code interacts with for SSE pubsub — subscribers
// connect through sse.Handler and publishers push via Manager.Publish.
//
// m.mu guards the broadcasters map: lookup, install, and removal are
// serialized so a Publish can never see a half-torn-down topic. Fan-out to
// individual subscribers is guarded by the broadcaster's own b.mu, which
// Publish acquires after releasing m.mu.
type Manager struct {
	broadcasters      map[string]*broadcaster
	mu                sync.RWMutex
	lc                log.Logger
	heartbeatInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager creates an SSE Manager that lives for the supplied parent
// context. The heartbeatInterval is applied to every handler the Manager
// serves; pass zero or a negative value to fall back to the package default.
func NewManager(ctx context.Context, lc log.Logger, heartbeatInterval time.Duration) *Manager {
	ctx, cancel := context.WithCancel(ctx)

	manager := &Manager{
		broadcasters:      make(map[string]*broadcaster),
		lc:                lc,
		ctx:               ctx,
		cancel:            cancel,
		heartbeatInterval: heartbeatInterval,
	}

	go func() {
		<-ctx.Done()
		manager.Shutdown()
	}()

	return manager
}

// Publish forwards data to every subscriber currently registered on topic.
// When no broadcaster exists for the topic (no subscriber has connected, or
// the last one left and cleanup has run), the call returns without doing
// anything — matching the standard fire-and-forget semantics of in-memory
// pubsub (Redis pubsub, NATS, in-memory event buses). Callers ship the
// event on a best-effort basis; SSE does not guarantee delivery.
//
// Safe to call at high frequency: the no-subscriber path is a single map
// lookup under a read lock.
func (m *Manager) Publish(topic string, data any) {
	m.mu.RLock()
	b, ok := m.broadcasters[topic]
	m.mu.RUnlock()
	if !ok {
		return
	}
	b.Publish(data)
}

// subscribe atomically (under m.mu) looks up or creates the broadcaster for
// topic and registers a fresh subscriber on it. The atomicity is essential:
// if lookup and subscribe were separate steps, an in-flight cleanup of a
// previous broadcaster could remove it between them, leaving the caller
// subscribed to a broadcaster nobody else can publish to.
//
// pollingService is applied only when the broadcaster is newly created.
// Existing broadcasters keep whatever polling service they were created
// with. Returns true in isNew when the caller should drive any first-time
// setup (e.g. start polling).
func (m *Manager) subscribe(topic string, pollingService PollingService) (b *broadcaster, ch subscriberCh, isNew bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.broadcasters[topic]
	if !ok {
		m.lc.Debugf("sse: creating broadcaster for topic %q", topic)
		b = newBroadcaster(m.lc, pollingService)
		m.broadcasters[topic] = b
		isNew = true
	}
	ch = b.subscribe()
	return b, ch, isNew
}

// unsubscribe removes ch from the broadcaster for topic. When that leaves
// the broadcaster empty, the entry is removed from the map synchronously
// (under m.mu), so any concurrent Publish or subscribe immediately sees the
// fresh state; the slow part of cleanup (stopPolling) runs asynchronously
// to keep Unsubscribe — invoked from the HTTP handler's defer — non-blocking.
func (m *Manager) unsubscribe(topic string, ch subscriberCh) {
	m.mu.Lock()
	b, ok := m.broadcasters[topic]
	if !ok {
		m.mu.Unlock()
		return
	}
	empty := b.unsubscribe(ch)
	if empty {
		delete(m.broadcasters, topic)
		m.lc.Debugf("sse: removed broadcaster for topic %q", topic)
	}
	m.mu.Unlock()

	if empty {
		go func() {
			if err := b.stopPolling(); err != nil {
				m.lc.Errorf("sse: failed to stop polling for topic %q: %v", topic, err)
			}
		}()
	}
}

// Shutdown cancels the Manager's context. Any handler currently servicing an
// SSE stream observes the cancellation and returns.
func (m *Manager) Shutdown() {
	m.cancel()
}
