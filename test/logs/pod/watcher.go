/*
Copyright 2017 Google Inc. All Rights Reserved.
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

package pod

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type Watcher struct {
	Pods corev1.PodInterface
	Name string

	versions chan *v1.Pod
	last     *v1.Pod
}

/*
Start watches the pod with Name using watch API
*/
func (w *Watcher) Start(ctx context.Context) error {
	w.versions = make(chan *v1.Pod, 100)

	watcher, err := w.Pods.Watch(metav1.ListOptions{
		IncludeUninitialized: true,
		FieldSelector:        fields.OneTermEqualSelector("metadata.name", w.Name).String(),
	})
	if err != nil {
		return fmt.Errorf("watching pod: %v", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case evt := <-watcher.ResultChan():
				// TODO: reconnect watch
				w.versions <- evt.Object.(*v1.Pod)
			}
		}
	}()

	return nil
}

func (w *Watcher) WaitForPod(ctx context.Context, predicate func(pod *v1.Pod) bool) (*v1.Pod, error) {
	if w.last != nil && predicate(w.last) {
		return w.last, nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("watch cancelled")
		case w.last = <-w.versions:
			if predicate(w.last) {
				return w.last, nil
			}
		}
	}
}
