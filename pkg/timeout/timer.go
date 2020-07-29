/*
Copyright 2020 The Tekton Authors

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

package timeout

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Timer knows how to track channels that can be used to communicate with go routines
type Timer struct {
	logger *zap.SugaredLogger

	// done is a map from the name of the object being watched to the channel to use to indicate
	// it is done (and so there is no need to wait on it any longer)
	done map[string]chan bool
	// doneMut is a mutex that protects access to done to ensure that multiple goroutines
	// don't try to update it simultaneously
	doneMut sync.Mutex

	// stopCh is used to signal to all goroutines that they should stop, e.g. because
	// the reconciler is stopping
	stopCh <-chan struct{}
}

// NewTimer returns an instance of Timer with the specified stopCh and logger, instantiated
// and ready to be used.
func NewTimer(
	stopCh <-chan struct{},
	logger *zap.SugaredLogger,
) *Timer {
	return &Timer{
		stopCh: stopCh,
		done:   make(map[string]chan bool),
		logger: logger,
	}
}

// Release deletes channels and data that are specific to a StatusKey object.
func (t *Timer) Release(runObj StatusKey) {
	t.doneMut.Lock()
	defer t.doneMut.Unlock()

	key := runObj.GetRunKey()
	if done, ok := t.done[key]; ok {
		delete(t.done, key)
		close(done)
	}
}

func (t *Timer) getOrCreateDoneChan(runObj StatusKey) chan bool {
	key := runObj.GetRunKey()
	t.doneMut.Lock()
	defer t.doneMut.Unlock()
	var done chan bool
	var ok bool
	if done, ok = t.done[key]; !ok {
		done = make(chan bool)
	}
	t.done[key] = done
	return done
}

// SetTimer waits until either the timeout has elapsed or it has recieved a signal via the done channel
// or t.stopCh indicating whatever we were waiting for has completed. If the timeout expires, callback
// is called with runObj.
func (t *Timer) SetTimer(runObj StatusKey, timeout time.Duration, callback func(interface{})) {
	done := t.getOrCreateDoneChan(runObj)
	started := time.Now()
	select {
	case <-t.stopCh:
		t.logger.Infof("stopping timer for %q", runObj.GetRunKey())
		return
	case <-done:
		t.logger.Infof("%q finished, stopping timer", runObj.GetRunKey())
		return
	case <-time.After(timeout):
		t.logger.Infof("timer for %q has activated after %s", runObj.GetRunKey(), time.Since(started).String())
		callback(runObj)
	}
}
