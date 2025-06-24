//
// Copyright (C) 2025 IOTech Ltd
//

package sse

import (
	"github.com/stretchr/testify/require"
	"testing"

	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log/mocks"
)

func TestShouldSendUpdate(t *testing.T) {
	mockLogger := &loggerMocks.Logger{}

	tests := []struct {
		name     string
		value1   any
		value2   any
		expected bool // test if the second comparison should return true (different) or false (equal)
	}{
		// Same values
		{"string equal", "hello", "hello", false},
		{"int equal", 1, 1, false},
		{"float equal", 3.14, 3.14, false},
		{"slice equal", []string{"a", "b"}, []string{"a", "b"}, false},
		{"map equal", map[string]any{"x": 1, "y": "a"}, map[string]any{"x": 1, "y": "a"}, false},
		{"nil equal", nil, nil, false},

		// Different values
		{"string different", "hello", "hello world", true},
		{"int different", 1, 2, true},
		{"float different", 3.14, 3.1415, true},
		{"slice different", []string{"a", "b"}, []string{"b", "a"}, true},
		{"map different value types", map[string]any{"x": 1, "y": "a"}, map[string]any{"x": "a", "y": 1}, true},
		{"nil vs non-nil", nil, "nil", true},

		// Edge cases
		{"map key order different", map[string]any{"x": 1, "y": "a"}, map[string]any{"y": "a", "x": 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBroadcaster(mockLogger)
			require.True(t, b.shouldSendUpdate(tt.value1), "first update should always return true")

			if tt.expected {
				require.True(t, b.shouldSendUpdate(tt.value2), "second comparison should return true (different)")
			} else {
				require.False(t, b.shouldSendUpdate(tt.value2), "second comparison should return false (equal)")
			}
		})
	}
}
