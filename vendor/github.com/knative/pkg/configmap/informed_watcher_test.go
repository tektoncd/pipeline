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

package configmap

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

type counter struct {
	name  string
	count int
}

func (c *counter) callback(*corev1.ConfigMap) {
	c.count++
}

func TestInformedWatcher(t *testing.T) {
	fooCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		},
	}
	barCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "bar",
		},
	}
	kc := fakekubeclientset.NewSimpleClientset(fooCM, barCM)
	cm := NewInformedWatcher(kc, "default")

	foo1 := &counter{name: "foo1"}
	foo2 := &counter{name: "foo2"}
	bar := &counter{name: "bar"}
	cm.Watch("foo", foo1.callback)
	cm.Watch("foo", foo2.callback)
	cm.Watch("bar", bar.callback)

	stopCh := make(chan struct{})
	defer close(stopCh)
	err := cm.Start(stopCh)
	if err != nil {
		t.Fatalf("cm.Start() = %v", err)
	}

	// When Start returns the callbacks should have been called with the
	// version of the objects that is available.
	for _, obj := range []*counter{foo1, foo2, bar} {
		if got, want := obj.count, 1; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}

	// After a "foo" event, the "foo" watchers should have 2,
	// and the "bar" watchers should still have 1
	cm.updateConfigMapEvent(nil, fooCM)
	for _, obj := range []*counter{foo1, foo2} {
		if got, want := obj.count, 2; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}

	for _, obj := range []*counter{bar} {
		if got, want := obj.count, 1; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}

	// After a "foo" and "bar" event, the "foo" watchers should have 3,
	// and the "bar" watchers should still have 2
	cm.updateConfigMapEvent(nil, fooCM)
	cm.updateConfigMapEvent(nil, barCM)
	for _, obj := range []*counter{foo1, foo2} {
		if got, want := obj.count, 3; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}
	for _, obj := range []*counter{bar} {
		if got, want := obj.count, 2; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}

	// After a "bar" event, all watchers should have 3
	cm.updateConfigMapEvent(nil, barCM)
	for _, obj := range []*counter{foo1, foo2, bar} {
		if got, want := obj.count, 3; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}

	// After an unwatched ConfigMap update, no change.

	cm.updateConfigMapEvent(nil, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "not-watched",
		},
	})
	for _, obj := range []*counter{foo1, foo2, bar} {
		if got, want := obj.count, 3; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}

	// After a change in an unrelated namespace, no change.
	cm.updateConfigMapEvent(nil, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "not-default",
			Name:      "foo",
		},
	})
	for _, obj := range []*counter{foo1, foo2, bar} {
		if got, want := obj.count, 3; got != want {
			t.Errorf("%v.count = %v, want %v", obj, got, want)
		}
	}
}

func TestWatchMissingFailsOnStart(t *testing.T) {
	kc := fakekubeclientset.NewSimpleClientset()
	cm := NewInformedWatcher(kc, "default")

	foo1 := &counter{name: "foo1"}
	cm.Watch("foo", foo1.callback)

	stopCh := make(chan struct{})
	defer close(stopCh)

	// This should error because we don't have a ConfigMap named "foo".
	err := cm.Start(stopCh)
	if err == nil {
		t.Fatal("cm.Start() succeeded, wanted error")
	}
}

func TestErrorOnMultipleStarts(t *testing.T) {
	fooCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		},
	}
	kc := fakekubeclientset.NewSimpleClientset(fooCM)
	cm := NewInformedWatcher(kc, "default")

	foo1 := &counter{name: "foo1"}
	cm.Watch("foo", foo1.callback)

	stopCh := make(chan struct{})
	defer close(stopCh)

	// This should succeed because the watched resource exists.
	if err := cm.Start(stopCh); err != nil {
		t.Fatalf("cm.Start() = %v", err)
	}

	// This should error because we already called Start()
	if err := cm.Start(stopCh); err == nil {
		t.Fatal("cm.Start() succeeded, wanted error")
	}
}
