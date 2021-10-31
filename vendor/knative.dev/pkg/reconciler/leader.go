/*
Copyright 2020 The Knative Authors

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

package reconciler

import (
	"sync"

	"k8s.io/apimachinery/pkg/types"
)

// Bucket is an opaque type used to scope leadership.
type Bucket interface {
	// Name returns a string representing this bucket, which uniquely
	// identifies the bucket and is suitable for use as a resource lock name.
	Name() string

	// Has determines whether this Bucket contains a particular key.
	Has(key types.NamespacedName) bool
}

// UniversalBucket returns a Bucket that "Has()" all keys.
func UniversalBucket() Bucket {
	return &bucket{}
}

type bucket struct{}

var _ Bucket = (*bucket)(nil)

// Name implements Bucket
func (b *bucket) Name() string {
	return ""
}

// Has implements Bucket
func (b *bucket) Has(nn types.NamespacedName) bool {
	return true
}

// LeaderAware is implemented by Reconcilers that are aware of their leader status.
type LeaderAware interface {
	// Promote is called when we become the leader of a given Bucket.  It must be
	// supplied with an enqueue function through which a Bucket resync may be triggered.
	Promote(b Bucket, enq func(Bucket, types.NamespacedName)) error

	// Demote is called when we stop being the leader for the specified Bucket.
	Demote(Bucket)
}

// LeaderAwareFuncs implements LeaderAware using the given functions for handling
// promotion and demotion.
type LeaderAwareFuncs struct {
	sync.RWMutex
	buckets map[string]Bucket

	PromoteFunc func(b Bucket, enq func(Bucket, types.NamespacedName)) error
	DemoteFunc  func(b Bucket)
}

var _ LeaderAware = (*LeaderAwareFuncs)(nil)

// IsLeaderFor implements LeaderAware
func (laf *LeaderAwareFuncs) IsLeaderFor(key types.NamespacedName) bool {
	laf.RLock()
	defer laf.RUnlock()

	for _, bkt := range laf.buckets {
		if bkt.Has(key) {
			return true
		}
	}
	return false
}

// Promote implements LeaderAware
func (laf *LeaderAwareFuncs) Promote(b Bucket, enq func(Bucket, types.NamespacedName)) error {
	func() {
		laf.Lock()
		defer laf.Unlock()
		if laf.buckets == nil {
			laf.buckets = make(map[string]Bucket, 1)
		}
		laf.buckets[b.Name()] = b
	}()

	if promote := laf.PromoteFunc; promote != nil && enq != nil {
		return promote(b, enq)
	}
	return nil
}

// Demote implements LeaderAware
func (laf *LeaderAwareFuncs) Demote(b Bucket) {
	func() {
		laf.Lock()
		defer laf.Unlock()
		delete(laf.buckets, b.Name())
	}()

	if demote := laf.DemoteFunc; demote != nil {
		demote(b)
	}
}
