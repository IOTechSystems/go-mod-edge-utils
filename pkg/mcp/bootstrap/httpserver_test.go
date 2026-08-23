//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/config"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/log"
)

type configStub struct {
	service *config.ServiceInfo
}

func (c configStub) GetBootstrap() config.BootstrapConfiguration {
	return config.BootstrapConfiguration{Service: c.service}
}
func (c configStub) GetLogLevel() string                        { return "INFO" }
func (c configStub) GetInsecureSecrets() config.InsecureSecrets { return nil }

func newDic(t *testing.T, service *config.ServiceInfo, cancel context.CancelFunc) *di.Container {
	t.Helper()
	return di.NewContainer(di.ServiceConstructorMap{
		container.LoggerInterfaceName: func(_ di.Get) any {
			return log.InitLogger("mcp-bootstrap-test", "INFO", nil)
		},
		container.ConfigurationInterfaceName: func(_ di.Get) any {
			return configStub{service: service}
		},
		container.CancelFuncName: func(_ di.Get) any { return cancel },
	})
}

// freePort reserves an ephemeral port and releases it for the server to bind.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

func TestBootstrapHandlerNotListening(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	server := NewHttpServer(echo.New(), false)

	ok := server.BootstrapHandler(ctx, wg, startup.Timer{}, newDic(t, nil, cancel))
	require.True(t, ok)

	require.Eventually(t, server.IsRunning, time.Second, 10*time.Millisecond)
	cancel()
	wg.Wait()
	assert.False(t, server.IsRunning())
}

func TestBootstrapHandlerBadRequestTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service := &config.ServiceInfo{Host: "127.0.0.1", Port: freePort(t), RequestTimeout: "not-a-duration"}

	ok := NewHttpServer(echo.New(), true).BootstrapHandler(ctx, &sync.WaitGroup{}, startup.Timer{}, newDic(t, service, cancel))
	assert.False(t, ok)
}

func TestBootstrapHandlerMissingService(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ok := NewHttpServer(echo.New(), true).BootstrapHandler(ctx, &sync.WaitGroup{}, startup.Timer{}, newDic(t, nil, cancel))
	assert.False(t, ok)
}

// TestStreamingAndBoundedShutdown locks in the two properties this handler
// exists for: a streamed response leaves the server while the handler is
// still running (no http.TimeoutHandler buffering), and a live stream does
// not block shutdown beyond the grace period.
func TestStreamingAndBoundedShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	port := freePort(t)

	router := echo.New()
	router.HideBanner = true
	handlerDone := make(chan struct{})
	router.GET("/stream", func(c echo.Context) error {
		defer close(handlerDone)
		c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
		c.Response().WriteHeader(http.StatusOK)
		if _, err := c.Response().Write([]byte("data: hello\n\n")); err != nil {
			return err
		}
		c.Response().Flush()
		// Emulate an MCP Streamable HTTP session: never return on its own.
		<-c.Request().Context().Done()
		return nil
	})

	service := &config.ServiceInfo{Host: "127.0.0.1", Port: port, RequestTimeout: "3s"}
	server := NewHttpServer(router, true)
	require.True(t, server.BootstrapHandler(ctx, wg, startup.Timer{}, newDic(t, service, cancel)))
	require.Eventually(t, server.IsRunning, 2*time.Second, 10*time.Millisecond)

	// The flushed chunk must arrive while the handler is still blocked.
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/stream", port))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "data: hello\n", line)
	select {
	case <-handlerDone:
		t.Fatal("handler returned before shutdown; stream should still be live")
	default:
	}

	// Cancelling the bootstrap context must complete shutdown within the
	// grace period even though the stream never ends.
	start := time.Now()
	cancel()
	waited := make(chan struct{})
	go func() { wg.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(shutdownGracePeriod + 3*time.Second):
		t.Fatal("shutdown blocked by live streaming connection")
	}
	assert.False(t, server.IsRunning())
	t.Logf("shutdown completed in %s", time.Since(start))
}

func TestServerAddress(t *testing.T) {
	assert.Equal(t, "0.0.0.0:1234", serverAddress("0.0.0.0", "host", 1234))
	assert.Equal(t, "host:"+strconv.Itoa(1234), serverAddress("", "host", 1234))
}
