/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package signals

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"time"
)

var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandler registered for SIGTERM and SIGINT. A stop channel is returned
// which is closed on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler) // panics when called twice

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

// NewContext creates a new context with SetupSignalHandler()
// as our Done() channel.
func NewContext() context.Context {
	return &signalContext{stopCh: SetupSignalHandler()}
}

type signalContext struct {
	stopCh <-chan struct{}
}

// Deadline implements context.Context
func (scc *signalContext) Deadline() (deadline time.Time, ok bool) {
	return
}

// Done implements context.Context
func (scc *signalContext) Done() <-chan struct{} {
	return scc.stopCh
}

// Err implements context.Context
func (scc *signalContext) Err() error {
	select {
	case _, ok := <-scc.Done():
		if !ok {
			return errors.New("received a termination signal")
		}
	default:
	}
	return nil
}

// Value implements context.Context
func (scc *signalContext) Value(key interface{}) interface{} {
	return nil
}
