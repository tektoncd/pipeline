/*
Copyright 2021 The Knative Authors

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

package informer

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

// namedWaitGroup is used to increment and decrement a WaitGroup by name
type namedWaitGroup struct {
	waitGroup sync.WaitGroup
	keys      sets.String
	mu        sync.Mutex
}

// newNamedWaitGroup returns an instantiated namedWaitGroup.
func newNamedWaitGroup() *namedWaitGroup {
	return &namedWaitGroup{
		keys: sets.NewString(),
	}
}

// Add will add the key to the list of keys being tracked and increment the wait group.
// If the key has already been added, the wait group will not be incremented again.
func (n *namedWaitGroup) Add(key string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.keys.Has(key) {
		n.keys.Insert(key)
		n.waitGroup.Add(1)
	}
}

// Done will decrement the counter if the key is present in the tracked keys. If it is not present
// it will be ignored.
func (n *namedWaitGroup) Done(key string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.keys.Has(key) {
		n.keys.Delete(key)
		n.waitGroup.Done()
	}
}

// Wait will wait for the underlying waitGroup to complete.
func (n *namedWaitGroup) Wait() {
	n.waitGroup.Wait()
}

// syncedCallback can be used to wait for a callback to be called at least once for a list of keys.
type syncedCallback struct {
	// namedWaitGroup will block until the callback has been called for all tracked entities
	namedWaitGroup *namedWaitGroup

	// callback is the callback that is intended to be called at least once for each key
	// being tracked via WaitGroup
	callback func(obj interface{})
}

// newSyncedCallback will return a syncedCallback that will track the provided keys.
func newSyncedCallback(keys []string, callback func(obj interface{})) *syncedCallback {
	s := &syncedCallback{
		callback:       callback,
		namedWaitGroup: newNamedWaitGroup(),
	}
	for _, key := range keys {
		s.namedWaitGroup.Add(key)
	}
	return s
}

// Event is intended to be a wrapper for the actual event handler; this wrapper will signal via
// the wait group that the event handler has been called at least once for the key.
func (s *syncedCallback) Call(obj interface{}, key string) {
	s.callback(obj)
	s.namedWaitGroup.Done(key)
}

// WaitForAllKeys will block until s.Call has been called for all the keys we are tracking or the stop signal is
// received.
func (s *syncedCallback) WaitForAllKeys(stopCh <-chan struct{}) error {
	c := make(chan struct{})
	go func() {
		defer close(c)
		s.namedWaitGroup.Wait()
	}()
	select {
	case <-c:
		return nil
	case <-stopCh:
		return wait.ErrWaitTimeout
	}
}
