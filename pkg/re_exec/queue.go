//
// Copyright (C) 2024 IOTech Ltd
//

package re_exec

import (
	"context"
	"sync"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/common"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
)

const (
	DefaultMaxQueueLimit = 10000
	DefaultRetryInterval = "10m"
)

type ReExecFunc[T any] func(context.Context, *di.Container, T) bool

type memoryQueue[T any] struct {
	dic           *di.Container
	ctx           context.Context
	queueLimit    int
	retryInterval time.Duration
	items         []T
	lock          sync.Mutex
}

// NewMemoryQueue is a factory method that returns an initialized ReExecQueue.
func NewMemoryQueue[T any](dic *di.Container, ctx context.Context, queueLimit int, retryInterval string, fun ReExecFunc[T]) common.Queue[T] {
	logger := container.LoggerFrom(dic.Get)

	if queueLimit == 0 {
		queueLimit = DefaultMaxQueueLimit
	}

	if retryInterval == "" {
		retryInterval = DefaultRetryInterval
	}

	interval, err := time.ParseDuration(retryInterval)
	if err != nil {
		logger.Warnf("Failed to parse RetryInterval '%s', set to default '%s', err: %v", retryInterval, DefaultRetryInterval, err)
		interval, _ = time.ParseDuration(DefaultRetryInterval)
	}

	q := &memoryQueue[T]{
		dic:           dic,
		ctx:           ctx,
		queueLimit:    queueLimit,
		retryInterval: interval,
		lock:          sync.Mutex{},
	}

	logger.Debugf("Start MemoryQueue with QueueLimit '%d' and RetryInterval '%s'", queueLimit, interval)
	go q.reExecLoop(fun)

	return q
}

// Enqueue method that adds a new item to the queue
func (q *memoryQueue[T]) Enqueue(item T) errors.Error {
	logger := container.LoggerFrom(q.dic.Get)
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) >= q.queueLimit {
		logger.Tracef("Exceeded queue limit, drop the item: %v", item)
		return errors.NewBaseError(errors.KindLimitExceeded, "Exceeded queue limit, drop the item", nil)
	}
	q.items = append(q.items, item)

	return nil
}

// Dequeue method that removes the first item from the items of the queue
func (q *memoryQueue[T]) Dequeue() {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) > 0 {
		q.items = q.items[1:]
	}
}

// Peek method that looks at the next item without removing it from the queue
func (q *memoryQueue[T]) Peek() T {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.items) > 0 {
		return q.items[0]
	}

	return *new(T)
}

// Size returns a number indicating how many items are in the queue
func (q *memoryQueue[T]) Size() int {
	q.lock.Lock()
	defer q.lock.Unlock()

	return len(q.items)
}

// reExecLoop method that triggers the retry function at intervals
func (q *memoryQueue[T]) reExecLoop(fun ReExecFunc[T]) {
	logger := container.LoggerFrom(q.dic.Get)

	for {
		select {
		case <-q.ctx.Done():
			logger.Info("Exiting retry loop")
			return
		case <-time.After(q.retryInterval):
			for q.Size() != 0 {
				item := q.Peek()
				ok := fun(q.ctx, q.dic, item)
				if !ok {
					logger.Tracef("Retry failed, '%d' items in the queue", q.Size())
					break
				}
				q.Dequeue()
				logger.Tracef("Retry successful, '%d' items left", q.Size())
			}
		}
	}
}
