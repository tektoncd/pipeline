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
	pipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/notifications/pipelinerun"
	ntesting "github.com/tektoncd/pipeline/pkg/reconciler/notifications/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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
	ctl := pipelinerun.NewController()(a.Ctx, configMapWatcher)
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
	ignoreResourceVersion := cmpopts.IgnoreFields(v1.PipelineRun{}, "ObjectMeta.ResourceVersion")

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetEventsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{"sink": "http://sink:8080"},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{"send-cloudevents-for-runs": "true"},
		},
	}

	pr := v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipelinerun",
			Namespace: "foo",
		},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1.PipelineRunReasonSuccessful.String(),
				}},
			},
		},
	}
	wantCloudEvents := []string{`(?s)dev.tekton.event.pipelinerun.successful.v1.*test-pipelinerun`}

	d := test.Data{
		PipelineRuns:            []*v1.PipelineRun{&pr},
		ConfigMaps:              cms,
		ExpectedCloudEventCount: len(wantCloudEvents),
	}
	testAssets, cancel := ntesting.InitializeTestAssets(t, &d)
	defer cancel()
	clients := testAssets.Clients

	testAssets = InitializeTestController(t, d, testAssets)
	c := testAssets.Controller

	if err := c.Reconciler.Reconcile(testAssets.Ctx, ntesting.GetTestResourceName(&pr)); err != nil {
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
		t.Fatalf("getting updated PipelineRun: %v", err)
	}
	if d := cmp.Diff(&pr, prAfter, ignoreResourceVersion); d != "" {
		t.Fatalf("PipelineRun should not have changed, got %v instead", d)
	}

	ceClient := clients.CloudEvents.(cloudevent.FakeClient)
	ceClient.CheckCloudEventsUnordered(t, "controller test", wantCloudEvents)

	// Try and reconcile again - expect no event (deduplication via cache)
	if err := c.Reconciler.Reconcile(testAssets.Ctx, ntesting.GetTestResourceName(&pr)); err != nil {
		t.Errorf("didn't expect an error, but got one: %v", err)
	}
	ceClient.CheckCloudEventsUnordered(t, "controller test duplicate", []string{})
}

func newPipelineRunForController(name string, status corev1.ConditionStatus, reason string) *v1.PipelineRun {
	return &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "foo"},
		Status: v1.PipelineRunStatus{
			Status: duckv1.Status{Conditions: []apis.Condition{{
				Type:   apis.ConditionSucceeded,
				Status: status,
				Reason: reason,
			}}},
		},
	}
}

// TestNewController_SkipsUpdatesWithoutConditionChange checks, through the
// informer and work queue of the real controller, that an update which
// leaves the Succeeded condition unchanged does not enqueue the PipelineRun again,
// while a condition change does (#10911).
func TestNewController_SkipsUpdatesWithoutConditionChange(t *testing.T) {
	testAssets, cancel := ntesting.InitializeTestAssets(t, &test.Data{})
	defer cancel()
	testAssets = InitializeTestController(t, test.Data{}, testAssets)
	ctx := testAssets.Ctx
	ntesting.RunInformer(t, ctx, testAssets.Clients.Pipeline, "pipelineruns", pipelineruninformer.Get(ctx).Informer())
	client := testAssets.Clients.Pipeline.TektonV1().PipelineRuns("foo")

	done := newPipelineRunForController("done", corev1.ConditionTrue, v1.PipelineRunReasonSuccessful.String())
	running := newPipelineRunForController("running", corev1.ConditionUnknown, v1.PipelineRunReasonRunning.String())
	for _, run := range []*v1.PipelineRun{done, running} {
		if _, err := client.Create(ctx, run, metav1.CreateOptions{}); err != nil {
			t.Fatalf("creating PipelineRun %s: %v", run.Name, err)
		}
	}
	gotAdds := sets.New(ntesting.NextQueueKey(t, testAssets.Controller).String(), ntesting.NextQueueKey(t, testAssets.Controller).String())
	wantAdds := sets.New("foo/done", "foo/running")
	if !gotAdds.Equal(wantAdds) {
		t.Fatalf("expected adds %v to be enqueued, got %v", sets.List(wantAdds), sets.List(gotAdds))
	}

	// Metadata-only update of the terminal run: must not be enqueued.
	doneLabelled := done.DeepCopy()
	doneLabelled.Labels = map[string]string{"example.com/touched": "true"}
	doneLabelled.Finalizers = []string{"example.com/finalizer"}
	if _, err := client.Update(ctx, doneLabelled, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("updating PipelineRun done: %v", err)
	}
	// Completion of the other run: must be enqueued. Informer notifications
	// are delivered in order, so once this key arrives the previous update
	// has already been handled.
	completed := running.DeepCopy()
	completed.Status.Conditions[0].Status = corev1.ConditionTrue
	completed.Status.Conditions[0].Reason = v1.PipelineRunReasonSuccessful.String()
	if _, err := client.UpdateStatus(ctx, completed, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("updating PipelineRun running: %v", err)
	}

	want := types.NamespacedName{Namespace: "foo", Name: "running"}
	if got := ntesting.NextQueueKey(t, testAssets.Controller); got != want {
		t.Fatalf("expected %v to be enqueued after its condition changed, got %v", want, got)
	}
	if n := testAssets.Controller.WorkQueue().Len(); n != 0 {
		t.Errorf("expected no other enqueued keys, got %d", n)
	}
}
