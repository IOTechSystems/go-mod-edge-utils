//
// Copyright (C) 2025 IOTech Ltd
//

package sse

import (
	"sync"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

// Manager manages multiple broadcasters for different topics.
type Manager struct {
	// broadcasters hold a map of topic names to their corresponding broadcasters.
	broadcasters map[string]*Broadcaster
	mu           sync.RWMutex
	lc           log.Logger
}

// NewManager creates a new SSE Manager instance.
func NewManager(lc log.Logger) *Manager {
	return &Manager{
		broadcasters: make(map[string]*Broadcaster),
		lc:           lc,
	}
}

// GetBroadcaster retrieves a broadcaster for the specified topic or creates a new one if it doesn't exist.
func (m *Manager) GetBroadcaster(topic string) (b *Broadcaster, isNew bool) {
	m.mu.RLock()
	b, ok := m.broadcasters[topic]
	m.mu.RUnlock()

	if ok {
		isNew = true
		m.lc.Debugf("sse: Broadcaster with topic '%s' found", topic)
		return b, isNew
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.lc.Debugf("sse: Creating new broadcaster for topic '%s'", topic)
	b = NewBroadcaster(m.lc)
	b.SetOnEmptyCallback(func() {
		m.RemoveBroadcaster(topic)
	})
	m.broadcasters[topic] = b
	return b, false
}

// RemoveBroadcaster removes a broadcaster for the specified topic.
func (m *Manager) RemoveBroadcaster(topic string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.broadcasters, topic)
	m.lc.Debugf("sse: Broadcaster of topic '%s' has been removed", topic)
}
