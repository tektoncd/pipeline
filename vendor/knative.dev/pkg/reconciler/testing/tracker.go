/*
Copyright 2019 The Knative Authors

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

package testing

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/tracker"
)

// NullTracker implements Tracker
//
// Alias is preserved for backwards compatibility
type NullTracker = FakeTracker

// FakeTracker implements Tracker.
type FakeTracker struct {
	sync.Mutex
	references []tracker.Reference
}

var _ tracker.Interface = (*FakeTracker)(nil)

// OnChanged implements OnChanged.
func (*FakeTracker) OnChanged(interface{}) {}

// Track implements tracker.Interface.
func (n *FakeTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	return n.TrackReference(tracker.Reference{
		APIVersion: ref.APIVersion,
		Kind:       ref.Kind,
		Namespace:  ref.Namespace,
		Name:       ref.Name,
	}, obj)
}

// TrackReference implements tracker.Interface.
func (n *FakeTracker) TrackReference(ref tracker.Reference, obj interface{}) error {
	n.Lock()
	defer n.Unlock()

	n.references = append(n.references, ref)
	return nil
}

// References returns the list of objects being tracked
func (n *FakeTracker) References() []tracker.Reference {
	n.Lock()
	defer n.Unlock()

	return append(n.references[:0:0], n.references...)
}
