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

package run

import (
	"context"
	"strings"
	"testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	cminformer "knative.dev/pkg/configmap/informer"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func initializeRunControllerAssets(t *testing.T, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx = ttesting.SetupFakeCloudClientContext(ctx, d.ExpectedCloudEventCount)
	ctx, cancel := context.WithCancel(ctx)
	test.EnsureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())
	ctl := NewController()(ctx, configMapWatcher)
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
	}

	return test.Assets{
		Logger:     logging.FromContext(ctx),
		Controller: ctl,
		Clients:    c,
		Informers:  informers,
		Recorder:   controller.GetEventRecorder(ctx).(*record.FakeRecorder),
		Ctx:        ctx,
	}, cancel
}

func getRunName(run v1alpha1.Run) string {
	return strings.Join([]string{run.Namespace, run.Name}, "/")
}

// getRunController returns an instance of the TaskRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getRunController(t *testing.T, d test.Data) (test.Assets, func()) {
	t.Helper()
	names.TestingSeed()
	return initializeRunControllerAssets(t, d)
}

// TestReconcile_CloudEvents runs reconcile with a cloud event sink configured
// to ensure that events are sent in different cases
func TestReconcile_CloudEvents(t *testing.T) {
	ignoreResourceVersion := cmpopts.IgnoreFields(v1alpha1.Run{}, "ObjectMeta.ResourceVersion")

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"default-cloud-events-sink": "http://synk:8080",
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
		name:            "Run with no condition",
		condition:       nil,
		wantCloudEvents: []string{`(?s)dev.tekton.event.run.started.v1.*test-run`},
	}, {
		name: "Run with unknown condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1beta1.TaskRunReasonRunning.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.run.running.v1.*test-run`},
	}, {
		name: "Run with finished true condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
			Reason: v1beta1.PipelineRunReasonSuccessful.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.run.successful.v1.*test-run`},
	}, {
		name: "Run with finished false condition",
		condition: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: v1beta1.PipelineRunReasonCancelled.String(),
		},
		wantCloudEvents: []string{`(?s)dev.tekton.event.run.failed.v1.*test-run`},
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			objectStatus := duckv1.Status{
				Conditions: []apis.Condition{},
			}
			runStatusFields := v1alpha1.RunStatusFields{}
			if tc.condition != nil {
				objectStatus.Conditions = append(objectStatus.Conditions, *tc.condition)
			}
			run := v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-run",
					Namespace: "foo",
				},
				Spec: v1alpha1.RunSpec{},
				Status: v1alpha1.RunStatus{
					Status:          objectStatus,
					RunStatusFields: runStatusFields,
				},
			}
			runs := []*v1alpha1.Run{&run}

			d := test.Data{
				Runs:                    runs,
				ConfigMaps:              cms,
				ExpectedCloudEventCount: len(tc.wantCloudEvents),
			}
			testAssets, cancel := getRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients

			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(run)); err != nil {
				t.Fatal("Didn't expect an error, but got one.")
			}

			for _, a := range clients.Kube.Actions() {
				aVerb := a.GetVerb()
				if aVerb != "get" && aVerb != "list" && aVerb != "watch" {
					t.Errorf("Expected only read actions to be logged in the kubeclient, got %s", aVerb)
				}
			}

			urun, err := clients.Pipeline.TektonV1alpha1().Runs(run.Namespace).Get(testAssets.Ctx, run.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated run: %v", err)
			}

			if d := cmp.Diff(&run, urun, ignoreResourceVersion); d != "" {
				t.Fatalf("run should not have changed, go %v instead", d)
			}

			ceClient := clients.CloudEvents.(cloudevent.FakeClient)
			ceClient.CheckCloudEventsUnordered(t, tc.name, tc.wantCloudEvents)

			// Try and reconcile again - expect no event
			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(run)); err != nil {
				t.Fatal("Didn't expect an error, but got one.")
			}
			ceClient.CheckCloudEventsUnordered(t, tc.name, []string{})
		})
	}
}

func TestReconcile_CloudEvents_Disabled(t *testing.T) {

	cmSinkOn := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"default-cloud-events-sink": "http://synk:8080",
		},
	}
	cmSinkOff := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			"default-cloud-events-sink": "",
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
	testcases := []struct {
		name string
		cms  []*corev1.ConfigMap
	}{{
		name: "Both disabled",
		cms:  []*corev1.ConfigMap{cmSinkOff, cmRunsOff},
	}, {
		name: "Sink Disabled",
		cms:  []*corev1.ConfigMap{cmSinkOff, cmRunsOn},
	}, {
		name: "Runs Disabled",
		cms:  []*corev1.ConfigMap{cmSinkOn, cmRunsOff},
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			objectStatus := duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: v1beta1.PipelineRunReasonCancelled.String(),
					},
				},
			}
			runStatusFields := v1alpha1.RunStatusFields{}
			run := v1alpha1.Run{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-run",
					Namespace: "foo",
				},
				Spec: v1alpha1.RunSpec{},
				Status: v1alpha1.RunStatus{
					Status:          objectStatus,
					RunStatusFields: runStatusFields,
				},
			}
			runs := []*v1alpha1.Run{&run}

			d := test.Data{
				Runs:       runs,
				ConfigMaps: tc.cms,
			}
			testAssets, cancel := getRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients

			if err := c.Reconciler.Reconcile(testAssets.Ctx, getRunName(run)); err != nil {
				t.Fatal("Didn't expect an error, but got one.")
			}
			if len(clients.Kube.Actions()) == 0 {
				t.Errorf("Expected actions to be logged in the kubeclient, got none")
			}

			urun, err := clients.Pipeline.TektonV1alpha1().Runs(run.Namespace).Get(testAssets.Ctx, run.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting updated run: %v", err)
			}

			if d := cmp.Diff(run.Status, urun.Status); d != "" {
				t.Fatalf("run should not have changed, go %v instead", d)
			}

			ceClient := clients.CloudEvents.(cloudevent.FakeClient)
			ceClient.CheckCloudEventsUnordered(t, tc.name, []string{})
		})
	}
}
