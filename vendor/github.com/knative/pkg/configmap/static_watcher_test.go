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
)

func TestStaticWatcher(t *testing.T) {
	fooCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-system",
			Name:      "foo",
		},
	}
	barCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "knative-system",
			Name:      "bar",
		},
	}

	cm := NewStaticWatcher(fooCM, barCM)

	foo1 := &counter{name: "foo1"}
	cm.Watch("foo", foo1.callback)
	foo2 := &counter{name: "foo2"}
	cm.Watch("foo", foo2.callback)
	bar := &counter{name: "bar"}
	cm.Watch("bar", bar.callback)

	err := cm.Start(nil)
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
}

func TestUnknownConfigMapName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Expected calling Watch with an unknown configmap name to panic")
		}
	}()

	cm := NewStaticWatcher()
	cm.Watch("unknown", func(*corev1.ConfigMap) {})
}
