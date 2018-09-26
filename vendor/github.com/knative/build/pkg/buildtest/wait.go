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

package buildtest

import (
	"sync"
	"time"
)

// Wait extends WaitGroup with a few useful methods to avoid tests having a
// failure mode of hanging.
type Wait struct {
	sync.WaitGroup
}

// Add should be overridden by implementors.
func (w *Wait) Add(delta int) {
	panic("Don't call Add(int) on wait!")
}

// In calls Done() after the specified duration.
func (w *Wait) In(d time.Duration) {
	go func() {
		time.Sleep(d)
		w.Done()
	}()
}

// WaitNop is a convenience for passing into WaitUntil.
func WaitNop() {}

// WaitUntil waits until Done() has been called or the specified duration has
// elapsed.  If Done() is called, then onSuccess is called.  If the duration
// elapses without Done() being called, then onTimeout is called.
func (w *Wait) WaitUntil(d time.Duration, onSuccess func(), onTimeout func()) {
	ch := make(chan struct{})
	go func() {
		w.Wait()
		close(ch)
	}()
	select {
	case <-ch:
		onSuccess()
	case <-time.After(d):
		onTimeout()
	}
}

// NewWait is a convenience for creating a Wait object.
func NewWait() *Wait {
	w := Wait{}
	w.WaitGroup.Add(1)
	return &w
}
