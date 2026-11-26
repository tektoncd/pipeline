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

package taskrun_test

import (
	"testing"

	bc "github.com/allegro/bigcache/v3"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications/taskrun"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// spareEventCapacity makes the fake CloudEvents client buffer events beyond the
// expected ones, so that duplicates are reported as "extra event received"
// instead of being silently rejected by the fake.
const spareEventCapacity = 10

func newTaskRunWithCondition(resourceVersion string, status corev1.ConditionStatus, reason string) *v1.TaskRun {
	return &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "test-taskrun",
			Namespace:       "foo",
			ResourceVersion: resourceVersion,
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: status,
				Reason: reason,
			}}},
		},
	}
}

// evictCache simulates the expiry or eviction of every cache entry, which is
// what happens to BigCache entries older than LifeWindow.
func evictCache(t *testing.T, cacheClient *bc.BigCache) {
	t.Helper()
	if err := cacheClient.Reset(); err != nil {
		t.Fatalf("failed to reset the events cache: %v", err)
	}
}

type resyncHarness struct {
	handler  cache.ResourceEventHandler
	ceClient cloudevent.FakeClient
	cache    *bc.BigCache
}

func newResyncHarness(t *testing.T, newHandler func(func(interface{})) cache.ResourceEventHandler) (*resyncHarness, func()) {
	t.Helper()
	d := test.Data{
		ConfigMaps:              []*corev1.ConfigMap{cmSinkOn, cmRunsOn},
		ExpectedCloudEventCount: spareEventCapacity,
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	r := taskrun.NewReconciler(ceClient, cacheClient)
	// Reconcile synchronously in place of the work queue, so that every
	// object the handler enqueues is reconciled exactly once.
	h := newHandler(func(obj interface{}) {
		if err := r.ReconcileKind(testAssets.Ctx, obj.(*v1.TaskRun)); err != nil {
			t.Errorf("didn't expect an error, but got one: %v", err)
		}
	})
	return &resyncHarness{handler: h, ceClient: ceClient.(cloudevent.FakeClient), cache: cacheClient}, cancel
}

// TestResync_DoesNotReemitAfterCacheEviction covers issue #10911: informer
// resyncs must not re-send CloudEvents for unchanged TaskRuns, even when their
// cache entries have expired.
func TestResync_DoesNotReemitAfterCacheEviction(t *testing.T) {
	runningEvent := `(?s)dev.tekton.event.taskrun.running.v1.*test-taskrun`
	testcases := []struct {
		name          string
		status        corev1.ConditionStatus
		reason        string
		terminalEvent string
	}{{
		name:          "successful",
		status:        corev1.ConditionTrue,
		reason:        v1.TaskRunReasonSuccessful.String(),
		terminalEvent: `(?s)dev.tekton.event.taskrun.successful.v1.*test-taskrun`,
	}, {
		name:          "failed",
		status:        corev1.ConditionFalse,
		reason:        v1.TaskRunReasonFailed.String(),
		terminalEvent: `(?s)dev.tekton.event.taskrun.failed.v1.*test-taskrun`,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			h, cancel := newResyncHarness(t, notifications.EventHandler)
			defer cancel()

			running := newTaskRunWithCondition("1", corev1.ConditionUnknown, v1.TaskRunReasonRunning.String())
			done := newTaskRunWithCondition("2", tc.status, tc.reason)

			h.handler.OnAdd(running, false)
			h.ceClient.CheckCloudEventsUnordered(t, "running", []string{runningEvent})

			// Resync of a running object after its cache entry expired.
			evictCache(t, h.cache)
			h.handler.OnUpdate(running, running)
			h.ceClient.CheckCloudEventsUnordered(t, "resync while running", []string{})

			// A message-only change while running is a legitimate update and
			// still produces a running event, as before.
			runningNewMessage := running.DeepCopy()
			runningNewMessage.ResourceVersion = "1a"
			runningNewMessage.Status.Conditions[0].Message = "progress"
			h.handler.OnUpdate(running, runningNewMessage)
			h.ceClient.CheckCloudEventsUnordered(t, "message change while running", []string{runningEvent})

			// A real transition is still delivered after eviction.
			h.handler.OnUpdate(running, done)
			h.ceClient.CheckCloudEventsUnordered(t, "completion", []string{tc.terminalEvent})

			// Repeated resyncs of the terminal object, each after eviction.
			for range 3 {
				evictCache(t, h.cache)
				h.handler.OnUpdate(done, done.DeepCopy())
			}
			h.ceClient.CheckCloudEventsUnordered(t, "resync after completion", []string{})

			// Metadata-only updates after eviction (labels, finalizers,
			// deletionTimestamp) do not re-send the terminal event either.
			doneTouched := done.DeepCopy()
			doneTouched.ResourceVersion = "3"
			doneTouched.Labels = map[string]string{"touched": "true"}
			doneTouched.Finalizers = []string{"example.com/finalizer"}
			doneDeleting := doneTouched.DeepCopy()
			doneDeleting.ResourceVersion = "4"
			doneDeleting.DeletionTimestamp = &metav1.Time{}
			h.handler.OnUpdate(done, doneTouched)
			h.handler.OnUpdate(doneTouched, doneDeleting)
			h.ceClient.CheckCloudEventsUnordered(t, "metadata-only updates", []string{})

			// An add after eviction (as for the initial list after a
			// controller restart) is not covered by this fix and still sends
			// the terminal event; reconciling it again is deduplicated by
			// the cache.
			h.handler.OnAdd(doneDeleting, false)
			h.handler.OnAdd(doneDeleting, false)
			h.ceClient.CheckCloudEventsUnordered(t, "add after eviction", []string{tc.terminalEvent})
		})
	}
}

// TestResync_NewlyCompletedTaskRun checks that a TaskRun first observed in a
// terminal state still produces its terminal event, and only once.
func TestResync_NewlyCompletedTaskRun(t *testing.T) {
	h, cancel := newResyncHarness(t, notifications.EventHandler)
	defer cancel()

	done := newTaskRunWithCondition("7", corev1.ConditionTrue, v1.TaskRunReasonSuccessful.String())
	h.handler.OnAdd(done, false)
	h.ceClient.CheckCloudEventsUnordered(t, "first observation", []string{
		`(?s)dev.tekton.event.taskrun.successful.v1.*test-taskrun`,
	})

	evictCache(t, h.cache)
	h.handler.OnUpdate(done, done)
	h.ceClient.CheckCloudEventsUnordered(t, "resync", []string{})
}
