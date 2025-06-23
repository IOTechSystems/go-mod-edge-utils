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

// Polling is a struct that implements a polling mechanism for fetching data from a data source at regular intervals.
// It is designed to be started once and can be stopped gracefully.
type Polling struct {
	interval    time.Duration
	pollingFunc func(context.Context) (any, error)
	lc          log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPolling creates a new Polling instance with the specified interval and data source.
func NewPolling(interval time.Duration, pollingFunc func(context.Context) (any, error), lc log.Logger) *Polling {
	return &Polling{
		interval:    interval,
		pollingFunc: pollingFunc,
		lc:          lc,
	}
}

// Start initializes the polling mechanism. It sets up an internal context with cancel and starts the polling goroutine.
func (p *Polling) Start(publisher Publisher) {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.pollingAndPublish(publisher)
	}()
	p.lc.Debugf("sse polling: Polling started with interval %v", p.interval)
}

// Stop gracefully stops the polling mechanism. It cancels the context and waits for the polling goroutine to finish.
func (p *Polling) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()
	p.lc.Debug("sse polling: Polling stopped")
	return nil
}

func (p *Polling) pollingAndPublish(publisher Publisher) {
	doPollAndPublish := func() {
		data, err := p.pollingFunc(p.ctx)
		if err != nil {
			p.lc.Errorf("sse polling: Failed to fetch data: %v", err)
			return
		}
		publisher.Publish(data)
	}

	// Initial poll to fetch data immediately before starting the ticker
	doPollAndPublish()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			doPollAndPublish()
		case <-p.ctx.Done():
			p.lc.Debug("sse polling: Polling context cancelled")
			return
		}
	}
}
