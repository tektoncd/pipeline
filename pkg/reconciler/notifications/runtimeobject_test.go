/*
Copyright 2023 The Tekton Authors

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
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

// TestReconcileRunObject runs reconcile with a cloud event sink configured
// and ensures that the event logic is correctly invoked for all supported types
func TestReconcileRunObject(t *testing.T) {
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"sink": "http://synk:8080",
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"send-cloudevents-for-runs": "true",
			},
		},
	}

	successCondition := apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	}

	for _, tc := range []struct {
		name      string
		runObject v1beta1.RunObject
		wantCEs   []string
	}{{
		name: "v1 TaskRun",
		runObject: &v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr", Namespace: "foo"},
			Status: v1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1.TaskRunReasonSuccessful.String(),
				}}},
			},
		},
		wantCEs: []string{`(?s)dev.tekton.event.taskrun.successful.v1.*tr`},
	}, {
		name: "v1 PipelineRun",
		runObject: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pr", Namespace: "foo"},
			Status: v1.PipelineRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1.PipelineRunReasonSuccessful.String(),
				}}},
			},
		},
		wantCEs: []string{`(?s)dev.tekton.event.pipelinerun.successful.v1.*pr`},
	}, {
		name: "v1beta1 TaskRun",
		runObject: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{Name: "tr-beta", Namespace: "foo"},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.TaskRunReasonSuccessful.String(),
				}}},
			},
		},
		wantCEs: []string{`(?s)dev.tekton.event.taskrun.successful.v1.*tr-beta`},
	}, {
		name: "v1beta1 PipelineRun",
		runObject: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pr-beta", Namespace: "foo"},
			Status: v1beta1.PipelineRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.PipelineRunReasonSuccessful.String(),
				}}},
			},
		},
		wantCEs: []string{`(?s)dev.tekton.event.pipelinerun.successful.v1.*pr-beta`},
	}, {
		name: "v1beta1 CustomRun",
		runObject: &v1beta1.CustomRun{
			ObjectMeta: metav1.ObjectMeta{Name: "cr", Namespace: "foo"},
			Status: v1beta1.CustomRunStatus{
				Status: duckv1.Status{Conditions: []apis.Condition{successCondition}},
			},
		},
		wantCEs: []string{`(?s)dev.tekton.event.customrun.successful.v1.*cr`},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				ConfigMaps:              cms,
				ExpectedCloudEventCount: len(tc.wantCEs),
			}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			clients := testAssets.Clients

			ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
			reconciler := &ntesting.FakeReconciler{
				CloudEventClient: ceClient,
				CacheClient:      cacheClient,
			}

			if err := notifications.ReconcileRunObject(testAssets.Ctx, reconciler, tc.runObject); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}

			for _, a := range clients.Kube.Actions() {
				aVerb := a.GetVerb()
				if aVerb != "get" && aVerb != "list" && aVerb != "watch" {
					t.Errorf("Expected only read actions to be logged in the kubeclient, got %s", aVerb)
				}
			}

			ceClientFake := ceClient.(cloudevent.FakeClient)
			ceClientFake.CheckCloudEventsUnordered(t, tc.name, tc.wantCEs)
		})
	}
}

func TestReconcileRunObject_Disabled(t *testing.T) {
	cmSinkOff := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{"sink": ""},
	}
	cmRunsOn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{"send-cloudevents-for-runs": "true"},
	}

	for _, tc := range []struct {
		name string
		cms  []*corev1.ConfigMap
	}{{
		name: "No sink",
		cms:  []*corev1.ConfigMap{cmSinkOff, cmRunsOn},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			customRun := &v1beta1.CustomRun{
				ObjectMeta: metav1.ObjectMeta{Name: "cr", Namespace: "foo"},
				Status: v1beta1.CustomRunStatus{
					Status: duckv1.Status{Conditions: []apis.Condition{{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					}}},
				},
			}
			d := test.Data{ConfigMaps: tc.cms}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			ceClient, cacheClient := notifications.EventClientsFromContext(testAssets.Ctx)
			reconciler := &ntesting.FakeReconciler{
				CloudEventClient: ceClient,
				CacheClient:      cacheClient,
			}

			if err := notifications.ReconcileRunObject(testAssets.Ctx, reconciler, customRun); err != nil {
				t.Fatalf("didn't expect an error, but got one: %v", err)
			}

			ceClientFake := ceClient.(cloudevent.FakeClient)
			ceClientFake.CheckCloudEventsUnordered(t, tc.name, []string{})
		})
	}
}
