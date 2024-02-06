//
// Copyright (C) 2024 IOTech Ltd
//

package common

import (
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/errors"
)

// Queue is the interface for the queue
type Queue[T any] interface {
	// Enqueue method that adds a new item to the queue
	Enqueue(item T) errors.Error

	// Dequeue method that removes the first item from the items of the queue
	Dequeue()

	// Peek method that looks at the next item without removing it from the queue
	Peek() T

	// Size returns a number indicating how many items are in the queue
	Size() int
}
