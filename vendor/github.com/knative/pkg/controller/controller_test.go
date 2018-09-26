/*
Copyright 2017 The Knative Authors

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

package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/workqueue"

	. "github.com/knative/pkg/logging/testing"
	. "github.com/knative/pkg/testing"
)

func TestPassNew(t *testing.T) {
	old := "foo"
	new := "bar"

	PassNew(func(got interface{}) {
		if new != got.(string) {
			t.Errorf("PassNew() = %v, wanted %v", got, new)
		}
	})(old, new)
}

var (
	boolTrue  = true
	boolFalse = false
	gvk       = schema.GroupVersionKind{
		Group:   "pkg.knative.dev",
		Version: "v1meta1",
		Kind:    "Parent",
	}
)

func TestFilter(t *testing.T) {
	filter := Filter(gvk)

	tests := []struct {
		name  string
		input interface{}
		want  bool
	}{{
		name:  "not a metav1.Object",
		input: "foo",
		want:  false,
	}, {
		name:  "nil",
		input: nil,
		want:  false,
	}, {
		name: "no owner reference",
		input: &Resource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		},
		want: false,
	}, {
		name: "wrong owner reference, not controller",
		input: &Resource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "another.knative.dev/v1beta3",
					Kind:       "Parent",
					Controller: &boolFalse,
				}},
			},
		},
		want: false,
	}, {
		name: "right owner reference, not controller",
		input: &Resource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: gvk.GroupVersion().String(),
					Kind:       gvk.Kind,
					Controller: &boolFalse,
				}},
			},
		},
		want: false,
	}, {
		name: "wrong owner reference, but controller",
		input: &Resource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: "another.knative.dev/v1beta3",
					Kind:       "Parent",
					Controller: &boolTrue,
				}},
			},
		},
		want: false,
	}, {
		name: "right owner reference, is controller",
		input: &Resource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: gvk.GroupVersion().String(),
					Kind:       gvk.Kind,
					Controller: &boolTrue,
				}},
			},
		},
		want: true,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := filter(test.input)
			if test.want != got {
				t.Errorf("Filter() = %v, wanted %v", got, test.want)
			}
		})
	}
}

type NopReconciler struct{}

func (nr *NopReconciler) Reconcile(context.Context, string) error {
	return nil
}

func TestEnqueues(t *testing.T) {
	tests := []struct {
		name      string
		work      func(*Impl)
		wantQueue []string
	}{{
		name: "do nothing",
		work: func(*Impl) {},
	}, {
		name: "enqueue key",
		work: func(impl *Impl) {
			impl.EnqueueKey("foo/bar")
		},
		wantQueue: []string{"foo/bar"},
	}, {
		name: "enqueue duplicate key",
		work: func(impl *Impl) {
			impl.EnqueueKey("foo/bar")
			impl.EnqueueKey("foo/bar")
		},
		// The queue deduplicates.
		wantQueue: []string{"foo/bar"},
	}, {
		name: "enqueue different keys",
		work: func(impl *Impl) {
			impl.EnqueueKey("foo/bar")
			impl.EnqueueKey("foo/baz")
		},
		wantQueue: []string{"foo/bar", "foo/baz"},
	}, {
		name: "enqueue resource",
		work: func(impl *Impl) {
			impl.Enqueue(&Resource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			})
		},
		wantQueue: []string{"bar/foo"},
	}, {
		name: "enqueue bad resource",
		work: func(impl *Impl) {
			impl.Enqueue("baz/blah")
		},
	}, {
		name: "enqueue controller of bad resource",
		work: func(impl *Impl) {
			impl.EnqueueControllerOf("baz/blah")
		},
	}, {
		name: "enqueue controller of resource without owner",
		work: func(impl *Impl) {
			impl.EnqueueControllerOf(&Resource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			})
		},
	}, {
		name: "enqueue controller of resource with owner",
		work: func(impl *Impl) {
			impl.EnqueueControllerOf(&Resource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: gvk.GroupVersion().String(),
						Kind:       gvk.Kind,
						Name:       "baz",
						Controller: &boolTrue,
					}},
				},
			})
		},
		wantQueue: []string{"bar/baz"},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			impl := NewImpl(&NopReconciler{}, TestLogger(t), "Testing")
			test.work(impl)

			// The rate limit on our queue delays when things are added to the queue.
			time.Sleep(10 * time.Millisecond)
			impl.WorkQueue.ShutDown()
			gotQueue := drainWorkQueue(impl.WorkQueue)

			if diff := cmp.Diff(test.wantQueue, gotQueue); diff != "" {
				t.Errorf("unexpected queue (-want +got): %s", diff)
			}
		})
	}
}

type CountingReconciler struct {
	Count int
}

func (cr *CountingReconciler) Reconcile(context.Context, string) error {
	cr.Count++
	return nil
}

func TestStartAndShutdown(t *testing.T) {
	r := &CountingReconciler{}
	impl := NewImpl(r, TestLogger(t), "Testing")

	stopCh := make(chan struct{})

	var eg errgroup.Group
	eg.Go(func() error {
		return impl.Run(1, stopCh)
	})

	time.Sleep(10 * time.Millisecond)
	close(stopCh)

	if err := eg.Wait(); err != nil {
		t.Errorf("Wait() = %v", err)
	}

	if got, want := r.Count, 0; got != want {
		t.Errorf("Count = %v, wanted %v", got, want)
	}
}

func TestStartAndShutdownWithWork(t *testing.T) {
	r := &CountingReconciler{}
	impl := NewImpl(r, TestLogger(t), "Testing")

	stopCh := make(chan struct{})

	impl.EnqueueKey("foo/bar")

	var eg errgroup.Group
	eg.Go(func() error {
		return impl.Run(1, stopCh)
	})

	time.Sleep(10 * time.Millisecond)
	close(stopCh)

	if err := eg.Wait(); err != nil {
		t.Errorf("Wait() = %v", err)
	}

	if got, want := r.Count, 1; got != want {
		t.Errorf("Count = %v, wanted %v", got, want)
	}
	if got, want := impl.WorkQueue.NumRequeues("foo/bar"), 0; got != want {
		t.Errorf("Count = %v, wanted %v", got, want)
	}
}

type ErrorReconciler struct{}

func (er *ErrorReconciler) Reconcile(context.Context, string) error {
	return errors.New("I always error")
}

func TestStartAndShutdownWithErroringWork(t *testing.T) {
	r := &ErrorReconciler{}
	impl := NewImpl(r, TestLogger(t), "Testing")

	stopCh := make(chan struct{})

	impl.EnqueueKey("foo/bar")

	var eg errgroup.Group
	eg.Go(func() error {
		return impl.Run(1, stopCh)
	})

	time.Sleep(10 * time.Millisecond)
	close(stopCh)

	if err := eg.Wait(); err != nil {
		t.Errorf("Wait() = %v", err)
	}

	// Check that the work was requeued.
	if got, want := impl.WorkQueue.NumRequeues("foo/bar"), 1; got != want {
		t.Errorf("Count = %v, wanted %v", got, want)
	}
}

func TestStartAndShutdownWithInvalidWork(t *testing.T) {
	r := &CountingReconciler{}
	impl := NewImpl(r, TestLogger(t), "Testing")

	stopCh := make(chan struct{})

	// Add a nonsense work item, which we couldn't ordinarily get into our workqueue.
	thing := struct{}{}
	impl.WorkQueue.AddRateLimited(thing)

	var eg errgroup.Group
	eg.Go(func() error {
		return impl.Run(1, stopCh)
	})

	time.Sleep(10 * time.Millisecond)
	close(stopCh)

	if err := eg.Wait(); err != nil {
		t.Errorf("Wait() = %v", err)
	}

	if got, want := r.Count, 0; got != want {
		t.Errorf("Count = %v, wanted %v", got, want)
	}
	if got, want := impl.WorkQueue.NumRequeues(thing), 0; got != want {
		t.Errorf("Count = %v, wanted %v", got, want)
	}
}

func drainWorkQueue(wq workqueue.RateLimitingInterface) (hasQueue []string) {
	for {
		key, shutdown := wq.Get()
		if key == nil && shutdown {
			break
		}
		hasQueue = append(hasQueue, key.(string))
	}
	return
}
