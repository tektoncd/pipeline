/*
Copyright 2026 The Tekton Authors

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

package notifications_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestConfigStoreFromContext(t *testing.T) {
	testAssets, cancel := ntesting.InitializeTestAssets(t, &test.Data{})
	defer cancel()

	cmw := cminformer.NewInformedWatcher(testAssets.Clients.Kube, system.Namespace())
	store := notifications.ConfigStoreFromContext(testAssets.Ctx, cmw)
	if store == nil {
		t.Fatal("expected a non-nil config store")
	}
}

func TestControllerOptions(t *testing.T) {
	testAssets, cancel := ntesting.InitializeTestAssets(t, &test.Data{})
	defer cancel()

	cmw := cminformer.NewInformedWatcher(testAssets.Clients.Kube, system.Namespace())
	store := notifications.ConfigStoreFromContext(testAssets.Ctx, cmw)

	optsFn := notifications.ControllerOptions("TestAgent", store)
	opts := optsFn(nil)

	if opts.AgentName != "TestAgent" {
		t.Errorf("expected AgentName %q, got %q", "TestAgent", opts.AgentName)
	}
	if opts.ConfigStore != store {
		t.Error("expected ConfigStore to match the provided store")
	}
	if !opts.SkipStatusUpdates {
		t.Error("expected SkipStatusUpdates to be true")
	}
}

func taskRun(rv string, cond *apis.Condition) *v1.TaskRun {
	tr := &v1.TaskRun{ObjectMeta: metav1.ObjectMeta{Name: "tr", Namespace: "ns", ResourceVersion: rv}}
	if cond != nil {
		tr.Status.SetCondition(cond)
	}
	return tr
}

func succeeded(status corev1.ConditionStatus, reason, message string) *apis.Condition {
	return &apis.Condition{Type: apis.ConditionSucceeded, Status: status, Reason: reason, Message: message}
}

func TestEventHandler(t *testing.T) {
	queued := taskRun("1", nil)
	running := taskRun("2", succeeded(corev1.ConditionUnknown, "Running", ""))
	runningNewMessage := taskRun("3", succeeded(corev1.ConditionUnknown, "Running", "step 2"))
	done := taskRun("4", succeeded(corev1.ConditionTrue, "Succeeded", "All Steps have completed executing"))
	failed := taskRun("4", succeeded(corev1.ConditionFalse, "Failed", "step failed"))

	doneLabelled := done.DeepCopy()
	doneLabelled.ResourceVersion = "5"
	doneLabelled.Labels = map[string]string{"foo": "bar"}
	doneDeleting := done.DeepCopy()
	doneDeleting.ResourceVersion = "6"
	doneDeleting.DeletionTimestamp = &metav1.Time{}
	doneFinalizer := done.DeepCopy()
	doneFinalizer.ResourceVersion = "7"
	doneFinalizer.Finalizers = []string{"results.tekton.dev/taskrun"}
	queuedNewRV := queued.DeepCopy()
	queuedNewRV.ResourceVersion = "8"

	cm1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", ResourceVersion: "1"}}
	cm2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", ResourceVersion: "2"}}
	cmNoRV := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}

	testcases := []struct {
		name    string
		fire    func(h cache.ResourceEventHandler)
		wantObj []interface{}
	}{{
		name:    "add is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnAdd(done, false) },
		wantObj: []interface{}{done},
	}, {
		name:    "add from initial list is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnAdd(done, true) },
		wantObj: []interface{}{done},
	}, {
		name:    "queued to running is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(queued, running) },
		wantObj: []interface{}{running},
	}, {
		name:    "running to successful is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(running, done) },
		wantObj: []interface{}{done},
	}, {
		name:    "running to failed is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(running, failed) },
		wantObj: []interface{}{failed},
	}, {
		name:    "message-only change is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(running, runningNewMessage) },
		wantObj: []interface{}{runningNewMessage},
	}, {
		name:    "resync of a terminal run is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(done, done) },
		wantObj: nil,
	}, {
		name:    "relist with an identical copy is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(failed, failed.DeepCopy()) },
		wantObj: nil,
	}, {
		name: "repeated resyncs are all dropped",
		fire: func(h cache.ResourceEventHandler) {
			for range 5 {
				h.OnUpdate(running, running)
				h.OnUpdate(done, done)
			}
		},
		wantObj: nil,
	}, {
		name:    "resync of a queued run is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(queued, queued) },
		wantObj: nil,
	}, {
		name:    "metadata-only change on a queued run is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(queued, queuedNewRV) },
		wantObj: nil,
	}, {
		name:    "label change on a terminal run is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(done, doneLabelled) },
		wantObj: nil,
	}, {
		name:    "finalizer added to a terminal run is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(done, doneFinalizer) },
		wantObj: nil,
	}, {
		name:    "deletion of a terminal run with finalizers is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(doneFinalizer, doneDeleting) },
		wantObj: nil,
	}, {
		name:    "non-run object with a new resourceVersion is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(cm1, cm2) },
		wantObj: []interface{}{cm2},
	}, {
		name:    "non-run object resync is dropped",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(cm1, cm1) },
		wantObj: nil,
	}, {
		name:    "non-run object without resourceVersion is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate(cmNoRV, cmNoRV) },
		wantObj: []interface{}{cmNoRV},
	}, {
		name:    "objects without metadata are enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnUpdate("old", "new") },
		wantObj: []interface{}{"new"},
	}, {
		name:    "delete is enqueued",
		fire:    func(h cache.ResourceEventHandler) { h.OnDelete(done) },
		wantObj: []interface{}{done},
	}, {
		name: "delete tombstone is enqueued",
		fire: func(h cache.ResourceEventHandler) {
			h.OnDelete(cache.DeletedFinalStateUnknown{Key: "ns/tr", Obj: done})
		},
		wantObj: []interface{}{cache.DeletedFinalStateUnknown{Key: "ns/tr", Obj: done}},
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			var got []interface{}
			h := notifications.EventHandler(func(obj interface{}) { got = append(got, obj) })
			tc.fire(h)
			if d := cmp.Diff(tc.wantObj, got); d != "" {
				t.Errorf("unexpected enqueued objects (-want, +got): %s", d)
			}
		})
	}
}

// TestEventHandlerConcurrent exercises the stateless handler from several
// goroutines; run with -race.
func TestEventHandlerConcurrent(t *testing.T) {
	var enqueued atomic.Int64
	h := notifications.EventHandler(func(interface{}) { enqueued.Add(1) })
	running := taskRun("1", succeeded(corev1.ConditionUnknown, "Running", ""))
	done := taskRun("2", succeeded(corev1.ConditionTrue, "Succeeded", ""))

	const workers = 8
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.OnUpdate(done, done)    // resync: dropped
			h.OnUpdate(running, done) // transition: enqueued
		}()
	}
	wg.Wait()
	if got := enqueued.Load(); got != workers {
		t.Errorf("expected %d enqueued objects, got %d", workers, got)
	}
}
