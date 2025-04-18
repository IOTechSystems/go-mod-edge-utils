/*******************************************************************************
 * Copyright 2019 Dell Inc.
 * Copyright 2023 Intel Corporation
 * Copyright 2023 IOTech Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/configprocessor"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/container"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/environment"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/flags"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/interfaces"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/secret"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/bootstrap/startup"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/di"
	"github.com/IOTechSystems/go-mod-edge-utils/pkg/log"
)

// Deferred defines the signature of a function returned by RunAndReturnWaitGroup that should be executed via defer.
type Deferred func()

// fatalError logs an error and exits the application.  It's intended to be used only within the bootstrap prior to
// any go routines being spawned.
func fatalError(err error, logger log.Logger) {
	logger.Error(err.Error())
	os.Exit(1)
}

// translateInterruptToCancel spawns a go routine to translate the receipt of a SIGTERM signal to a call to cancel
// the context used by the bootstrap implementation.
func translateInterruptToCancel(ctx context.Context, wg *sync.WaitGroup, cancel context.CancelFunc) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		signalStream := make(chan os.Signal, 1)
		defer func() {
			signal.Stop(signalStream)
			close(signalStream)
		}()
		signal.Notify(signalStream, os.Interrupt, syscall.SIGTERM)
		select {
		case <-signalStream:
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}()
}

// RunAndReturnWaitGroup bootstraps an application.  It loads configuration and calls the provided list of handlers.
// Any long-running process should be spawned as a go routine in a handler.  Handlers are expected to return
// immediately.  Once all of the handlers are called this function will return a sync.WaitGroup reference to the caller.
// It is intended that the caller take whatever additional action makes sense before calling Wait() on the returned
// reference to wait for the application to be signaled to stop (and the corresponding goroutines spawned in the
// various handlers to be stopped cleanly).
func RunAndReturnWaitGroup(
	ctx context.Context,
	cancel context.CancelFunc,
	commonFlags flags.Common,
	serviceKey string,
	serviceConfig interfaces.Configuration,
	startupTimer startup.Timer,
	dic *di.Container,
	useSecretProvider bool,
	handlers []interfaces.BootstrapHandler) (*sync.WaitGroup, Deferred, bool) {

	var err error
	var wg sync.WaitGroup
	deferred := func() {}

	// Check if service provided an initial Logger to use. If not create one and add it to the DIC.
	logger := container.LoggerFrom(dic.Get)
	if logger == nil {
		logger = log.InitLogger(serviceKey, log.InfoLog, nil)
		dic.Update(di.ServiceConstructorMap{
			container.LoggerInterfaceName: func(get di.Get) any {
				return logger
			},
		})
	}

	translateInterruptToCancel(ctx, &wg, cancel)

	envVars := environment.NewVariables(logger)

	var secretProvider interfaces.SecretProvider
	if useSecretProvider {
		secretProvider, err = secret.NewSecretProvider(serviceConfig, envVars, ctx, startupTimer, dic, serviceKey)
		if err != nil {
			fatalError(fmt.Errorf("failed to create SecretProvider: %s", err.Error()), logger)
		}
	}

	// The SecretProvider is initialized and placed in the DIS as part of processing the configuration due
	// to the need for it to be used to get Access Token for the Configuration Provider and having to wait to
	// initialize it until after the configuration is loaded from file.
	configProcessor := configprocessor.NewProcessor(commonFlags, envVars, startupTimer, ctx, &wg, dic)
	if err := configProcessor.Process(serviceKey, serviceConfig, secretProvider); err != nil {
		fatalError(err, logger)
	}

	dic.Update(di.ServiceConstructorMap{
		container.ConfigurationInterfaceName: func(get di.Get) any {
			return serviceConfig
		},
		container.CancelFuncName: func(get di.Get) any {
			return cancel
		},
	})

	// call individual bootstrap handlers.
	startedSuccessfully := true
	for i := range handlers {
		if !handlers[i](ctx, &wg, startupTimer, dic) {
			cancel()
			startedSuccessfully = false
			break
		}
	}

	return &wg, deferred, startedSuccessfully
}

// Run bootstraps an application.  It loads configuration and calls the provided list of handlers.  Any long-running
// process should be spawned as a go routine in a handler.  Handlers are expected to return immediately.  Once all of
// the handlers are called this function will wait for any go routines spawned inside the handlers to exit before
// returning to the caller.  It is intended that the caller stop executing on the return of this function.
func Run(
	ctx context.Context,
	cancel context.CancelFunc,
	commonFlags flags.Common,
	serviceKey string,
	serviceConfig interfaces.Configuration,
	startupTimer startup.Timer,
	dic *di.Container,
	useSecretProvider bool,
	handlers []interfaces.BootstrapHandler) {

	wg, deferred, success := RunAndReturnWaitGroup(
		ctx,
		cancel,
		commonFlags,
		serviceKey,
		serviceConfig,
		startupTimer,
		dic,
		useSecretProvider,
		handlers,
	)

	if !success {
		// This only occurs when a bootstrap handler has fail.
		// The handler will have logged an error, so not need to log a message here.
		cancel()
		os.Exit(1)
	}

	defer deferred()

	// wait for go routines to stop executing.
	wg.Wait()
}
