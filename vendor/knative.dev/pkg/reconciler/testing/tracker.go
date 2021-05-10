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
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/tracker"
)

// NullTracker implements Tracker
//
// Alias is preserved for backwards compatibility
type NullTracker = FakeTracker

// FakeTracker implements Tracker.
type FakeTracker struct {
	sync.Mutex
	references map[tracker.Reference]map[types.NamespacedName]struct{}
}

var _ tracker.Interface = (*FakeTracker)(nil)

// OnChanged implements OnChanged.
func (*FakeTracker) OnChanged(interface{}) {}

// GetObservers implements GetObservers.
func (n *FakeTracker) GetObservers(obj interface{}) []types.NamespacedName {
	item, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		return nil
	}

	or := kmeta.ObjectReference(item)
	ref := tracker.Reference{
		APIVersion: or.APIVersion,
		Kind:       or.Kind,
		Namespace:  or.Namespace,
		Name:       or.Name,
	}

	n.Lock()
	defer n.Unlock()

	keys := make([]types.NamespacedName, 0, len(n.references[ref]))
	for key := range n.references[ref] {
		keys = append(keys, key)
	}
	return keys
}

// OnDeletedObserver implements OnDeletedObserver.
func (n *FakeTracker) OnDeletedObserver(obj interface{}) {
	item, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		return
	}
	key := types.NamespacedName{Namespace: item.GetNamespace(), Name: item.GetName()}

	n.Lock()
	defer n.Unlock()

	for ref, objs := range n.references {
		delete(objs, key)
		if len(objs) == 0 {
			delete(n.references, ref)
		}
	}
}

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
	item, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		return err
	}
	key := types.NamespacedName{Namespace: item.GetNamespace(), Name: item.GetName()}

	n.Lock()
	defer n.Unlock()

	if n.references == nil {
		n.references = make(map[tracker.Reference]map[types.NamespacedName]struct{}, 1)
	}

	objs := n.references[ref]
	if objs == nil {
		objs = make(map[types.NamespacedName]struct{}, 1)
	}
	objs[key] = struct{}{}
	n.references[ref] = objs

	return nil
}

// References returns the list of objects being tracked
func (n *FakeTracker) References() []tracker.Reference {
	n.Lock()
	defer n.Unlock()

	refs := make([]tracker.Reference, 0, len(n.references))
	for ref := range n.references {
		refs = append(refs, ref)
	}

	return refs
}
