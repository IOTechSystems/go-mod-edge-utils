//
// Copyright (C) 2024 IOTech Ltd
//

package re_exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/errors"
	loggerMocks "github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log/mocks"
)

func TestEnqueue(t *testing.T) {
	mockLogger := &loggerMocks.Logger{}
	mockLogger.On("Tracef", mock.AnythingOfType("string"), mock.Anything).Return()
	mockLogger.On("Debugf", mock.AnythingOfType("string"), mock.AnythingOfType("int"), mock.AnythingOfType("Duration")).Return()

	dic := di.NewContainer(di.ServiceConstructorMap{
		container.LoggerInterfaceName: func(get di.Get) any {
			return mockLogger
		},
	})

	ctx := context.Background()
	testFun := func(context.Context, *di.Container, int) bool {
		return true
	}
	items := []int{1, 2}
	tests := []struct {
		Name     string
		MaxCount int
		Err      bool
	}{
		{"Successfully add items to retry queue", len(items), false},
		{"Exceeded queue max count", len(items) - 1, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			queue := NewMemoryQueue[int](dic, ctx, test.MaxCount, "", testFun)
			var err errors.Error
			for _, item := range items {
				err = queue.Enqueue(item)
			}
			if test.Err {
				assert.Error(t, err, "Should return an error")
			}
		})
	}
}
