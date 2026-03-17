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
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications/taskrun"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	cminformer "knative.dev/pkg/configmap/informer"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func InitializeTestController(t *testing.T, d test.Data, a test.Assets) test.Assets {
	t.Helper()
	configMapWatcher := cminformer.NewInformedWatcher(a.Clients.Kube, system.Namespace())
	ctl := taskrun.NewController()(a.Ctx, configMapWatcher)
	if err := configMapWatcher.Start(a.Ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}

	if la, ok := ctl.Reconciler.(pkgreconciler.LeaderAware); ok {
		la.Promote(pkgreconciler.UniversalBucket(), func(pkgreconciler.Bucket, types.NamespacedName) {})
	}
	a.Controller = ctl
	return a
}

// TestReconcileNewController tests that the controller can be created and reconciles correctly
func TestReconcileNewController(t *testing.T) {
	ignoreResourceVersion := cmpopts.IgnoreFields(v1.TaskRun{}, "ObjectMeta.ResourceVersion")

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{"sink": "http://sink:8080"},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{"send-cloudevents-for-runs": "true"},
		},
	}

	tr := v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun",
			Namespace: "foo",
		},
		Status: v1.TaskRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1.TaskRunReasonSuccessful.String(),
				}},
			},
		},
	}
	wantCloudEvents := []string{`(?s)dev.tekton.event.taskrun.successful.v1.*test-taskrun`}

	d := test.Data{
		TaskRuns:                []*v1.TaskRun{&tr},
		ConfigMaps:              cms,
		ExpectedCloudEventCount: len(wantCloudEvents),
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()
	clients := testAssets.Clients

	testAssets = InitializeTestController(t, d, testAssets)
	c := testAssets.Controller

	if err := c.Reconciler.Reconcile(testAssets.Ctx, ntesting.GetTestResourceName(&tr)); err != nil {
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
		t.Fatalf("getting updated TaskRun: %v", err)
	}
	if d := cmp.Diff(&tr, trAfter, ignoreResourceVersion); d != "" {
		t.Fatalf("TaskRun should not have changed, got %v instead", d)
	}

	ceClient := clients.CloudEvents.(cloudevent.FakeClient)
	ceClient.CheckCloudEventsUnordered(t, "controller test", wantCloudEvents)

	// Try and reconcile again - expect no event (deduplication via cache)
	if err := c.Reconciler.Reconcile(testAssets.Ctx, ntesting.GetTestResourceName(&tr)); err != nil {
		t.Errorf("didn't expect an error, but got one: %v", err)
	}
	ceClient.CheckCloudEventsUnordered(t, "controller test duplicate", []string{})
}
