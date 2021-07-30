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

package test

import (
	"os"
	"os/signal"
	"sync"
)

type logFunc func(template string, args ...interface{})

var cleanup struct {
	once  sync.Once
	mutex sync.RWMutex
	funcs []func()
}

func waitForInterrupt() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c

		cleanup.mutex.RLock()
		defer cleanup.mutex.RUnlock()

		for i := len(cleanup.funcs) - 1; i >= 0; i-- {
			cleanup.funcs[i]()
		}

		os.Exit(1)
	}()
}

// CleanupOnInterrupt will execute the function if an interrupt signal is caught
// Deprecated - use OnInterrupt
func CleanupOnInterrupt(f func(), log logFunc) {
	OnInterrupt(f)
}

// OnInterrupt registers a cleanup function to run if an interrupt signal is caught
func OnInterrupt(cleanupFunc func()) {
	cleanup.once.Do(waitForInterrupt)

	cleanup.mutex.Lock()
	defer cleanup.mutex.Unlock()

	cleanup.funcs = append(cleanup.funcs, cleanupFunc)
}
