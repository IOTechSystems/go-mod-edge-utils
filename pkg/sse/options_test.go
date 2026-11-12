//
// Copyright (C) 2026 IOTech Ltd
//

package sse

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWithJWTDeadline(t *testing.T) {
	config := &HandlerConfig{}
	WithJWTDeadline()(config)
	assert.True(t, config.JWTDeadline)
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

func TestWithStopCallback(t *testing.T) {
	called := false
	fn := func() { called = true }
	config := &PollingConfig{}
	WithStopCallback(fn)(config)
	assert.NotNil(t, config.StopCallback)
	config.StopCallback()
	assert.True(t, called)
}

func TestWithFirstMessage(t *testing.T) {
	config := &HandlerConfig{}
	WithFirstMessage(func(_ context.Context, _ PollingService) (any, func(any) bool) { return "first", nil })(config)
	require.NotNil(t, config.FirstMessage)
	first, skip := config.FirstMessage(context.Background(), nil)
	assert.Equal(t, "first", first)
	assert.Nil(t, skip)
}
