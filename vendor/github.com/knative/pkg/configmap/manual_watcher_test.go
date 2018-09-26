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

func TestManualStartNOOP(t *testing.T) {
	watcher := ManualWatcher{
		Namespace: "default",
	}
	if err := watcher.Start(nil); err != nil {
		t.Errorf("Unexpected error watcher.Start() = %v", err)
	}
}

func TestCallbackInvoked(t *testing.T) {
	watcher := ManualWatcher{
		Namespace: "default",
	}

	observer := counter{}

	watcher.Watch("foo", observer.callback)
	watcher.OnChange(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		},
	})

	if observer.count == 0 {
		t.Errorf("Expected callback to be invoked - got invocations %v", observer.count)
	}
}

func TestDifferentNamespace(t *testing.T) {
	watcher := ManualWatcher{
		Namespace: "default",
	}

	observer := counter{}

	watcher.Watch("foo", observer.callback)
	watcher.OnChange(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "not-default",
			Name:      "foo",
		},
	})

	if observer.count != 0 {
		t.Errorf("Expected callback to be not be invoked - got invocations %v", observer.count)
	}
}

func TestLateRegistration(t *testing.T) {
	watcher := ManualWatcher{
		Namespace: "default",
	}

	observer := counter{}

	watcher.OnChange(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		},
	})

	watcher.Watch("foo", observer.callback)

	if observer.count != 0 {
		t.Errorf("Expected callback to be not be invoked - got invocations %v", observer.count)
	}
}

func TestDifferentConfigName(t *testing.T) {
	watcher := ManualWatcher{
		Namespace: "default",
	}

	observer := counter{}

	watcher.OnChange(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "foo",
		},
	})

	watcher.Watch("bar", observer.callback)

	if observer.count != 0 {
		t.Errorf("Expected callback to be not be invoked - got invocations %v", observer.count)
	}
}
