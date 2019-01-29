/*
Copyright 2019 The Knative Authors.

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

package entrypoint

import (
	"log"
	"os"
	"time"
)

// Waiter encapsulates waiting for files to exist.
type Waiter interface {
	// Wait blocks until the specified file exists.
	Wait(file string)
}

// RealWaiter actually waits for files, by polling.
type RealWaiter struct{ waitFile string }

var _ Waiter = (*RealWaiter)(nil)

func (*RealWaiter) Wait(file string) {
	if file == "" {
		return
	}
	for ; ; time.Sleep(time.Second) {
		if _, err := os.Stat(file); err == nil {
			return
		} else if !os.IsNotExist(err) {
			log.Fatalf("Waiting for %q: %v", file, err)
		}
	}
}
