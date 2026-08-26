//
// Copyright (C) 2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/handlers"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/v2/pkg/di"

	"github.com/labstack/echo/v4"
)

// shutdownGracePeriod bounds how long Shutdown waits for in-flight requests
// to drain before forcibly closing connections. MCP Streamable HTTP sessions
// are long-lived and will never end on their own, so the wait must be capped.
const shutdownGracePeriod = 5 * time.Second

// HttpServer is a BootstrapHandler that mirrors the generic
// pkg/bootstrap/handlers.HttpServer but omits the request-timeout middleware
// and the http.Server WriteTimeout, both of which would terminate MCP
// Streamable HTTP / SSE connections.
type HttpServer struct {
	router           *echo.Echo
	isRunning        atomic.Bool
	doListenAndServe bool
}

// NewHttpServer constructs an HttpServer for the given echo router.
func NewHttpServer(router *echo.Echo, doListenAndServe bool) *HttpServer {
	return &HttpServer{router: router, doListenAndServe: doListenAndServe}
}

// IsRunning reports whether the handler's lifecycle is active (started, ctx
// not yet cancelled) — true even when doListenAndServe is false. Consumers
// poll it from another goroutine for delayed shutdown, hence the atomic flag.
func (b *HttpServer) IsRunning() bool {
	return b.isRunning.Load()
}

// BootstrapHandler fulfills the BootstrapHandler contract. It installs the
// common bootstrap middlewares (sans request timeout), starts the HTTP
// server, and registers a shutdown goroutine bound to ctx.
func (b *HttpServer) BootstrapHandler(
	ctx context.Context,
	wg *sync.WaitGroup,
	_ startup.Timer,
	dic *di.Container,
) bool {
	logger := container.LoggerFrom(dic.Get)

	if !b.doListenAndServe {
		logger.Info("Web server intentionally NOT started.")
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.isRunning.Store(true)
			<-ctx.Done()
			b.isRunning.Store(false)
		}()
		return true
	}

	bootstrapConfig := container.ConfigurationFrom(dic.Get).GetBootstrap()
	if bootstrapConfig.Service == nil {
		logger.Error("Service section is missing from service's configuration")
		return false
	}
	if bootstrapConfig.Service.Port == 0 {
		logger.Error("Service.Port is missing from service's configuration or should not be 0")
		return false
	}
	addr := serverAddress(bootstrapConfig.Service.ServerBindAddr, bootstrapConfig.Service.Host, bootstrapConfig.Service.Port)

	timeout, err := time.ParseDuration(bootstrapConfig.Service.RequestTimeout)
	if err != nil {
		logger.Errorf("unable to parse RequestTimeout value of %s to a duration: %v", bootstrapConfig.Service.RequestTimeout, err)
		return false
	}

	// NOTE: RequestTimeoutMiddleware intentionally omitted so the MCP
	// Streamable HTTP transport is not cut off (or buffered) mid-stream.
	b.router.Use(handlers.ManageHeader)
	b.router.Use(handlers.LoggingMiddleware(logger))
	b.router.Use(handlers.RequestLimitMiddleware(bootstrapConfig.Service.MaxRequestSize, logger))

	// NOTE: WriteTimeout intentionally omitted to keep streamed responses
	// open. ReadTimeout still bounds slow request bodies, and
	// ReadHeaderTimeout guards against Slowloris-style header dribbling.
	server := &http.Server{
		Addr:              addr,
		Handler:           b.router,
		ReadTimeout:       timeout,
		ReadHeaderTimeout: 5 * time.Second,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		logger.Info("Web server shutting down")
		// Bound the shutdown so live MCP Streamable HTTP / SSE handlers cannot
		// block process exit indefinitely. Long-lived agent sessions never
		// return on their own; force-close after the deadline.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Warnf("graceful web server shutdown exceeded %s; forcing close: %v", shutdownGracePeriod, err)
			_ = server.Close()
		}
		logger.Info("Web server shut down")
	}()

	logger.Info("Web server starting (" + addr + ")")

	wg.Add(1)
	go func() {
		defer func() {
			wg.Done()
			b.isRunning.Store(false)
		}()
		b.isRunning.Store(true)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Web server failed: %v", err)
			cancel := container.CancelFuncFrom(dic.Get)
			cancel()
		} else {
			logger.Info("Web server stopped")
		}
	}()

	return true
}

// serverAddress prefers ServerBindAddr; falls back to Host for backwards
// compatibility when ServerBindAddr is unset.
func serverAddress(bindAddr, host string, port int) string {
	p := strconv.Itoa(port)
	if bindAddr != "" {
		return net.JoinHostPort(bindAddr, p)
	}
	return net.JoinHostPort(host, p)
}
