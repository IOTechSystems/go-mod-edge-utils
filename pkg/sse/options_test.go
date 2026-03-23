//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithPollingService(t *testing.T) {
	mockService := &mockPollingService{}
	config := &HandlerConfig{}
	WithPollingService(mockService)(config)
	assert.Equal(t, mockService, config.PollingService)
}

func TestWithCustomTopic(t *testing.T) {
	config := &HandlerConfig{}
	WithCustomTopic("my-topic")(config)
	assert.Equal(t, "my-topic", config.CustomTopic)
}

func TestWithCustomPollingInterval(t *testing.T) {
	config := &PollingConfig{}
	WithCustomPollingInterval(10 * time.Second)(config)
	assert.Equal(t, 10*time.Second, config.interval)
}

func TestWithCustomApiVersion(t *testing.T) {
	config := &PollingConfig{}
	WithCustomApiVersion("v2")(config)
	assert.Equal(t, "v2", config.ApiVersion)
}

func TestWithStopCondition(t *testing.T) {
	fn := func(data any) bool { return data == "done" }
	config := &PollingConfig{}
	WithStopCondition(fn)(config)
	assert.NotNil(t, config.StopCondition)
	assert.True(t, config.StopCondition("done"))
	assert.False(t, config.StopCondition("not done"))
}
