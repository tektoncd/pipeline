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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications/taskrun"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

var (
	cmSinkOn = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{"sink": "http://sink:8080"},
	}
	cmSinkOff = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{"sink": ""},
	}
	cmRunsOn = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{"send-cloudevents-for-runs": "true"},
	}
	cmRunsOff = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{"send-cloudevents-for-runs": "false"},
	}
)

// TestReconcileKind_CloudEvents tests cloud event emission for the TaskRun notifications reconciler
func TestReconcileKind_CloudEvents(t *testing.T) {
	ignoreResourceVersion := cmpopts.IgnoreFields(v1.TaskRun{}, "ObjectMeta.ResourceVersion")

	testcases := []struct {
		name            string
		condition       *apis.Condition
		wantCloudEvents []string
	}{{
		name:            "TaskRun with no condition (queued)",
		condition:       nil,
		wantCloudEvents: []string{`(?s)dev.tekton.event.taskrun.queued.v1.*test-taskrun`},
	}, {
		name: "TaskRun with unknown/started condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1.TaskRunReasonStarted.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.taskrun.started.v1.*test-taskrun`},
	}, {
		name: "TaskRun with unknown/running condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1.TaskRunReasonRunning.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.taskrun.running.v1.*test-taskrun`},
	}, {
		name: "TaskRun with successful condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: v1.TaskRunReasonSuccessful.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.taskrun.successful.v1.*test-taskrun`},
	}, {
		name: "TaskRun with failed condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: v1.TaskRunReasonFailed.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.taskrun.failed.v1.*test-taskrun`},
	}}

	cms := []*corev1.ConfigMap{cmSinkOn, cmRunsOn}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			objectStatus := duckv1.Status{
				Conditions: []apis.Condition{},
			}
			if tc.condition != nil {
				objectStatus.Conditions = append(objectStatus.Conditions, *tc.condition)
			}
			tr := v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-taskrun",
					Namespace: "foo",
				},
				Spec: v1.TaskRunSpec{},
				Status: v1.TaskRunStatus{
					Status: objectStatus,
				},
			}
			taskRuns := []*v1.TaskRun{&tr}

			d := test.Data{
				TaskRuns:                taskRuns,
				ConfigMaps:              cms,
				ExpectedCloudEventCount: len(tc.wantCloudEvents),
			}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			clients := testAssets.Clients

			ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
			reconciler := taskrun.NewReconciler(ceClient, cacheClient)

			if err := reconciler.ReconcileKind(testAssets.Ctx, &tr); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}

			for _, a := range clients.Kube.Actions() {
				aVerb := a.GetVerb()
				if aVerb != "get" && aVerb != "list" && aVerb != "watch" {
					t.Errorf("Expected only read actions to be logged in the kubeclient, got %s", aVerb)
				}
			}

			trAfter, err := clients.Pipeline.TektonV1().TaskRuns(tr.Namespace).Get(testAssets.Ctx, tr.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("getting updated taskRun: %v", err)
			}
			if d := cmp.Diff(&tr, trAfter, ignoreResourceVersion); d != "" {
				t.Errorf("TaskRun should not have changed, got %v instead", d)
			}

			ceClientFake := ceClient.(cloudevent.FakeClient)
			ceClientFake.CheckCloudEventsUnordered(t, tc.name, tc.wantCloudEvents)

			// Try and reconcile again - expect no event (deduplication via cache)
			if err := reconciler.ReconcileKind(testAssets.Ctx, &tr); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}
			ceClientFake.CheckCloudEventsUnordered(t, tc.name+" (duplicate)", []string{})
		})
	}
}

// TestReconcileKind_RunningAfterQueued tests that when a TaskRun is first seen in Running
// state (no prior Started observation), only the running event is sent.
func TestReconcileKind_RunningAfterQueued(t *testing.T) {
	cms := []*corev1.ConfigMap{cmSinkOn, cmRunsOn}

	tr := v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun",
			Namespace: "foo",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: v1.TaskRunReasonRunning.String(),
				}},
			},
		},
	}

	d := test.Data{
		TaskRuns:                []*v1.TaskRun{&tr},
		ConfigMaps:              cms,
		ExpectedCloudEventCount: 1,
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()

	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	reconciler := taskrun.NewReconciler(ceClient, cacheClient)

	if err := reconciler.ReconcileKind(testAssets.Ctx, &tr); err != nil {
		t.Errorf("didn't expect an error, but got one: %v", err)
	}

	ceClientFake := ceClient.(cloudevent.FakeClient)
	ceClientFake.CheckCloudEventsUnordered(t, "running event", []string{
		`(?s)dev.tekton.event.taskrun.running.v1.*test-taskrun`,
	})
}

// TestReconcileKind_NoSink tests that no events are sent when no sink is configured
func TestReconcileKind_NoSink(t *testing.T) {
	tr := v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-taskrun", Namespace: "foo"},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
				Reason: v1.TaskRunReasonSuccessful.String(),
			}}},
		},
	}

	d := test.Data{
		TaskRuns:   []*v1.TaskRun{&tr},
		ConfigMaps: []*corev1.ConfigMap{cmSinkOff, cmRunsOn},
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()

	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	reconciler := taskrun.NewReconciler(ceClient, cacheClient)

	if err := reconciler.ReconcileKind(testAssets.Ctx, &tr); err != nil {
		t.Fatalf("didn't expect an error, but got one: %v", err)
	}

	ceClientFake := ceClient.(cloudevent.FakeClient)
	ceClientFake.CheckCloudEventsUnordered(t, "no sink", []string{})
}

// TestReconcileKind_FeatureFlagOff tests that no events are sent when feature flag is off
func TestReconcileKind_FeatureFlagOff(t *testing.T) {
	tr := v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-taskrun", Namespace: "foo"},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
				Reason: v1.TaskRunReasonSuccessful.String(),
			}}},
		},
	}

	d := test.Data{
		TaskRuns:   []*v1.TaskRun{&tr},
		ConfigMaps: []*corev1.ConfigMap{cmSinkOn, cmRunsOff},
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()

	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	reconciler := taskrun.NewReconciler(ceClient, cacheClient)

	if err := reconciler.ReconcileKind(testAssets.Ctx, &tr); err != nil {
		t.Fatalf("didn't expect an error, but got one: %v", err)
	}

	ceClientFake := ceClient.(cloudevent.FakeClient)
	ceClientFake.CheckCloudEventsUnordered(t, "feature flag off", []string{})
}
