//
// Copyright (C) 2025 IOTech Ltd
//

package common

import "sync"

type AtomicString struct {
	mutex sync.Mutex
	value string
}

// Value returns the current value of the AtomicString.
func (b *AtomicString) Value() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	v := b.value
	return v
}

// Set updates the value of the AtomicString to the provided string.
func (b *AtomicString) Set(v string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.value = v
}

// CompareAndSwap checks if the current value is different from the provided string.
// If they are different, it updates the value to the new string and returns true.
func (b *AtomicString) CompareAndSwap(new string) bool {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.value == new {
		return false
	}
	b.value = new
	return true
}
