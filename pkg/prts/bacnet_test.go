//
// Copyright (C) 2026 IOTech Ltd
//

package prts

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBACnetObjectTypeName(t *testing.T) {
	tests := []struct {
		name           string
		objectType     int
		expectedResult string
		expectError    bool
	}{
		// Known defined types
		{"First defined type (Analog Input)", 0, "Analog Input", false},
		{"Mid defined type (Device)", 8, "Device", false},
		{"Last defined type (Staging)", 60, "Staging", false},

		// Reserved (ASHRAE, 0-127 not in map)
		{"Reserved type below proprietary min", 61, fmt.Sprintf("Reserved Type (%d)", 61), false},
		{"Reserved type upper boundary", 127, fmt.Sprintf("Reserved Type (%d)", 127), false},

		// Proprietary range (128-1023)
		{"Proprietary type at lower boundary", 128, fmt.Sprintf("Proprietary Type (%d)", 128), false},
		{"Proprietary type mid range", 500, fmt.Sprintf("Proprietary Type (%d)", 500), false},
		{"Proprietary type at upper boundary", 1023, fmt.Sprintf("Proprietary Type (%d)", 1023), false},

		// None sentinel
		{"ObjectTypeNone sentinel (0xFFFF)", ObjectTypeNone, ObjectTypeNoneString, false},

		// Error cases
		{"Negative object type", -1, "", true},
		{"At MaxObjectType boundary", MaxObjectType, "", true},
		{"Exceeds MaxObjectType", 2000, "", true},
		{"Between MaxObjectType and ObjectTypeNone", 1025, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GetBACnetObjectTypeName(tt.objectType)
			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
