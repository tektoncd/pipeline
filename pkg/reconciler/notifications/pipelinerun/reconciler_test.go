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

package pipelinerun_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications/pipelinerun"
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

// TestReconcileKind_CloudEvents tests cloud event emission for the PipelineRun notifications reconciler
func TestReconcileKind_CloudEvents(t *testing.T) {
	ignoreResourceVersion := cmpopts.IgnoreFields(v1.PipelineRun{}, "ObjectMeta.ResourceVersion")

	testcases := []struct {
		name            string
		condition       *apis.Condition
		wantCloudEvents []string
	}{{
		name:            "PipelineRun with no condition (queued)",
		condition:       nil,
		wantCloudEvents: []string{`(?s)dev.tekton.event.pipelinerun.queued.v1.*test-pipelinerun`},
	}, {
		name: "PipelineRun with unknown/started condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1.PipelineRunReasonStarted.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.pipelinerun.started.v1.*test-pipelinerun`},
	}, {
		name: "PipelineRun with unknown/running condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1.PipelineRunReasonRunning.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.pipelinerun.running.v1.*test-pipelinerun`},
	}, {
		name: "PipelineRun with successful condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: v1.PipelineRunReasonSuccessful.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.pipelinerun.successful.v1.*test-pipelinerun`},
	}, {
		name: "PipelineRun with failed condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: v1.PipelineRunReasonFailed.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.pipelinerun.failed.v1.*test-pipelinerun`},
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
			pr := v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipelinerun",
					Namespace: "foo",
				},
				Spec: v1.PipelineRunSpec{},
				Status: v1.PipelineRunStatus{
					Status: objectStatus,
				},
			}
			pipelineRuns := []*v1.PipelineRun{&pr}

			d := test.Data{
				PipelineRuns:            pipelineRuns,
				ConfigMaps:              cms,
				ExpectedCloudEventCount: len(tc.wantCloudEvents),
			}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			clients := testAssets.Clients

			ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
			reconciler := pipelinerun.NewReconciler(ceClient, cacheClient)

			if err := reconciler.ReconcileKind(testAssets.Ctx, &pr); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}

			for _, a := range clients.Kube.Actions() {
				aVerb := a.GetVerb()
				if aVerb != "get" && aVerb != "list" && aVerb != "watch" {
					t.Errorf("Expected only read actions to be logged in the kubeclient, got %s", aVerb)
				}
			}

			prAfter, err := clients.Pipeline.TektonV1().PipelineRuns(pr.Namespace).Get(testAssets.Ctx, pr.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("getting updated pipelineRun: %v", err)
			}
			if d := cmp.Diff(&pr, prAfter, ignoreResourceVersion); d != "" {
				t.Errorf("PipelineRun should not have changed, got %v instead", d)
			}

			ceClientFake := ceClient.(cloudevent.FakeClient)
			ceClientFake.CheckCloudEventsUnordered(t, tc.name, tc.wantCloudEvents)

			// Try and reconcile again - expect no event (deduplication via cache)
			if err := reconciler.ReconcileKind(testAssets.Ctx, &pr); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}
			ceClientFake.CheckCloudEventsUnordered(t, tc.name+" (duplicate)", []string{})
		})
	}
}

// TestReconcileKind_RunningAfterQueued tests that when a PipelineRun is first seen in Running
// state (no prior Started observation), only the running event is sent.
func TestReconcileKind_RunningAfterQueued(t *testing.T) {
	cms := []*corev1.ConfigMap{cmSinkOn, cmRunsOn}

	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipelinerun",
			Namespace: "foo",
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionUnknown,
					Reason: v1.PipelineRunReasonRunning.String(),
				}},
			},
		},
	}

	d := test.Data{
		PipelineRuns:            []*v1.PipelineRun{&pr},
		ConfigMaps:              cms,
		ExpectedCloudEventCount: 1,
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()

	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	reconciler := pipelinerun.NewReconciler(ceClient, cacheClient)

	if err := reconciler.ReconcileKind(testAssets.Ctx, &pr); err != nil {
		t.Errorf("didn't expect an error, but got one: %v", err)
	}

	ceClientFake := ceClient.(cloudevent.FakeClient)
	ceClientFake.CheckCloudEventsUnordered(t, "running event", []string{
		`(?s)dev.tekton.event.pipelinerun.running.v1.*test-pipelinerun`,
	})
}

// TestReconcileKind_NoSink tests that no events are sent when no sink is configured
func TestReconcileKind_NoSink(t *testing.T) {
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipelinerun", Namespace: "foo"},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
				Reason: v1.PipelineRunReasonSuccessful.String(),
			}}},
		},
	}

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{&pr},
		ConfigMaps:   []*corev1.ConfigMap{cmSinkOff, cmRunsOn},
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()

	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	reconciler := pipelinerun.NewReconciler(ceClient, cacheClient)

	if err := reconciler.ReconcileKind(testAssets.Ctx, &pr); err != nil {
		t.Fatalf("didn't expect an error, but got one: %v", err)
	}

	ceClientFake := ceClient.(cloudevent.FakeClient)
	ceClientFake.CheckCloudEventsUnordered(t, "no sink", []string{})
}

// TestReconcileKind_FeatureFlagOff tests that no events are sent when feature flag is off
func TestReconcileKind_FeatureFlagOff(t *testing.T) {
	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipelinerun", Namespace: "foo"},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
				Reason: v1.PipelineRunReasonSuccessful.String(),
			}}},
		},
	}

	d := test.Data{
		PipelineRuns: []*v1.PipelineRun{&pr},
		ConfigMaps:   []*corev1.ConfigMap{cmSinkOn, cmRunsOff},
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()

	ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
	reconciler := pipelinerun.NewReconciler(ceClient, cacheClient)

	if err := reconciler.ReconcileKind(testAssets.Ctx, &pr); err != nil {
		t.Fatalf("didn't expect an error, but got one: %v", err)
	}

	ceClientFake := ceClient.(cloudevent.FakeClient)
	ceClientFake.CheckCloudEventsUnordered(t, "feature flag off", []string{})
}
