/*
Copyright 2019 The Tekton Authors

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

package customrun_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
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
	// _ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

// TestReconcileKind_CloudEvents runs reconcile with a cloud event sink configured
// to ensure that events are sent in different cases
func TestReconcileKind_CloudEvents(t *testing.T) {
	ignoreResourceVersion := cmpopts.IgnoreFields(v1beta1.CustomRun{}, "ObjectMeta.ResourceVersion")

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

	testcases := []struct {
		name            string
		condition       *apis.Condition
		wantCloudEvents []string
	}{{
		name:            "CustomRun with no condition",
		condition:       nil,
		wantCloudEvents: []string{`(?s)dev.tekton.event.customrun.started.v1.*test-customRun`},
	}, {
		name: "CustomRun with unknown condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1beta1.CustomRunReasonRunning.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.customrun.running.v1.*test-customRun`},
	}, {
		name: "CustomRun with finished true condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: v1beta1.CustomRunReasonSuccessful.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.customrun.successful.v1.*test-customRun`},
	}, {
		name: "CustomRun with finished false condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: v1beta1.CustomRunReasonCancelled.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.customrun.failed.v1.*test-customRun`},
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			objectStatus := duckv1.Status{
				Conditions: []apis.Condition{},
			}
			crStatusFields := v1beta1.CustomRunStatusFields{}
			if tc.condition != nil {
				objectStatus.Conditions = append(objectStatus.Conditions, *tc.condition)
			}
			customRun := v1beta1.CustomRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-customRun",
					Namespace: "foo",
				},
				Spec: v1beta1.CustomRunSpec{},
				Status: v1beta1.CustomRunStatus{
					Status:                objectStatus,
					CustomRunStatusFields: crStatusFields,
				},
			}
			customRuns := []*v1beta1.CustomRun{&customRun}

			d := test.Data{
				CustomRuns:              customRuns,
				ConfigMaps:              cms,
				ExpectedCloudEventCount: len(tc.wantCloudEvents),
			}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			clients := testAssets.Clients
			reconciler := &ntesting.FakeReconciler{}
			notifications.ReconcilerFromContext(testAssets.Ctx, reconciler)

			if err := notifications.ReconcileRuntimeObject(testAssets.Ctx, reconciler, &customRun); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}

			for _, a := range clients.Kube.Actions() {
				aVerb := a.GetVerb()
				if aVerb != "get" && aVerb != "list" && aVerb != "watch" {
					t.Errorf("Expected only read actions to be logged in the kubeclient, got %s", aVerb)
				}
			}

			crAfter, err := clients.Pipeline.TektonV1beta1().CustomRuns(customRun.Namespace).Get(testAssets.Ctx, customRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("getting updated customRun: %v", err)
			}

			if d := cmp.Diff(&customRun, crAfter, ignoreResourceVersion); d != "" {
				t.Errorf("CustomRun should not have changed, got %v instead", d)
			}

			ceClient := clients.CloudEvents.(cloudevent.FakeClient)
			ceClient.CheckCloudEventsUnordered(t, tc.name, tc.wantCloudEvents)

			// Try and reconcile again - expect no event
			if err := notifications.ReconcileRuntimeObject(testAssets.Ctx, reconciler, &customRun); err != nil {
				t.Errorf("didn't expect an error, but got one: %v", err)
			}
			ceClient.CheckCloudEventsUnordered(t, tc.name, []string{})
		})
	}
}

func TestReconcile_CloudEvents_Disabled(t *testing.T) {
	cmSinkOn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"sink": "http://synk:8080",
		},
	}
	cmSinkOff := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"sink": "",
		},
	}
	cmRunsOn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"send-cloudevents-for-runs": "true",
		},
	}
	cmRunsOff := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"send-cloudevents-for-runs": "false",
		},
	}
	for _, tc := range []struct {
		name string
		cms  []*corev1.ConfigMap
	}{{
		name: "Both disabled",
		cms:  []*corev1.ConfigMap{cmSinkOff, cmRunsOff},
	}, {
		name: "Sink Disabled",
		cms:  []*corev1.ConfigMap{cmSinkOff, cmRunsOn},
	}, {
		name: "CustomRuns Disabled",
		cms:  []*corev1.ConfigMap{cmSinkOn, cmRunsOff},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			objectStatus := duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: v1beta1.CustomRunReasonCancelled.String(),
					},
				},
			}
			crStatusFields := v1beta1.CustomRunStatusFields{}
			customRun := v1beta1.CustomRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-customRun",
					Namespace: "foo",
				},
				Spec: v1beta1.CustomRunSpec{},
				Status: v1beta1.CustomRunStatus{
					Status:                objectStatus,
					CustomRunStatusFields: crStatusFields,
				},
			}
			customRuns := []*v1beta1.CustomRun{&customRun}

			d := test.Data{
				CustomRuns: customRuns,
				ConfigMaps: tc.cms,
			}
			testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
			defer cancel()
			clients := testAssets.Clients
			reconciler := &ntesting.FakeReconciler{}
			notifications.ReconcilerFromContext(testAssets.Ctx, reconciler)

			if err := notifications.ReconcileRuntimeObject(testAssets.Ctx, reconciler, &customRun); err != nil {
				t.Fatalf("didn't expect an error, but got one: %v", err)
			}

			crAfter, err := clients.Pipeline.TektonV1beta1().CustomRuns(customRun.Namespace).Get(testAssets.Ctx, customRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated customRun: %v", err)
			}

			if d := cmp.Diff(customRun.Status, crAfter.Status); d != "" {
				t.Fatalf("CustomRun should not have changed, got %v instead", d)
			}

			ceClient := clients.CloudEvents.(cloudevent.FakeClient)
			ceClient.CheckCloudEventsUnordered(t, tc.name, []string{})
		})
	}
}
