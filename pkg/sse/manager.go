//
// Copyright (C) 2025 IOTech Ltd
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

// CreateOrGetBroadcaster retrieves a broadcaster for the specified topic or creates a new one if it doesn't exist.
func (m *Manager) CreateOrGetBroadcaster(topic string) (b *Broadcaster, isNew bool) {
	if b, ok := m.GetBroadcaster(topic); ok {
		return b, false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.lc.Debugf("sse: Creating new broadcaster for topic '%s'", topic)
	b = NewBroadcaster(m.lc)
	b.SetOnEmptyCallback(func() {
		m.RemoveBroadcaster(topic)
	})
	m.broadcasters[topic] = b
	return b, true
}

// RemoveBroadcaster removes a broadcaster for the specified topic.
func (m *Manager) RemoveBroadcaster(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.broadcasters, topic)
	m.lc.Debugf("sse: Broadcaster of topic '%s' has been removed", topic)
}

func (m *Manager) Shutdown() {
	m.cancel()
}
