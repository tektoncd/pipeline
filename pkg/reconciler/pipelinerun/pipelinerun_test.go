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

package pipelinerun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	taskrunresources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	eventstest "github.com/tektoncd/pipeline/test/events"
	"github.com/tektoncd/pipeline/test/names"
	"gomodules.xyz/jsonpatch/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	cminformer "knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"

	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

var (
	ignoreLastTransitionTime = cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime.Inner.Time")
	images                   = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "override-with-nop:latest",
		GitImage:                 "override-with-git:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
		ShellImage:               "busybox",
		GsutilImage:              "gcr.io/google.com/cloudsdktool/cloud-sdk",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}

	ignoreResourceVersion    = cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion")
	trueb                    = true
	simpleHelloWorldTask     = &v1beta1.Task{ObjectMeta: baseObjectMeta("hello-world", "foo")}
	simpleSomeTask           = &v1beta1.Task{ObjectMeta: baseObjectMeta("some-task", "foo")}
	simpleHelloWorldPipeline = &v1beta1.Pipeline{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
			}},
		},
	}

	now = time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
)

const (
	apiFieldsFeatureFlag = "enable-api-fields"
)

type PipelineRunTest struct {
	test.Data  `json:"inline"`
	Test       *testing.T
	TestAssets test.Assets
	Cancel     func()
}

type testClock struct{}

func (testClock) Now() time.Time                  { return now }
func (testClock) Since(t time.Time) time.Duration { return now.Sub(t) }

func ensureConfigurationConfigMapsExist(d *test.Data) {
	var defaultsExists, featureFlagsExists, artifactBucketExists, artifactPVCExists, metricsExists bool
	for _, cm := range d.ConfigMaps {
		if cm.Name == config.GetDefaultsConfigName() {
			defaultsExists = true
		}
		if cm.Name == config.GetFeatureFlagsConfigName() {
			featureFlagsExists = true
		}
		if cm.Name == config.GetArtifactBucketConfigName() {
			artifactBucketExists = true
		}
		if cm.Name == config.GetArtifactPVCConfigName() {
			artifactPVCExists = true
		}
		if cm.Name == config.GetMetricsConfigName() {
			metricsExists = true
		}
	}
	if !defaultsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !featureFlagsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !artifactBucketExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetArtifactBucketConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !artifactPVCExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetArtifactPVCConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
	if !metricsExists {
		d.ConfigMaps = append(d.ConfigMaps, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetMetricsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		})
	}
}

// getPipelineRunController returns an instance of the PipelineRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getPipelineRunController(t *testing.T, d test.Data) (test.Assets, func()) {
	return initializePipelineRunControllerAssets(t, d, pipeline.Options{Images: images})
}

// initiailizePipelinerunControllerAssets is a shared helper for
// controller initialization.
func initializePipelineRunControllerAssets(t *testing.T, d test.Data, opts pipeline.Options) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	ensureConfigurationConfigMapsExist(&d)
	c, informers := test.SeedTestData(t, ctx, d)
	configMapWatcher := cminformer.NewInformedWatcher(c.Kube, system.Namespace())
	ctl := NewController(&opts, testClock{})(ctx, configMapWatcher)
	if la, ok := ctl.Reconciler.(reconciler.LeaderAware); ok {
		la.Promote(reconciler.UniversalBucket(), func(reconciler.Bucket, types.NamespacedName) {})
	}
	if err := configMapWatcher.Start(ctx.Done()); err != nil {
		t.Fatalf("error starting configmap watcher: %v", err)
	}
	return test.Assets{
		Logger:     logging.FromContext(ctx),
		Clients:    c,
		Controller: ctl,
		Informers:  informers,
		Recorder:   controller.GetEventRecorder(ctx).(*record.FakeRecorder),
		Ctx:        ctx,
	}, cancel
}

// conditionCheckFromTaskRun converts takes a pointer to a TaskRun and wraps it into a ConditionCheck
func conditionCheckFromTaskRun(tr *v1beta1.TaskRun) *v1beta1.ConditionCheck {
	cc := v1beta1.ConditionCheck(*tr)
	return &cc
}

// getTaskRunCreations will look through a set of actions to find all task run creation actions and return the set of
// them. It will fatal the test if none are found.
func getTaskRunCreations(t *testing.T, actions []ktesting.Action) []*v1beta1.TaskRun {
	t.Helper()
	outputs := []*v1beta1.TaskRun{}
	for _, a := range actions {
		if action, ok := a.(ktesting.CreateAction); ok {
			if output, ok := action.GetObject().(*v1beta1.TaskRun); ok {
				outputs = append(outputs, output)
			}
		}
	}

	if len(outputs) > 0 {
		return outputs
	}
	t.Log("actions", actions)
	t.Fatal("failed to find an action that created a taskrun")
	return nil
}

// getPipelineRunUpdates will look through a set of actions to find all PipelineRun creation actions and return the
// set of them. It will fatal the test if none are found.
func getPipelineRunUpdates(t *testing.T, actions []ktesting.Action) []*v1beta1.PipelineRun {
	t.Helper()
	outputs := []*v1beta1.PipelineRun{}
	for _, a := range actions {
		if action, ok := a.(ktesting.UpdateAction); ok {
			if output, ok := action.GetObject().(*v1beta1.PipelineRun); ok {
				outputs = append(outputs, output)
			}
		}
	}

	if len(outputs) > 0 {
		return outputs
	}
	t.Fatal("failed to find an action that updated a pipelinerun")
	return nil
}

func TestReconcile(t *testing.T) {
	// TestReconcile runs "Reconcile" on a PipelineRun with one Task that has not been started yet.
	// It verifies that the TaskRun is created, it checks the resulting API actions, status and events.
	names.TestingSeed()
	const pipelineRunName = "test-pipeline-run-success"
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(pipelineRunName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "test-pipeline",
			},
			ServiceAccountName: "test-sa",
			Resources: []v1beta1.PipelineResourceBinding{
				{
					Name: "git-repo",
					ResourceRef: &v1beta1.PipelineResourceRef{
						Name: "some-repo",
					},
				},
				{
					Name: "best-image",
					ResourceSpec: &resourcev1alpha1.PipelineResourceSpec{
						Type: resourcev1alpha1.PipelineResourceTypeImage,
						Params: []resourcev1alpha1.ResourceParam{{
							Name:  "url",
							Value: "gcr.io/sven",
						}},
					},
				},
			},
			Params: []v1beta1.Param{{
				Name:  "bar",
				Value: *v1beta1.NewArrayOrString("somethingmorefun"),
			}},
		},
	}}
	funParam := v1beta1.Param{
		Name:  "foo",
		Value: *v1beta1.NewArrayOrString("somethingfun"),
	}
	moreFunParam := v1beta1.Param{
		Name:  "bar",
		Value: *v1beta1.NewArrayOrString("$(params.bar)"),
	}
	templatedParam := v1beta1.Param{
		Name:  "templatedparam",
		Value: *v1beta1.NewArrayOrString("$(inputs.workspace.$(params.rev-param))"),
	}
	contextRunParam := v1beta1.Param{
		Name:  "contextRunParam",
		Value: *v1beta1.NewArrayOrString("$(context.pipelineRun.name)"),
	}
	contextPipelineParam := v1beta1.Param{
		Name:  "contextPipelineParam",
		Value: *v1beta1.NewArrayOrString("$(context.pipeline.name)"),
	}
	retriesParam := v1beta1.Param{
		Name:  "contextRetriesParam",
		Value: *v1beta1.NewArrayOrString("$(context.pipelineTask.retries)"),
	}
	const pipelineName = "test-pipeline"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta(pipelineName, "foo"),
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{
				{
					Name: "git-repo",
					Type: resourcev1alpha1.PipelineResourceTypeGit,
				},
				{
					Name: "best-image",
					Type: resourcev1alpha1.PipelineResourceTypeImage,
				},
			},
			Params: []v1beta1.ParamSpec{
				{
					Name:    "pipeline-param",
					Type:    v1beta1.ParamTypeString,
					Default: v1beta1.NewArrayOrString("somethingdifferent"),
				},
				{
					Name:    "rev-param",
					Type:    v1beta1.ParamTypeString,
					Default: v1beta1.NewArrayOrString("revision"),
				},
				{
					Name: "bar",
					Type: v1beta1.ParamTypeString,
				},
			},
			Tasks: []v1beta1.PipelineTask{
				{
					// unit-test-3 uses runAfter to indicate it should run last
					Name: "unit-test-3",
					TaskRef: &v1beta1.TaskRef{
						Name: "unit-test-task",
					},
					Params: []v1beta1.Param{funParam, moreFunParam, templatedParam, contextRunParam, contextPipelineParam, retriesParam},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "workspace",
							Resource: "git-repo",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{
							{
								Name:     "image-to-use",
								Resource: "best-image",
							},
							{
								Name:     "workspace",
								Resource: "git-repo",
							},
						},
					},
					RunAfter: []string{"unit-test-2"},
				},
				{
					// unit-test-1 can run right away because it has no dependencies
					Name: "unit-test-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "unit-test-task",
					},
					Params: []v1beta1.Param{funParam, moreFunParam, templatedParam, contextRunParam, contextPipelineParam, retriesParam},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "workspace",
							Resource: "git-repo",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{
							{
								Name:     "image-to-use",
								Resource: "best-image",
							},
							{
								Name:     "workspace",
								Resource: "git-repo",
							},
						},
					},
					Retries: 5,
				},
				{
					Name: "unit-test-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "unit-test-followup-task",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "workspace",
							Resource: "git-repo",
							From:     []string{"unit-test-1"},
						}},
					},
				},
				{
					Name: "unit-test-cluster-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "unit-test-cluster-task",
						Kind: v1beta1.ClusterTaskKind,
					},
					Params: []v1beta1.Param{funParam, moreFunParam, templatedParam, contextRunParam, contextPipelineParam},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "workspace",
							Resource: "git-repo",
						}},
						Outputs: []v1beta1.PipelineTaskOutputResource{
							{
								Name:     "image-to-use",
								Resource: "best-image",
							},
							{
								Name:     "workspace",
								Resource: "git-repo",
							},
						},
					},
				},
			},
		},
	}}
	ts := []*v1beta1.Task{
		{
			ObjectMeta: baseObjectMeta("unit-test-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{
					{
						Name: "foo",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "bar",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "templatedparam",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "contextRunParam",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "contextPipelineParam",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "contextRetriesParam",
						Type: v1beta1.ParamTypeString,
					},
				},
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "workspace",
							Type: v1beta1.PipelineResourceTypeGit,
						},
					}},
					Outputs: []v1beta1.TaskResource{
						{
							ResourceDeclaration: v1beta1.ResourceDeclaration{
								Name: "image-to-use",
								Type: v1beta1.PipelineResourceTypeImage,
							},
						},
						{
							ResourceDeclaration: v1beta1.ResourceDeclaration{
								Name: "workspace",
								Type: v1beta1.PipelineResourceTypeGit,
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: baseObjectMeta("unit-test-followup-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "workspace",
							Type: v1beta1.PipelineResourceTypeGit,
						},
					}},
				},
			},
		},
	}
	clusterTasks := []*v1beta1.ClusterTask{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "unit-test-cluster-task"},
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{
					{
						Name: "foo",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "bar",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "templatedparam",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "contextRunParam",
						Type: v1beta1.ParamTypeString,
					},
					{
						Name: "contextPipelineParam",
						Type: v1beta1.ParamTypeString,
					},
				},
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "workspace",
							Type: v1beta1.PipelineResourceTypeGit,
						},
					}},
					Outputs: []v1beta1.TaskResource{
						{
							ResourceDeclaration: v1beta1.ResourceDeclaration{
								Name: "image-to-use",
								Type: v1beta1.PipelineResourceTypeImage,
							},
						},
						{
							ResourceDeclaration: v1beta1.ResourceDeclaration{
								Name: "workspace",
								Type: v1beta1.PipelineResourceTypeGit,
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "unit-test-followup-task"},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "workspace",
							Type: v1beta1.PipelineResourceTypeGit,
						},
					}},
				},
			},
		},
	}
	rs := []*resourcev1alpha1.PipelineResource{{
		ObjectMeta: baseObjectMeta("some-repo", "foo"),
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "url",
				Value: "https://github.com/kristoff/reindeer",
			}},
		},
	}}

	// When PipelineResources are created in the cluster, Kubernetes will add a SelfLink. We
	// are using this to differentiate between Resources that we are referencing by Spec or by Ref
	// after we have resolved them.
	rs[0].SelfLink = "some/link"

	d := test.Data{
		PipelineRuns:      prs,
		Pipelines:         ps,
		Tasks:             ts,
		ClusterTasks:      clusterTasks,
		PipelineResources: rs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-success", wantEvents, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-success-unit-test-1", "foo", "test-pipeline-run-success", "test-pipeline", "unit-test-1", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "unit-test-task",
			},
			ServiceAccountName: "test-sa",
			Params: []v1beta1.Param{
				{
					Name:  "foo",
					Value: *v1beta1.NewArrayOrString("somethingfun"),
				},
				{
					Name:  "bar",
					Value: *v1beta1.NewArrayOrString("somethingmorefun"),
				},
				{
					Name:  "templatedparam",
					Value: *v1beta1.NewArrayOrString("$(inputs.workspace.revision)"),
				},
				{
					Name:  "contextRunParam",
					Value: *v1beta1.NewArrayOrString(pipelineRunName),
				},
				{
					Name:  "contextPipelineParam",
					Value: *v1beta1.NewArrayOrString(pipelineName),
				},
				{
					Name:  "contextRetriesParam",
					Value: *v1beta1.NewArrayOrString("5"),
				},
			},
			Resources: &v1beta1.TaskRunResources{
				Inputs: []v1beta1.TaskResourceBinding{{
					PipelineResourceBinding: v1beta1.PipelineResourceBinding{
						Name: "workspace",
						ResourceRef: &v1beta1.PipelineResourceRef{
							Name: "some-repo",
						},
					},
				}},
				Outputs: []v1beta1.TaskResourceBinding{
					{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "image-to-use",
							ResourceSpec: &resourcev1alpha1.PipelineResourceSpec{
								Type: resourcev1alpha1.PipelineResourceTypeImage,
								Params: []resourcev1alpha1.ResourceParam{{
									Name:  "url",
									Value: "gcr.io/sven",
								}},
							},
						},
						Paths: []string{"/pvc/unit-test-1/image-to-use"},
					},
					{
						PipelineResourceBinding: v1beta1.PipelineResourceBinding{
							Name: "workspace",
							ResourceRef: &v1beta1.PipelineResourceRef{
								Name: "some-repo",
							},
						},
						Paths: []string{"/pvc/unit-test-1/workspace"},
					},
				},
			},
			Timeout: &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, cmpopts.SortSlices(func(x, y v1beta1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
	// test taskrun is able to recreate correct pipeline-pvc-name
	if expectedTaskRun.GetPipelineRunPVCName() != "test-pipeline-run-success-pvc" {
		t.Errorf("expected to see TaskRun PVC name set to %q created but got %s", "test-pipeline-run-success-pvc", expectedTaskRun.GetPipelineRunPVCName())
	}

	// This PipelineRun is in progress now and the status should reflect that
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %v", condition)
	}
	if condition != nil && condition.Reason != v1beta1.PipelineRunReasonRunning.String() {
		t.Errorf("Expected reason %q but was %s", v1beta1.PipelineRunReasonRunning.String(), condition.Reason)
	}

	if len(reconciledRun.Status.TaskRuns) != 2 {
		t.Errorf("Expected PipelineRun status to include both TaskRun status items that can run immediately: %v", reconciledRun.Status.TaskRuns)
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-1"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-cluster-task"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}

	// A PVC should have been created to deal with output -> input linking
	ensurePVCCreated(prt.TestAssets.Ctx, t, clients, expectedTaskRun.GetPipelineRunPVCName(), "foo")
}

// TestReconcile_CustomTask runs "Reconcile" on a PipelineRun with one Custom
// Task reference that has not been run yet.  It verifies that the Run is
// created, it checks the resulting API actions, status and events.
func TestReconcile_CustomTask(t *testing.T) {
	names.TestingSeed()
	const pipelineRunName = "test-pipelinerun"
	const pipelineTaskName = "custom-task"
	const namespace = "namespace"
	tcs := []struct {
		name    string
		pr      *v1beta1.PipelineRun
		wantRun *v1alpha1.Run
	}{{
		name: "simple custom task with taskRef",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pipelineRunName,
				Namespace: namespace,
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    pipelineTaskName,
						Retries: 3,
						Params: []v1beta1.Param{{
							Name:  "param1",
							Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "value1"},
						}},
						TaskRef: &v1beta1.TaskRef{
							APIVersion: "example.dev/v0",
							Kind:       "Example",
						},
					}},
				},
			},
		},
		wantRun: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pipelinerun-custom-task",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "tekton.dev/v1beta1",
					Kind:               "PipelineRun",
					Name:               pipelineRunName,
					Controller:         &trueb,
					BlockOwnerDeletion: &trueb,
				}},
				Labels: map[string]string{
					"tekton.dev/pipeline":     pipelineRunName,
					"tekton.dev/pipelineRun":  pipelineRunName,
					"tekton.dev/pipelineTask": pipelineTaskName,
					pipeline.MemberOfLabelKey: v1beta1.PipelineTasks,
				},
				Annotations: map[string]string{},
			},
			Spec: v1alpha1.RunSpec{
				Retries: 3,
				Params: []v1beta1.Param{{
					Name:  "param1",
					Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "value1"},
				}},
				Timeout: &metav1.Duration{Duration: time.Hour},
				Ref: &v1beta1.TaskRef{
					APIVersion: "example.dev/v0",
					Kind:       "Example",
				},
				ServiceAccountName: "default",
			},
		},
	}, {
		name: "simple custom task with taskSpec",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pipelineRunName,
				Namespace: namespace,
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: pipelineTaskName,
						Params: []v1beta1.Param{{
							Name:  "param1",
							Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "value1"},
						}},
						TaskSpec: &v1beta1.EmbeddedTask{
							TypeMeta: runtime.TypeMeta{
								APIVersion: "example.dev/v0",
								Kind:       "Example",
							},
							Metadata: v1beta1.PipelineTaskMetadata{Labels: map[string]string{"test-label": "test"}},
							Spec: runtime.RawExtension{
								Raw: []byte(`{"field1":123,"field2":"value"}`),
							},
						},
					}},
				},
			},
		},
		wantRun: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pipelinerun-custom-task",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "tekton.dev/v1beta1",
					Kind:               "PipelineRun",
					Name:               pipelineRunName,
					Controller:         &trueb,
					BlockOwnerDeletion: &trueb,
				}},
				Labels: map[string]string{
					"tekton.dev/pipeline":     pipelineRunName,
					"tekton.dev/pipelineRun":  pipelineRunName,
					"tekton.dev/pipelineTask": pipelineTaskName,
					pipeline.MemberOfLabelKey: v1beta1.PipelineTasks,
				},
				Annotations: map[string]string{},
			},
			Spec: v1alpha1.RunSpec{
				Params: []v1beta1.Param{{
					Name:  "param1",
					Value: v1beta1.ArrayOrString{Type: v1beta1.ParamTypeString, StringVal: "value1"},
				}},
				Spec: &v1alpha1.EmbeddedRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "example.dev/v0",
						Kind:       "Example",
					},
					Metadata: v1beta1.PipelineTaskMetadata{Labels: map[string]string{"test-label": "test"}},
					Spec: runtime.RawExtension{
						Raw: []byte(`{"field1":123,"field2":"value"}`),
					},
				},
				ServiceAccountName: "default",
				Timeout:            &metav1.Duration{Duration: time.Hour},
			},
		},
	}, {
		name: "custom task with workspace",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pipelineRunName,
				Namespace: namespace,
			},
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{Name: "pipelinews"}},
					Tasks: []v1beta1.PipelineTask{{
						Name: pipelineTaskName,
						TaskRef: &v1beta1.TaskRef{
							APIVersion: "example.dev/v0",
							Kind:       "Example",
						},
						Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
							Name:      "taskws",
							Workspace: "pipelinews",
							SubPath:   "bar",
						}},
					}},
				},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:    "pipelinews",
					SubPath: "foo",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myclaim",
						ReadOnly:  false,
					}},
				},
			},
		},
		wantRun: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pipelinerun-custom-task",
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "tekton.dev/v1beta1",
					Kind:               "PipelineRun",
					Name:               pipelineRunName,
					Controller:         &trueb,
					BlockOwnerDeletion: &trueb,
				}},
				Labels: map[string]string{
					"tekton.dev/pipeline":     pipelineRunName,
					"tekton.dev/pipelineRun":  pipelineRunName,
					"tekton.dev/pipelineTask": pipelineTaskName,
					pipeline.MemberOfLabelKey: v1beta1.PipelineTasks,
				},
				Annotations: map[string]string{
					"pipeline.tekton.dev/affinity-assistant": getAffinityAssistantName("pipelinews", pipelineRunName),
				},
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "example.dev/v0",
					Kind:       "Example",
				},
				ServiceAccountName: "default",
				Timeout:            &metav1.Duration{Duration: time.Hour},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:    "taskws",
					SubPath: "foo/bar",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myclaim",
						ReadOnly:  false,
					}},
				},
			},
		},
	}}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"enable-custom-tasks": "true",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {

			d := test.Data{
				PipelineRuns: []*v1beta1.PipelineRun{tc.pr},
				ConfigMaps:   cms,
			}
			prt := newPipelineRunTest(d, t)
			defer prt.Cancel()

			wantEvents := []string{
				"Normal Started",
				"Normal Running Tasks Completed: 0",
			}
			reconciledRun, clients := prt.reconcileRun(namespace, pipelineRunName, wantEvents, false)

			actions := clients.Pipeline.Actions()
			if len(actions) < 2 {
				t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
			}

			// Check that the expected Run was created.
			actual := actions[0].(ktesting.CreateAction).GetObject()
			if d := cmp.Diff(tc.wantRun, actual); d != "" {
				t.Errorf("expected to see Run created: %s", diff.PrintWantGot(d))
			}

			// This PipelineRun is in progress now and the status should reflect that
			condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionUnknown {
				t.Errorf("Expected PipelineRun status to be in progress, but was %v", condition)
			}
			if condition != nil && condition.Reason != v1beta1.PipelineRunReasonRunning.String() {
				t.Errorf("Expected reason %q but was %s", v1beta1.PipelineRunReasonRunning.String(), condition.Reason)
			}

			if len(reconciledRun.Status.Runs) != 1 {
				t.Errorf("Expected PipelineRun status to include one Run status, got %d", len(reconciledRun.Status.Runs))
			}
			if _, exists := reconciledRun.Status.Runs[tc.wantRun.Name]; !exists {
				t.Errorf("Expected PipelineRun status to include Run status but was %v", reconciledRun.Status.Runs)
			}
		})
	}
}

func TestReconcile_PipelineSpecTaskSpec(t *testing.T) {
	// TestReconcile_PipelineSpecTaskSpec runs "Reconcile" on a PipelineRun that has an embedded PipelineSpec that has an embedded TaskSpec.
	// It verifies that a TaskRun is created, it checks the resulting API actions, status and events.
	names.TestingSeed()

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-success", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{
				Name: "test-pipeline",
			},
		},
	}}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "unit-test-task-spec",
				TaskSpec: &v1beta1.EmbeddedTask{
					TaskSpec: v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{Container: corev1.Container{
							Name:  "mystep",
							Image: "myimage"}}},
					},
				},
			}},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-success", wantEvents, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-success-unit-test-task-spec", "foo", "test-pipeline-run-success", "test-pipeline", "unit-test-task-spec", false),
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Name:  "mystep",
						Image: "myimage",
					},
				}},
			},
			ServiceAccountName: config.DefaultServiceAccountValue,
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, cmpopts.SortSlices(func(x, y v1beta1.TaskSpec) bool { return len(x.Steps) == len(y.Steps) })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// test taskrun is able to recreate correct pipeline-pvc-name
	if expectedTaskRun.GetPipelineRunPVCName() != "test-pipeline-run-success-pvc" {
		t.Errorf("expected to see TaskRun PVC name set to %q created but got %s", "test-pipeline-run-success-pvc", expectedTaskRun.GetPipelineRunPVCName())
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to include both TaskRun status items that can run immediately: %v", reconciledRun.Status.TaskRuns)
	}

	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-task-spec"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}
}

func getInvalidPipelineRun(prName, pName string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: baseObjectMeta(prName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: pName},
		},
	}
}

// TestReconcile_InvalidPipelineRuns runs "Reconcile" on several PipelineRuns that are invalid in different ways.
// It verifies that reconcile fails, how it fails and which events are triggered.
func TestReconcile_InvalidPipelineRuns(t *testing.T) {
	ts := []*v1beta1.Task{
		{
			ObjectMeta: baseObjectMeta("a-task-that-exists", "foo"),
		},
		{
			ObjectMeta: baseObjectMeta("a-task-that-needs-params", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "some-param",
					Type: v1beta1.ParamTypeString,
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-task-that-needs-array-params", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "some-param",
					Type: v1beta1.ParamTypeArray,
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-task-that-needs-a-resource", "foo"),
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name: "workspace",
							Type: resourcev1alpha1.PipelineResourceTypeGit,
						},
					}},
				},
			},
		},
	}
	ps := []*v1beta1.Pipeline{
		{
			ObjectMeta: baseObjectMeta("pipeline-missing-tasks", "foo"),
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "myspecialtask",
					TaskRef: &v1beta1.TaskRef{Name: "sometask"},
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-pipeline-without-params", "foo"),
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "some-task",
					TaskRef: &v1beta1.TaskRef{Name: "a-task-that-needs-params"},
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-fine-pipeline", "foo"),
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "some-task",
					TaskRef: &v1beta1.TaskRef{Name: "a-task-that-exists"},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "needed-resource",
							Resource: "a-resource",
						}},
					},
				}},
				Resources: []v1beta1.PipelineDeclaredResource{{
					Name: "a-resource",
					Type: resourcev1alpha1.PipelineResourceTypeGit,
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-pipeline-that-should-be-caught-by-admission-control", "foo"),
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "some-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "a-task-that-exists",
					},
					Resources: &v1beta1.PipelineTaskResources{
						Inputs: []v1beta1.PipelineTaskInputResource{{
							Name:     "needed-resource",
							Resource: "a-resource",
						}},
					},
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-pipeline-with-array-params", "foo"),
			Spec: v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "some-param",
					Type: v1beta1.ParamTypeArray,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name: "some-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "a-task-that-needs-array-params",
					},
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("a-pipeline-with-missing-conditions", "foo"),
			Spec: v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "some-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "a-task-that-exists",
					},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "condition-does-not-exist",
					}},
				}},
			},
		},
	}

	for _, tc := range []struct {
		name               string
		pipelineRun        *v1beta1.PipelineRun
		reason             string
		hasNoDefaultLabels bool
		permanentError     bool
		wantEvents         []string
	}{{
		name:               "invalid-pipeline-shd-be-stop-reconciling",
		pipelineRun:        getInvalidPipelineRun("invalid-pipeline", "pipeline-not-exist"),
		reason:             ReasonCouldntGetPipeline,
		hasNoDefaultLabels: true,
		permanentError:     true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Error retrieving pipeline for pipelinerun",
		},
	}, {
		name:           "invalid-pipeline-run-missing-tasks-shd-stop-reconciling",
		pipelineRun:    getInvalidPipelineRun("pipelinerun-missing-tasks", "pipeline-missing-tasks"),
		reason:         ReasonCouldntGetTask,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Pipeline foo/pipeline-missing-tasks can't be Run",
		},
	}, {
		name:           "invalid-pipeline-run-params-dont-exist-shd-stop-reconciling",
		pipelineRun:    getInvalidPipelineRun("pipeline-params-dont-exist", "a-pipeline-without-params"),
		reason:         ReasonFailedValidation,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed invalid input params for task a-task-that-needs-params: missing values",
		},
	}, {
		name:           "invalid-pipeline-run-resources-not-bound-shd-stop-reconciling",
		pipelineRun:    getInvalidPipelineRun("pipeline-resources-not-bound", "a-fine-pipeline"),
		reason:         ReasonInvalidBindings,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-resources-not-bound doesn't bind Pipeline",
		},
	}, {
		name: "invalid-pipeline-run-missing-resource-shd-stop-reconciling",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("pipeline-resources-dont-exist", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "a-fine-pipeline"},
				Resources: []v1beta1.PipelineResourceBinding{{
					Name:        "a-resource",
					ResourceRef: &v1beta1.PipelineResourceRef{Name: "missing-resource"},
				}},
			},
		},
		reason:         ReasonCouldntGetResource,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-resources-dont-exist can't be Run; it tries to bind Resources",
		},
	}, {
		name:           "invalid-pipeline-missing-declared-resource-shd-stop-reconciling",
		pipelineRun:    getInvalidPipelineRun("pipeline-resources-not-declared", "a-pipeline-that-should-be-caught-by-admission-control"),
		reason:         ReasonFailedValidation,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Pipeline foo/a-pipeline-that-should-be-caught-by-admission-control can't be Run; it has an invalid spec",
		},
	}, {
		name: "invalid-pipeline-mismatching-parameter-types",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("pipeline-mismatching-param-type", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: "a-pipeline-with-array-params"},
				Params: []v1beta1.Param{{
					Name:  "some-param",
					Value: *v1beta1.NewArrayOrString("stringval"),
				}},
			},
		},
		reason:         ReasonParameterTypeMismatch,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-mismatching-param-type parameters have mismatching types",
		},
	}, {
		name:           "invalid-pipeline-missing-conditions-shd-stop-reconciling",
		pipelineRun:    getInvalidPipelineRun("pipeline-conditions-missing", "a-pipeline-with-missing-conditions"),
		reason:         ReasonCouldntGetCondition,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-conditions-missing can't be Run; it contains Conditions",
		},
	}, {
		name: "invalid-embedded-pipeline-resources-bot-bound-shd-stop-reconciling",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("embedded-pipeline-resources-not-bound", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "some-task",
						TaskRef: &v1beta1.TaskRef{Name: "a-task-that-needs-a-resource"},
					}},
					Resources: []v1beta1.PipelineDeclaredResource{{
						Name: "workspace",
						Type: resourcev1alpha1.PipelineResourceTypeGit,
					}},
				},
			},
		},
		reason:         ReasonInvalidBindings,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/embedded-pipeline-resources-not-bound doesn't bind Pipeline",
		},
	}, {
		name: "invalid-embedded-pipeline-bad-name-shd-stop-reconciling",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("embedded-pipeline-invalid", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "bad-t@$k",
						TaskRef: &v1beta1.TaskRef{Name: "b@d-t@$k"},
					}},
				},
			},
		},
		reason:         ReasonFailedValidation,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed Pipeline foo/embedded-pipeline-invalid can't be Run; it has an invalid spec",
		},
	}, {
		name: "invalid-embedded-pipeline-mismatching-parameter-types",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("embedded-pipeline-mismatching-param-type", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "some-param",
						Type: v1beta1.ParamTypeArray,
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name:    "some-task",
						TaskRef: &v1beta1.TaskRef{Name: "a-task-that-needs-array-params"},
					}},
				},
				Params: []v1beta1.Param{{
					Name:  "some-param",
					Value: *v1beta1.NewArrayOrString("stringval"),
				}},
			},
		},
		reason:         ReasonParameterTypeMismatch,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/embedded-pipeline-mismatching-param-type parameters have mismatching types",
		},
	}, {
		name: "invalid-pipeline-run-missing-params-shd-stop-reconciling",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("pipelinerun-missing-params", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "some-param",
						Type: v1beta1.ParamTypeString,
					}},
					Tasks: []v1beta1.PipelineTask{{
						Name:    "some-task",
						TaskRef: &v1beta1.TaskRef{Name: "a-task-that-needs-params"},
					}},
				},
			},
		},
		reason:         ReasonParameterMissing,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo parameters is missing some parameters required by Pipeline pipelinerun-missing-params",
		},
	}, {
		name: "invalid-pipeline-with-invalid-dag-graph",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("pipeline-invalid-dag-graph", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:     "dag-task-1",
						TaskRef:  &v1beta1.TaskRef{Name: "dag-task-1"},
						RunAfter: []string{"dag-task-1"},
					}},
				},
			},
		},
		reason:         ReasonInvalidGraph,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo/pipeline-invalid-dag-graph's Pipeline DAG is invalid",
		},
	}, {
		name: "invalid-pipeline-with-invalid-final-tasks-graph",
		pipelineRun: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta("pipeline-invalid-final-graph", "foo"),
			Spec: v1beta1.PipelineRunSpec{
				PipelineSpec: &v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "dag-task-1",
						TaskRef: &v1beta1.TaskRef{Name: "taskName"},
					}},
					Finally: []v1beta1.PipelineTask{
						{
							Name:    "final-task-1",
							TaskRef: &v1beta1.TaskRef{Name: "taskName"},
						},
						{
							Name:    "final-task-1",
							TaskRef: &v1beta1.TaskRef{Name: "taskName"},
						},
					},
				},
			},
		},
		reason:         ReasonInvalidGraph,
		permanentError: true,
		wantEvents: []string{
			"Normal Started",
			"Warning Failed PipelineRun foo's Pipeline DAG is invalid for finally clause",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			d := test.Data{
				PipelineRuns: []*v1beta1.PipelineRun{tc.pipelineRun},
				Pipelines:    ps,
				Tasks:        ts,
			}
			prt := newPipelineRunTest(d, t)
			defer prt.Cancel()

			wantEvents := append(tc.wantEvents, "Warning InternalError 1 error occurred") //nolint
			reconciledRun, _ := prt.reconcileRun("foo", tc.pipelineRun.Name, wantEvents, tc.permanentError)

			if reconciledRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
			}

			// Since the PipelineRun is invalid, the status should say it has failed
			condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected status to be failed on invalid PipelineRun but was: %v", condition)
			}
			if condition != nil && condition.Reason != tc.reason {
				t.Errorf("Expected failure to be because of reason %q but was %s", tc.reason, condition.Reason)
			}
			if !tc.hasNoDefaultLabels {
				expectedPipelineLabel := reconciledRun.Name
				// Embedded pipelines use the pipelinerun name
				if reconciledRun.Spec.PipelineRef != nil {
					expectedPipelineLabel = reconciledRun.Spec.PipelineRef.Name
				}
				expectedLabels := map[string]string{pipeline.PipelineLabelKey: expectedPipelineLabel}
				if len(reconciledRun.ObjectMeta.Labels) != len(expectedLabels) {
					t.Errorf("Expected labels : %v, got %v", expectedLabels, reconciledRun.ObjectMeta.Labels)
				}
				for k, ev := range expectedLabels {
					if v, ok := reconciledRun.ObjectMeta.Labels[k]; ok {
						if ev != v {
							t.Errorf("Expected labels %s=%s, but was %s", k, ev, v)
						}
					} else {
						t.Errorf("Expected labels %s=%v, but was not present", k, ev)
					}
				}
			}
		})
	}
}

func TestReconcile_InvalidPipelineRunNames(t *testing.T) {
	// TestReconcile_InvalidPipelineRunNames runs "Reconcile" on several PipelineRuns that have invalid names.
	// It verifies that reconcile fails, how it fails and which events are triggered.
	// Note that the code tested here is part of the genreconciler.
	invalidNames := []string{
		"foo/test-pipeline-run-doesnot-exist",
		"test/invalidformat/t",
	}
	tcs := []struct {
		name        string
		pipelineRun string
	}{
		{
			name:        "invalid-pipeline-run-shd-stop-reconciling",
			pipelineRun: invalidNames[0],
		}, {
			name:        "invalid-pipeline-run-name-shd-stop-reconciling",
			pipelineRun: invalidNames[1],
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getPipelineRunController(t, test.Data{})
			defer cancel()
			c := testAssets.Controller

			err := c.Reconciler.Reconcile(testAssets.Ctx, tc.pipelineRun)
			// No reason to keep reconciling something that doesnt or can't exist
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid PipelineRun but saw %q", err)
			}
		})
	}
}

func TestUpdateTaskRunsState(t *testing.T) {
	// TestUpdateTaskRunsState runs "getTaskRunsStatus" and verifies how it updates a PipelineRun status
	// from a TaskRun associated to the PipelineRun
	pr := &v1beta1.PipelineRun{
		ObjectMeta: baseObjectMeta("test-pipeline-run", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
		},
	}
	pipelineTask := v1beta1.PipelineTask{
		Name: "unit-test-1",
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
		TaskRef: &v1beta1.TaskRef{Name: "unit-test-task"},
	}
	task := &v1beta1.Task{
		ObjectMeta: baseObjectMeta("unit-test-task", "foo"),
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "workspace",
						Type: resourcev1alpha1.PipelineResourceTypeGit,
					},
				}},
			},
		},
	}
	taskrun := &v1beta1.TaskRun{
		ObjectMeta: baseObjectMeta("test-pipeline-run-success-unit-test-1", "foo"),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "unit-test-task"},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type: apis.ConditionSucceeded,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				Steps: []v1beta1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(0)},
					},
				}},
			},
		},
	}

	expectedTaskRunsStatus := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	expectedTaskRunsStatus["test-pipeline-run-success-unit-test-1"] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "unit-test-1",
		Status: &v1beta1.TaskRunStatus{
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				Steps: []v1beta1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
					},
				}}},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
	}
	expectedPipelineRunStatus := v1beta1.PipelineRunStatus{
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: expectedTaskRunsStatus,
		},
	}

	state := resources.PipelineRunState{{
		PipelineTask: &pipelineTask,
		TaskRunName:  "test-pipeline-run-success-unit-test-1",
		TaskRun:      taskrun,
		ResolvedTaskResources: &taskrunresources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}
	pr.Status.InitializeConditions(testClock{})
	status := state.GetTaskRunsStatus(pr)
	if d := cmp.Diff(expectedPipelineRunStatus.TaskRuns, status); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", diff.PrintWantGot(d))
	}

}

func TestUpdateRunsState(t *testing.T) {
	// TestUpdateRunsState runs "getRunsStatus" and verifies how it updates a PipelineRun status
	// from a Run associated to the PipelineRun
	pr := &v1beta1.PipelineRun{
		ObjectMeta: baseObjectMeta("test-pipeline-run", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
		},
	}
	pipelineTask := v1beta1.PipelineTask{
		Name: "unit-test-1",
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
		TaskRef: &v1beta1.TaskRef{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
			Name:       "unit-test-run",
		},
	}
	run := v1alpha1.Run{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "example.dev/v0",
			Kind:       "Example",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-run",
			Namespace: "foo",
		},
		Spec: v1alpha1.RunSpec{},
		Status: v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	}
	expectedRunsStatus := make(map[string]*v1beta1.PipelineRunRunStatus)
	expectedRunsStatus["test-pipeline-run-success-unit-test-1"] = &v1beta1.PipelineRunRunStatus{
		PipelineTaskName: "unit-test-1",
		Status: &v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		WhenExpressions: []v1beta1.WhenExpression{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"foo", "bar"},
		}},
	}
	expectedPipelineRunStatus := v1beta1.PipelineRunStatus{
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			Runs: expectedRunsStatus,
		},
	}

	state := resources.PipelineRunState{{
		PipelineTask: &pipelineTask,
		CustomTask:   true,
		RunName:      "test-pipeline-run-success-unit-test-1",
		Run:          &run,
	}}
	pr.Status.InitializeConditions(testClock{})
	status := state.GetRunsStatus(pr)
	if d := cmp.Diff(expectedPipelineRunStatus.Runs, status); d != "" {
		t.Fatalf("Expected PipelineRun status to match Run(s) status, but got a mismatch: %s", diff.PrintWantGot(d))
	}

}

func TestUpdateTaskRunStateWithConditionChecks(t *testing.T) {
	// TestUpdateTaskRunsState runs "getTaskRunsStatus" and verifies how it updates a PipelineRun status
	// from several different TaskRun with Conditions associated to the PipelineRun
	taskrunName := "task-run"
	successConditionCheckName := "success-condition"
	failingConditionCheckName := "fail-condition"

	successCondition := &v1alpha1.Condition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cond-1",
			Namespace: "foo",
		},
	}
	failingCondition := &v1alpha1.Condition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cond-2",
			Namespace: "foo",
		},
	}

	pipelineTask := v1beta1.PipelineTask{
		TaskRef: &v1beta1.TaskRef{Name: "unit-test-task"},
		Conditions: []v1beta1.PipelineTaskCondition{{
			ConditionRef: successCondition.Name,
		}, {
			ConditionRef: failingCondition.Name,
		}},
	}

	successConditionCheck := conditionCheckFromTaskRun(&v1beta1.TaskRun{
		ObjectMeta: baseObjectMeta(successConditionCheckName, "foo"),
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				Steps: []v1beta1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(0)},
					},
				}},
			},
		},
	})
	failingConditionCheck := conditionCheckFromTaskRun(&v1beta1.TaskRun{
		ObjectMeta: baseObjectMeta(failingConditionCheckName, "foo"),
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				Steps: []v1beta1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: int32(127)},
					},
				}},
			},
		},
	})

	successrcc := resources.ResolvedConditionCheck{
		ConditionRegisterName: successCondition.Name + "-0",
		ConditionCheckName:    successConditionCheckName,
		Condition:             successCondition,
		ConditionCheck:        successConditionCheck,
	}
	failingrcc := resources.ResolvedConditionCheck{
		ConditionRegisterName: failingCondition.Name + "-0",
		ConditionCheckName:    failingConditionCheckName,
		Condition:             failingCondition,
		ConditionCheck:        failingConditionCheck,
	}

	successConditionCheckStatus := &v1beta1.PipelineRunConditionCheckStatus{
		ConditionName: successrcc.ConditionRegisterName,
		Status: &v1beta1.ConditionCheckStatus{
			ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
				Check: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				},
			},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}},
			},
		},
	}
	failingConditionCheckStatus := &v1beta1.PipelineRunConditionCheckStatus{
		ConditionName: failingrcc.ConditionRegisterName,
		Status: &v1beta1.ConditionCheckStatus{
			ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
				Check: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 127},
				},
			},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}},
			},
		},
	}

	failedTaskRunStatus := v1beta1.TaskRunStatus{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{{
				Type:    apis.ConditionSucceeded,
				Status:  corev1.ConditionFalse,
				Reason:  resources.ReasonConditionCheckFailed,
				Message: fmt.Sprintf("ConditionChecks failed for Task %s in PipelineRun %s", taskrunName, "test-pipeline-run"),
			}},
		},
	}

	tcs := []struct {
		name           string
		rcc            resources.TaskConditionCheckState
		expectedStatus v1beta1.PipelineRunTaskRunStatus
	}{{
		name: "success-condition-checks",
		rcc:  resources.TaskConditionCheckState{&successrcc},
		expectedStatus: v1beta1.PipelineRunTaskRunStatus{
			ConditionChecks: map[string]*v1beta1.PipelineRunConditionCheckStatus{
				successrcc.ConditionCheck.Name: successConditionCheckStatus,
			},
		},
	}, {
		name: "failing-condition-checks",
		rcc:  resources.TaskConditionCheckState{&failingrcc},
		expectedStatus: v1beta1.PipelineRunTaskRunStatus{
			Status: &failedTaskRunStatus,
			ConditionChecks: map[string]*v1beta1.PipelineRunConditionCheckStatus{
				failingrcc.ConditionCheck.Name: failingConditionCheckStatus,
			},
		},
	}, {
		name: "multiple-condition-checks",
		rcc:  resources.TaskConditionCheckState{&successrcc, &failingrcc},
		expectedStatus: v1beta1.PipelineRunTaskRunStatus{
			Status: &failedTaskRunStatus,
			ConditionChecks: map[string]*v1beta1.PipelineRunConditionCheckStatus{
				successrcc.ConditionCheck.Name: successConditionCheckStatus,
				failingrcc.ConditionCheck.Name: failingConditionCheckStatus,
			},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1beta1.PipelineRun{
				ObjectMeta: baseObjectMeta("test-pipeline-run", "foo"),
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
				},
			}

			state := resources.PipelineRunState{{
				PipelineTask:            &pipelineTask,
				TaskRunName:             taskrunName,
				ResolvedConditionChecks: tc.rcc,
			}}
			pr.Status.InitializeConditions(testClock{})
			status := state.GetTaskRunsStatus(pr)
			expected := map[string]*v1beta1.PipelineRunTaskRunStatus{
				taskrunName: &tc.expectedStatus,
			}
			if d := cmp.Diff(status, expected, ignoreLastTransitionTime); d != "" {
				t.Fatalf("Did not get expected status for %s %s", tc.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcileOnCompletedPipelineRun(t *testing.T) {
	// TestReconcileOnCompletedPipelineRun runs "Reconcile" on a PipelineRun that already reached completion
	// and that does not have the latest status from TaskRuns yet. It checks that the TaskRun status is updated
	// in the PipelineRun status, that the completion status is not altered, that not error is returned and
	// a successful event is triggered
	taskRunName := "test-pipeline-run-completed-hello-world"
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-completed", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionTrue,
						Reason:  v1beta1.PipelineRunReasonSuccessful.String(),
						Message: "All Tasks have completed executing",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					taskRunName: {
						PipelineTaskName: "hello-world-1",
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	trs := []*v1beta1.TaskRun{createHelloWorldTaskRunWithStatus(taskRunName, "foo",
		"test-pipeline-run-completed", "test-pipeline", "",
		apis.Condition{
			Type: apis.ConditionSucceeded,
		})}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Succeeded All Tasks have completed executing",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-completed", wantEvents, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Errorf("# Actions: %d, Actions: %#v", len(actions), actions)
		t.Fatalf("Expected client to have at least two action implementation")
	}

	_ = getPipelineRunUpdates(t, actions)
	pipelineUpdates := 0
	for _, action := range actions {
		if action != nil {
			switch {
			case action.Matches("create", "taskruns"):
				t.Errorf("Expected client to not have created a TaskRun, but it did")
			case action.Matches("update", "pipelineruns"):
				pipelineUpdates++
			}
		}
	}

	if pipelineUpdates != 1 {
		// If only the pipelinerun status changed, we expect one update
		t.Fatalf("Expected client to have updated the pipelinerun twice, but it did %d times", pipelineUpdates)
	}

	// This PipelineRun should still be complete and the status should reflect that
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	expectedTaskRunsStatus := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	expectedTaskRunsStatus[taskRunName] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-1",
		Status: &v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}

	if d := cmp.Diff(reconciledRun.Status.TaskRuns, expectedTaskRunsStatus); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch %s", diff.PrintWantGot(d))
	}
}

func TestReconcileOnCancelledPipelineRunDeprecated(t *testing.T) {
	// TestReconcileOnCancelledPipelineRunDeprecated runs "Reconcile" on a PipelineRun that has been cancelled.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	// This test uses the deprecated status "PipelineRunCancelled" in PipelineRunSpec.
	prs := []*v1beta1.PipelineRun{createCancelledPipelineRun("test-pipeline-run-cancelled", v1beta1.PipelineRunSpecStatusCancelledDeprecated)}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	trs := []*v1beta1.TaskRun{createHelloWorldTaskRun("test-pipeline-run-cancelled-hello-world", "foo",
		"test-pipeline-run-cancelled", "test-pipeline")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-cancelled\" was cancelled",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// The PipelineRun should be still cancelled.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != ReasonCancelledDeprecated {
		t.Errorf("Expected PipelineRun to be cancelled, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func getConfigMapsWithEnabledAlphaAPIFields() []*corev1.ConfigMap {
	return []*corev1.ConfigMap{{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data:       map[string]string{apiFieldsFeatureFlag: config.AlphaAPIFields},
	}}
}

func TestReconcileOnCancelledPipelineRun(t *testing.T) {
	// TestReconcileOnCancelledPipelineRun runs "Reconcile" on a PipelineRun that has been cancelled.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{createCancelledPipelineRun("test-pipeline-run-cancelled", v1beta1.PipelineRunSpecStatusCancelled)}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	trs := []*v1beta1.TaskRun{createHelloWorldTaskRun("test-pipeline-run-cancelled-hello-world", "foo",
		"test-pipeline-run-cancelled", "test-pipeline")}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-cancelled\" was cancelled",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileForCustomTaskWithPipelineTaskTimedOut(t *testing.T) {
	names.TestingSeed()
	// TestReconcileForCustomTaskWithPipelineTaskTimedOut runs "Reconcile" on a PipelineRun.
	// It verifies that reconcile is successful, and the individual
	// custom task which has timed out, is patched as cancelled.
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "test"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
			}},
		},
	}}
	prName := "test-pipeline-run-custom-task-with-timeout"
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(prName, "test"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
		},
	}}
	ps[0].Spec.Tasks[0].TaskRef = &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"}
	startedRun := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-custom-task-with-timeout-hello-world-1",
			Namespace: "test",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "PipelineRun",
				Name: prName,
			}},
			Labels: map[string]string{
				pipeline.PipelineLabelKey:     "test-pipeline",
				pipeline.PipelineRunLabelKey:  prName,
				pipeline.PipelineTaskLabelKey: "hello-world-1",
			},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.RunSpec{
			Timeout: &metav1.Duration{Duration: time.Minute},
			Ref: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Example",
			},
		},
		Status: v1alpha1.RunStatus{
			RunStatusFields: v1alpha1.RunStatusFields{
				StartTime: &metav1.Time{Time: now.Add(-61 * time.Second)},
			},
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}
	runs := []*v1alpha1.Run{startedRun}
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"enable-custom-tasks": "true",
			},
		},
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   cms,
		Runs:         runs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 0",
	}
	_, clients := prt.reconcileRun("test", prName, wantEvents, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// The patch operation to cancel the run must be executed.
	got := []jsonpatch.Operation{}
	for _, a := range actions {
		if action, ok := a.(ktesting.PatchAction); ok {
			if a.(ktesting.PatchAction).Matches("patch", "runs") {
				err := json.Unmarshal(action.GetPatch(), &got)
				if err != nil {
					t.Fatalf("Expected to get a patch operation for cancel,"+
						" but got error: %v\n", err)
				}
				break
			}
		}
	}
	want := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     "RunCancelled",
	}}
	if d := cmp.Diff(got, want); d != "" {
		t.Fatalf("Expected cancel patch operation, but got a mismatch %s", diff.PrintWantGot(d))
	}
}

func TestReconcileForCustomTaskWithPipelineRunTimedOut(t *testing.T) {
	names.TestingSeed()
	// TestReconcileForCustomTaskWithPipelineRunTimedOut runs "Reconcile" on a
	// PipelineRun that has timed out.
	// It verifies that reconcile is successful, and the custom task has also timed
	// out and patched as cancelled.
	for _, tc := range []struct {
		name     string
		timeout  *metav1.Duration
		timeouts *v1beta1.TimeoutFields
	}{{
		name:    "spec.Timeout",
		timeout: &metav1.Duration{Duration: 12 * time.Hour},
	}, {
		name:     "spec.Timeouts.Pipeline",
		timeouts: &v1beta1.TimeoutFields{Pipeline: &metav1.Duration{Duration: 12 * time.Hour}},
	}} {
		t.Run(tc.name, func(*testing.T) {
			ps := []*v1beta1.Pipeline{{
				ObjectMeta: baseObjectMeta("test-pipeline", "test"),
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name:    "hello-world-1",
						TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
					}},
				},
			}}

			prName := "test-pipeline-run-custom-task"
			prs := []*v1beta1.PipelineRun{{
				ObjectMeta: baseObjectMeta(prName, "test"),
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
					ServiceAccountName: "test-sa",
					Timeout:            tc.timeout,
					Timeouts:           tc.timeouts,
				},
				Status: v1beta1.PipelineRunStatus{
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						StartTime: &metav1.Time{Time: now.AddDate(0, 0, -1)},
					},
				},
			}}

			cms := []*corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
					Data: map[string]string{
						"enable-custom-tasks": "true",
					},
				},
			}
			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				ConfigMaps:   cms,
			}
			prt := newPipelineRunTest(d, t)
			defer prt.Cancel()

			wantEvents := []string{
				fmt.Sprintf("Warning Failed PipelineRun \"%s\" failed to finish within \"12h0m0s\"", prName),
			}
			runName := "test-pipeline-run-custom-task-hello-world-1"

			reconciledRun, clients := prt.reconcileRun("test", prName, wantEvents, false)

			postReconcileRun, err := clients.Pipeline.TektonV1alpha1().Runs("test").Get(prt.TestAssets.Ctx, runName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Run get request failed, %v", err)
			}

			gotTimeoutValue := postReconcileRun.GetTimeout()
			expectedTimeoutValue := time.Second

			if d := cmp.Diff(gotTimeoutValue, expectedTimeoutValue); d != "" {
				t.Fatalf("Expected timeout for created Run, but got a mismatch %s", diff.PrintWantGot(d))
			}

			if reconciledRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on already timedout PipelineRun but was nil")
			}

			// The PipelineRun should be timed out.
			if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunTimeout" {
				t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s",
					reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
			}

			actions := clients.Pipeline.Actions()
			if len(actions) < 2 {
				t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
			}

			// The patch operation to cancel the run must be executed.
			got := []jsonpatch.Operation{}
			for _, a := range actions {
				if action, ok := a.(ktesting.PatchAction); ok {
					if a.(ktesting.PatchAction).Matches("patch", "runs") {
						err := json.Unmarshal(action.GetPatch(), &got)
						if err != nil {
							t.Fatalf("Expected to get a patch operation for cancel,"+
								" but got error: %v\n", err)
						}
						break
					}
				}
			}
			want := []jsonpatch.JsonPatchOperation{{
				Operation: "add",
				Path:      "/spec/status",
				Value:     "RunCancelled",
			}}
			if d := cmp.Diff(got, want); d != "" {
				t.Fatalf("Expected RunCancelled patch operation, but got a mismatch %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcileOnCancelledRunFinallyPipelineRun(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRun runs "Reconcile" on a PipelineRun that has been gracefully cancelled.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{createCancelledPipelineRun("test-pipeline-run-cancelled-run-finally", v1beta1.PipelineRunSpecStatusCancelledRunFinally)}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					RunAfter: []string{"hello-world-1"},
				},
			},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-cancelled-run-finally\" was cancelled",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be no task runs triggered for the pipeline tasks
	if len(reconciledRun.Status.TaskRuns) != 0 {
		t.Errorf("Expected PipelineRun status to have exactly no task runs, but was %v", len(reconciledRun.Status.TaskRuns))
	}

	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "hello-world-1",
	}, {
		Name: "hello-world-2",
	}}

	if d := cmp.Diff(expectedSkippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
		t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
	}

}

func TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTask(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTask runs "Reconcile" on a PipelineRun that has been gracefully cancelled.
	// It verifies that reconcile is successful, final tasks run, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{createCancelledPipelineRun("test-pipeline-run-cancelled-run-finally", v1beta1.PipelineRunSpecStatusCancelledRunFinally)}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					RunAfter: []string{"hello-world-1"},
				},
			},
			Finally: []v1beta1.PipelineTask{{
				Name: "final-task-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "some-task",
				},
			}},
		},
	}}
	ts := []*v1beta1.Task{
		simpleHelloWorldTask,
		simpleSomeTask,
	}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected a CompletionTime to be nil on incomplete PipelineRun but was %v", reconciledRun.Status.CompletionTime)
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete and unknown, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be exactly one task run triggered for the "final-task-1" final task
	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to have exactly one task run, but was %v", len(reconciledRun.Status.TaskRuns))
	}

	expectedTaskRunsStatus := &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "final-task-1",
		Status:           &v1beta1.TaskRunStatus{},
	}
	for _, taskRun := range reconciledRun.Status.TaskRuns {
		if d := cmp.Diff(taskRun, expectedTaskRunsStatus); d != "" {
			t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch %s", diff.PrintWantGot(d))
		}
	}
}

func TestReconcileOnCancelledRunFinallyPipelineRunWithRunningFinalTask(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRunWithRunningFinalTask runs "Reconcile" on a PipelineRun that has been gracefully cancelled.
	// It verifies that reconcile is successful and completed tasks and running final tasks are left untouched.
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-cancelled-run-finally", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"test-pipeline-run-cancelled-run-finally-hello-world": {
						PipelineTaskName: "hello-world-1",
						Status: &v1beta1.TaskRunStatus{
							Status: duckv1beta1.Status{
								Conditions: []apis.Condition{{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionTrue,
								}},
							},
						},
					},
					"test-pipeline-run-cancelled-run-finally-final-task": {
						PipelineTaskName: "final-task-1",
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name: "final-task-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "some-task",
				},
			}},
		},
	}}
	ts := []*v1beta1.Task{
		simpleHelloWorldTask,
		simpleSomeTask,
	}
	trs := []*v1beta1.TaskRun{
		createHelloWorldTaskRunWithStatus("test-pipeline-run-cancelled-run-finally-hello-world", "foo",
			"test-pipeline-run-cancelled-run-finally", "test-pipeline", "my-pod-name",
			apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}),
		createHelloWorldTaskRun("test-pipeline-run-cancelled-run-finally-final-task", "foo",
			"test-pipeline-run-cancelled-run-finally", "test-pipeline"),
	}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected a CompletionTime to be nil on incomplete PipelineRun but was %v", reconciledRun.Status.CompletionTime)
	}

	// This PipelineRun should still be complete and unknown, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete and unknown, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be 2 task runs, one for already completed "hello-world-1" task and one for the "final-task-1" final task
	if len(reconciledRun.Status.TaskRuns) != 2 {
		t.Errorf("Expected PipelineRun status to have 2 task runs, but was %v", len(reconciledRun.Status.TaskRuns))
	}

	actions := clients.Pipeline.Actions()
	patchActions := make([]ktesting.PatchAction, 0)
	for _, action := range actions {
		if patchAction, ok := action.(ktesting.PatchAction); ok {
			patchActions = append(patchActions, patchAction)
		}
	}
	if len(patchActions) != 0 {
		t.Errorf("Expected no patch actions, but was %v", len(patchActions))
	}
}

func TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTaskAndRetries(t *testing.T) {
	// TestReconcileOnCancelledRunFinallyPipelineRunWithFinalTaskAndRetries runs "Reconcile" on a PipelineRun that has
	// been gracefully cancelled. It verifies that reconcile is successful, the pipeline status updated and events generated.

	// Pipeline has a DAG task "hello-world-1" and Finally task "hello-world-2"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
				Retries: 2,
			}},
			Finally: []v1beta1.PipelineTask{{
				Name: "hello-world-2",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
			}},
		},
	}}

	// PipelineRun has been gracefully cancelled, and it has a TaskRun for DAG task "hello-world-1" that has failed
	// with reason of cancellation
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-cancelled-run-finally", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"test-pipeline-run-cancelled-run-finally-hello-world": {
						PipelineTaskName: "hello-world-1",
						Status: &v1beta1.TaskRunStatus{
							Status: duckv1beta1.Status{
								Conditions: []apis.Condition{{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionFalse,
									Reason: v1beta1.TaskRunReasonCancelled.String(),
								}},
							},
						},
					},
				},
			},
		},
	}}

	// TaskRun exists for DAG task "hello-world-1" that has failed with reason of cancellation
	trs := []*v1beta1.TaskRun{createHelloWorldTaskRunWithStatus("test-pipeline-run-cancelled-run-finally-hello-world", "foo",
		"test-pipeline-run-cancelled-run-finally", "test-pipeline", "my-pod-name",
		apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
			Reason: v1beta1.TaskRunSpecStatusCancelled,
		})}

	ts := []*v1beta1.Task{simpleHelloWorldTask}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		TaskRuns:     trs,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal CancelledRunningFinally Tasks Completed: 1 \\(Failed: 0, Cancelled 1\\), Incomplete: 1, Skipped: 0",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-cancelled-run-finally", wantEvents, false)

	// This PipelineRun should still be running to execute the finally task, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be running to execute the finally task, but was %v",
			reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// There should be two task runs (failed dag task and one triggered for the finally task)
	if len(reconciledRun.Status.TaskRuns) != 2 {
		t.Errorf("Expected PipelineRun status to have exactly two task runs, but was %v", len(reconciledRun.Status.TaskRuns))
	}

}

func TestReconcileCancelledRunFinallyFailsTaskRunCancellation(t *testing.T) {
	// TestReconcileCancelledRunFinallyFailsTaskRunCancellation runs "Reconcile" on a PipelineRun with a single TaskRun.
	// The TaskRun cannot be cancelled. Check that the pipelinerun graceful cancel fails, that reconcile fails and
	// an event is generated
	names.TestingSeed()
	ptName := "hello-world-1"
	prName := "test-pipeline-fails-to-cancel"
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(prName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Status:      v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  v1beta1.PipelineRunReasonRunning.String(),
						Message: "running...",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					prName + ptName: {
						PipelineTaskName: ptName,
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
				StartTime: &metav1.Time{Time: now},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{
		simpleHelloWorldTask,
	}
	trs := []*v1beta1.TaskRun{
		getTaskRun(
			"test-pipeline-fails-to-cancelhello-world-1",
			prName,
			"test-pipeline",
			"hello-world",
			corev1.ConditionUnknown,
		),
	}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   cms,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients
	failingReactorActivated := true

	// Make the patch call fail, i.e. make it so that the controller fails to cancel the TaskRun
	clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		return failingReactorActivated, nil, fmt.Errorf("i'm sorry Dave, i'm afraid i can't do that")
	})

	err := c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-cancel")
	if err == nil {
		t.Errorf("Expected to see error returned from reconcile after failing to cancel TaskRun but saw none!")
	}

	// Check that the PipelineRun is still running with correct error message
	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Get(testAssets.Ctx, "test-pipeline-fails-to-cancel", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// The PipelineRun should not be cancelled b/c we couldn't cancel the TaskRun
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected PipelineRun to still be running since the TaskRun could not be cancelled but succeeded condition is %v", condition.Status)
	}
	if condition.Reason != ReasonCouldntCancel {
		t.Errorf("Expected PipelineRun condition to indicate the cancellation failed but reason was %s", condition.Reason)
	}
	// The event here is "Normal" because in case we fail to cancel we leave the condition to unknown
	// Further reconcile might converge then the status of the pipeline.
	// See https://github.com/tektoncd/pipeline/issues/2647 for further details.
	wantEvents := []string{
		"Normal PipelineRunCouldntCancel PipelineRun \"test-pipeline-fails-to-cancel\" was cancelled but had errors trying to cancel TaskRuns",
		"Warning InternalError 1 error occurred",
	}
	err = eventstest.CheckEventsOrdered(t, testAssets.Recorder.Events, prName, wantEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}

	// Turn off failing reactor and retry reconciliation
	failingReactorActivated = false

	err = c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-cancel")
	if err == nil {
		// No error is ok
	} else if ok, _ := controller.IsRequeueKey(err); !ok { // Requeue is also fine.
		t.Errorf("Expected to cancel TaskRun successfully!")
	}

}

func TestReconcileTaskResolutionError(t *testing.T) {
	ts := []*v1beta1.Task{
		simpleHelloWorldTask,
	}
	ptName := "hello-world-1"
	prName := "test-pipeline-fails-task-resolution"
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(prName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Status:      v1beta1.PipelineRunSpecStatusCancelledRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  v1beta1.PipelineRunReasonRunning.String(),
						Message: "running...",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					prName + ptName: {
						PipelineTaskName: ptName,
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
				StartTime: &metav1.Time{Time: now},
			},
		},
	}}
	trs := []*v1beta1.TaskRun{
		getTaskRun(
			"test-pipeline-fails-task-resolutionhello-world-1",
			prName,
			"test-pipeline",
			"hello-world",
			corev1.ConditionUnknown,
		),
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    []*v1beta1.Pipeline{simpleHelloWorldPipeline},
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   []*corev1.ConfigMap{},
	}
	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	failingReactorActivated := true
	// Create an error when the Pipeline client attempts to resolve the Task
	clients.Pipeline.PrependReactor("*", "tasks", func(action ktesting.Action) (bool, runtime.Object, error) {
		return failingReactorActivated, nil, errors.New("etcdserver: leader changed")
	})
	err := c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-task-resolution")
	if err == nil {
		t.Errorf("Expected error returned from reconcile after failing to cancel TaskRun but saw none!")
	}

	if controller.IsPermanentError(err) {
		t.Errorf("Expected non-permanent error reconciling PipelineRun but got permanent error: %v", err)
	}

	// Check that the PipelineRun is still running with correct error message
	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Get(testAssets.Ctx, "test-pipeline-fails-task-resolution", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// The PipelineRun should not be cancelled since the error was retryable
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected PipelineRun to still be running but succeeded condition is %v", condition.Status)
	}

	failingReactorActivated = false
	err = c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-task-resolution")
	if err != nil {
		if ok, _ := controller.IsRequeueKey(err); !ok {
			t.Errorf("unexpected error in PipelineRun reconciliation: %v", err)
		}
	}
	condition = reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected PipelineRun to still be running but succeeded condition is %v", condition.Status)
	}
}

func TestReconcileOnStoppedRunFinallyPipelineRun(t *testing.T) {
	// TestReconcileOnStoppedRunFinallyPipelineRun runs "Reconcile" on a PipelineRun that has been gracefully stopped
	// and waits for all running tasks to be completed, before cancelling the run.
	// It verifies that reconcile is successful, final tasks run, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-stopped-run-finally", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-stopped-run-finally\" was cancelled",
	}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-stopped-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	if len(reconciledRun.Status.TaskRuns) != 0 {
		t.Fatalf("Expected no TaskRun but got %d", len(reconciledRun.Status.TaskRuns))
	}

	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "hello-world-1",
	}}

	if d := cmp.Diff(expectedSkippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
		t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
	}

}

func TestReconcileOnStoppedRunFinallyPipelineRunWithRunningTask(t *testing.T) {
	// TestReconcileOnStoppedRunFinallyPipelineRunWithRunningTask runs "Reconcile" on a PipelineRun that has been gracefully stopped
	// and waits for all running tasks to be completed, before cancelling the run.
	// It verifies that reconcile is successful, final tasks run, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-stopped-run-finally", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"test-pipeline-run-stopped-run-finally-hello-world": {
						PipelineTaskName: "hello-world-1",
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	trs := []*v1beta1.TaskRun{
		getTaskRun(
			"test-pipeline-run-stopped-run-finally-hello-world",
			"test-pipeline-run-stopped-run-finally",
			"test-pipeline",
			"hello-world",
			corev1.ConditionUnknown,
		),
	}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-stopped-run-finally", wantEvents, false)

	if reconciledRun.Status.CompletionTime != nil {
		t.Errorf("Expected a CompletionTime to be nil on incomplete PipelineRun but was %v", reconciledRun.Status.CompletionTime)
	}

	// This PipelineRun should still be complete and unknown, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete and unknown, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Fatalf("Expected 1 TaskRun but got %d", len(reconciledRun.Status.TaskRuns))
	}

	actions := clients.Pipeline.Actions()
	patchCount := 0
	for _, action := range actions {
		if _, ok := action.(ktesting.PatchAction); ok {
			patchCount++
		}
	}
	if patchCount != 0 {
		t.Errorf("Expected no patch action, but was %v", patchCount)
	}
}

func TestReconcileOnStoppedPipelineRunWithCompletedTask(t *testing.T) {
	// TestReconcileOnStoppedPipelineRunWithCompletedTask runs "Reconcile" on a PipelineRun that has been gracefully stopped
	// and waits for all running tasks to be completed, before stopping the run.
	// It verifies that reconcile is successful, final tasks run, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-stopped", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusStoppedRunFinally,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					"test-pipeline-run-stopped-hello-world": {
						PipelineTaskName: "hello-world-1",
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					RunAfter: []string{"hello-world-1"},
				},
			},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}
	trs := []*v1beta1.TaskRun{
		getTaskRun(
			"test-pipeline-run-stopped-hello-world",
			"test-pipeline-run-stopped",
			"test-pipeline",
			"hello-world",
			corev1.ConditionTrue,
		),
	}
	cms := getConfigMapsWithEnabledAlphaAPIFields()

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-stopped\" was cancelled",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-stopped", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Fatalf("Expected 1 TaskRun but got %d", len(reconciledRun.Status.TaskRuns))
	}

	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "hello-world-2",
	}}

	if d := cmp.Diff(expectedSkippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
		t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
	}

	actions := clients.Pipeline.Actions()
	patchCount := 0
	for _, action := range actions {
		if _, ok := action.(ktesting.PatchAction); ok {
			patchCount++
		}
	}
	if patchCount != 0 {
		t.Errorf("Expected no patch action, but was %v", patchCount)
	}
}

func TestReconcileOnPendingPipelineRun(t *testing.T) {
	// TestReconcileOnPendingPipelineRun runs "Reconcile" on a PipelineRun that is pending.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-pending", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusPending,
		},
	}}
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	ts := []*v1beta1.Task{}
	trs := []*v1beta1.TaskRun{}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{}
	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-pending", wantEvents, false)

	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() || condition.Reason != ReasonPending {
		t.Errorf("Expected PipelineRun condition to indicate the pending failed but reason was %s", condition.Reason)
	}

	if reconciledRun.Status.StartTime != nil {
		t.Errorf("Start time should be nil, not: %s", reconciledRun.Status.StartTime)
	}
}

func TestReconcileWithTimeout(t *testing.T) {
	// TestReconcileWithTimeout runs "Reconcile" on a PipelineRun that has timed out.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-with-timeout", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Timeout:            &metav1.Duration{Duration: 12 * time.Hour},
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now.AddDate(0, 0, -1)},
			},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-with-timeout\" failed to finish within \"12h0m0s\"",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-with-timeout", wantEvents, false)

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// The PipelineRun should be timed out.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunTimeout" {
		t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]

	// The TaskRun timeout should be less than or equal to the PipelineRun timeout.
	if actual.Spec.Timeout.Duration > prs[0].Spec.Timeout.Duration {
		t.Errorf("TaskRun timeout %s should be less than or equal to PipelineRun timeout %s", actual.Spec.Timeout.Duration.String(), prs[0].Spec.Timeout.Duration.String())
	}
}

func TestReconcileWithoutPVC(t *testing.T) {
	// TestReconcileWithoutPVC runs "Reconcile" on a PipelineRun that has two unrelated tasks.
	// It verifies that reconcile is successful and that no PVC is created
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
			},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run", []string{}, false)
	actions := clients.Pipeline.Actions()

	// Check that the expected TaskRun was created
	for _, a := range actions {
		if ca, ok := a.(ktesting.CreateAction); ok {
			obj := ca.GetObject()
			if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				t.Errorf("Did not expect to see a PVC created when no resources are linked. %s was created", pvc)
			}
		}
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileCancelledFailsTaskRunCancellation(t *testing.T) {
	// TestReconcileCancelledFailsTaskRunCancellation runs "Reconcile" on a PipelineRun with a single TaskRun.
	// The TaskRun cannot be cancelled. Check that the pipelinerun cancel fails, that reconcile fails and
	// an event is generated
	names.TestingSeed()
	ptName := "hello-world-1"
	prName := "test-pipeline-fails-to-cancel"
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(prName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Status:      v1beta1.PipelineRunSpecStatusCancelled,
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  v1beta1.PipelineRunReasonRunning.String(),
						Message: "running...",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					prName + ptName: {
						PipelineTaskName: ptName,
						Status:           &v1beta1.TaskRunStatus{},
					},
				},
				StartTime: &metav1.Time{Time: now},
			},
		},
	}}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	// Make the patch call fail, i.e. make it so that the controller fails to cancel the TaskRun
	clients.Pipeline.PrependReactor("patch", "taskruns", func(action ktesting.Action) (bool, runtime.Object, error) {
		return true, nil, fmt.Errorf("i'm sorry Dave, i'm afraid i can't do that")
	})

	err := c.Reconciler.Reconcile(testAssets.Ctx, "foo/test-pipeline-fails-to-cancel")
	if err == nil {
		t.Errorf("Expected to see error returned from reconcile after failing to cancel TaskRun but saw none!")
	}

	// Check that the PipelineRun is still running with correct error message
	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Get(testAssets.Ctx, "test-pipeline-fails-to-cancel", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	if val, ok := reconciledRun.GetLabels()[pipeline.PipelineLabelKey]; !ok {
		t.Fatalf("expected pipeline label")
	} else if d := cmp.Diff("test-pipeline", val); d != "" {
		t.Errorf("expected to see pipeline label. Diff %s", diff.PrintWantGot(d))
	}

	// The PipelineRun should not be cancelled b/c we couldn't cancel the TaskRun
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if !condition.IsUnknown() {
		t.Errorf("Expected PipelineRun to still be running since the TaskRun could not be cancelled but succeeded condition is %v", condition.Status)
	}
	if condition.Reason != ReasonCouldntCancel {
		t.Errorf("Expected PipelineRun condition to indicate the cancellation failed but reason was %s", condition.Reason)
	}
	// The event here is "Normal" because in case we fail to cancel we leave the condition to unknown
	// Further reconcile might converge then the status of the pipeline.
	// See https://github.com/tektoncd/pipeline/issues/2647 for further details.
	wantEvents := []string{
		"Normal PipelineRunCouldntCancel PipelineRun \"test-pipeline-fails-to-cancel\" was cancelled but had errors trying to cancel TaskRuns",
		"Warning InternalError 1 error occurred",
	}
	err = eventstest.CheckEventsOrdered(t, testAssets.Recorder.Events, prName, wantEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}
}

func TestReconcileCancelledPipelineRun(t *testing.T) {
	// TestReconcileCancelledPipelineRun runs "Reconcile" on a PipelineRun that has been cancelled.
	// The PipelineRun had no TaskRun associated yet, and no TaskRun should have been created.
	// It verifies that reconcile is successful, the pipeline status updated and events generated.
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
				Retries: 1,
			}},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-cancelled", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Status:      v1beta1.PipelineRunSpecStatusCancelled,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Warning Failed PipelineRun \"test-pipeline-run-cancelled\" was cancelled",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-cancelled", wantEvents, false)
	actions := clients.Pipeline.Actions()

	// The PipelineRun should be still cancelled.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != ReasonCancelled {
		t.Errorf("Expected PipelineRun to be cancelled, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Check that no TaskRun is created or run
	for _, action := range actions {
		actionType := fmt.Sprintf("%T", action)
		if !(actionType == "testing.UpdateActionImpl" || actionType == "testing.GetActionImpl") {
			t.Errorf("Expected a TaskRun to be get/updated, but it was %s", actionType)
		}
	}
}

func TestReconcilePropagateLabels(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-with-labels",
			Namespace: "foo",
			Labels: map[string]string{
				"PipelineRunLabel":        "PipelineRunValue",
				pipeline.PipelineLabelKey: "WillNotBeUsed",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	expectedObjectMeta := taskRunObjectMeta("test-pipeline-run-with-labels-hello-world-1", "foo", "test-pipeline-run-with-labels", "test-pipeline", "hello-world-1", false)
	expectedObjectMeta.Labels["PipelineRunLabel"] = "PipelineRunValue"
	expected := &v1beta1.TaskRun{
		ObjectMeta: expectedObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-with-labels", []string{}, false)
	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]
	if d := cmp.Diff(actual, expected); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expected, diff.PrintWantGot(d))
	}
}

func TestReconcilePropagateLabelsPending(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-with-labels",
			Namespace: "foo",
			Labels: map[string]string{
				"PipelineRunLabel":        "PipelineRunValue",
				pipeline.PipelineLabelKey: "WillNotBeUsed",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusPending,
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-with-labels", []string{}, false)

	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "test-pipeline-run-with-labels", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error when updating status: %v", err)
	}

	want := "test-pipeline"
	got := reconciledRun.ObjectMeta.Labels["tekton.dev/pipeline"]
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("expected to see label %v created. Diff %s", want, diff.PrintWantGot(d))
	}
}

func TestReconcilePropagateLabelsCancelled(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-with-labels",
			Namespace: "foo",
			Labels: map[string]string{
				"PipelineRunLabel":        "PipelineRunValue",
				pipeline.PipelineLabelKey: "WillNotBeUsed",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             v1beta1.PipelineRunSpecStatusCancelled,
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-with-labels", []string{}, false)

	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Get(prt.TestAssets.Ctx, "test-pipeline-run-with-labels", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error when updating status: %v", err)
	}

	want := "test-pipeline"
	got := reconciledRun.ObjectMeta.Labels["tekton.dev/pipeline"]
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("expected to see label %v created. Diff %s", want, diff.PrintWantGot(d))
	}
}

func TestReconcileWithDifferentServiceAccounts(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-0",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world-task",
					},
				},
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world-task",
					},
				},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
			ServiceAccountNames: []v1beta1.PipelineRunSpecServiceAccountName{{
				TaskName:           "hello-world-1",
				ServiceAccountName: "test-sa-1",
			}},
		},
	}}
	ts := []*v1beta1.Task{
		{ObjectMeta: baseObjectMeta("hello-world-task", "foo")},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	taskRunNames := []string{"test-pipeline-run-different-service-accs-hello-world-0", "test-pipeline-run-different-service-accs-hello-world-1"}

	expectedTaskRuns := []*v1beta1.TaskRun{
		{
			ObjectMeta: taskRunObjectMeta(taskRunNames[0], "foo", "test-pipeline-run-different-service-accs", "test-pipeline", "hello-world-0", false),
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world-task",
				},
				ServiceAccountName: "test-sa-0",
				Resources:          &v1beta1.TaskRunResources{},
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
		{
			ObjectMeta: taskRunObjectMeta(taskRunNames[1], "foo", "test-pipeline-run-different-service-accs", "test-pipeline", "hello-world-1", false),
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world-task",
				},
				ServiceAccountName: "test-sa-1",
				Resources:          &v1beta1.TaskRunResources{},
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		},
	}
	for i := range ps[0].Spec.Tasks {
		// Check that the expected TaskRun was created
		actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").Get(prt.TestAssets.Ctx, taskRunNames[i], metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Expected a TaskRun to be created, but it wasn't: %s", err)
		}
		if d := cmp.Diff(actual, expectedTaskRuns[i], ignoreResourceVersion); d != "" {
			t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i], diff.PrintWantGot(d))
		}

	}
}

func TestReconcileCustomTasksWithDifferentServiceAccounts(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name:    "hello-world-0",
					TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
				},
				{
					Name:    "hello-world-1",
					TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
				},
			},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
			ServiceAccountNames: []v1beta1.PipelineRunSpecServiceAccountName{{
				TaskName:           "hello-world-1",
				ServiceAccountName: "test-sa-1",
			}},
		},
	}}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"enable-custom-tasks": "true",
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	runNames := []string{"test-pipeline-run-different-service-accs-hello-world-0", "test-pipeline-run-different-service-accs-hello-world-1"}
	expectedSANames := []string{"test-sa-0", "test-sa-1"}

	for i := range ps[0].Spec.Tasks {
		actual, err := clients.Pipeline.TektonV1alpha1().Runs("foo").Get(prt.TestAssets.Ctx, runNames[i], metav1.GetOptions{})
		if err != nil {
			t.Errorf("Expected a Run %s to be created but it wasn't: %s", runNames[i], err)
			continue
		}
		if actual.Spec.ServiceAccountName != expectedSANames[i] {
			t.Errorf("Expected Run %s to have service account %s but it was %s", runNames[i], expectedSANames[i], actual.Spec.ServiceAccountName)
		}
	}
}

// TestReconcileWithTimeoutAndRetry runs "Reconcile" against pipelines with
// retries and timeout settings, and status that represents different number of
// retries already performed.  It verifies the reconciled status and events
// generated
func TestReconcileWithTimeoutAndRetry(t *testing.T) {
	for _, tc := range []struct {
		name               string
		retries            int
		conditionSucceeded corev1.ConditionStatus
		wantEvents         []string
	}{{
		name:               "One try has to be done",
		retries:            1,
		conditionSucceeded: corev1.ConditionFalse,
		wantEvents: []string{
			"Warning Failed PipelineRun \"test-pipeline-retry-run-with-timeout\" failed to finish within",
		},
	}, {
		name:               "No more retries are needed",
		retries:            2,
		conditionSucceeded: corev1.ConditionUnknown,
		wantEvents: []string{
			"Warning Failed PipelineRun \"test-pipeline-retry-run-with-timeout\" failed to finish within",
		},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ps := []*v1beta1.Pipeline{{
				ObjectMeta: baseObjectMeta("test-pipeline-retry", "foo"),
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{{
						Name: "hello-world-1",
						TaskRef: &v1beta1.TaskRef{
							Name: "hello-world",
						},
						Retries: tc.retries,
					}},
				},
			}}
			prs := []*v1beta1.PipelineRun{{
				ObjectMeta: baseObjectMeta("test-pipeline-retry-run-with-timeout", "foo"),
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline-retry"},
					ServiceAccountName: "test-sa",
					Timeout:            &metav1.Duration{Duration: 12 * time.Hour},
				},
				Status: v1beta1.PipelineRunStatus{
					PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
						StartTime: &metav1.Time{Time: now.AddDate(0, 0, -1)},
					},
				},
			}}

			ts := []*v1beta1.Task{
				simpleHelloWorldTask,
			}
			trs := []*v1beta1.TaskRun{{
				ObjectMeta: baseObjectMeta("hello-world-1", "foo"),
				Status: v1beta1.TaskRunStatus{
					Status: duckv1beta1.Status{
						Conditions: duckv1beta1.Conditions{
							apis.Condition{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionFalse,
							},
						},
					},
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						PodName: "my-pod-name",
						RetriesStatus: []v1beta1.TaskRunStatus{{
							Status: duckv1beta1.Status{
								Conditions: []apis.Condition{{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionFalse,
								}},
							},
						}},
					},
				},
			}}

			prtrs := &v1beta1.PipelineRunTaskRunStatus{
				PipelineTaskName: "hello-world-1",
				Status:           &trs[0].Status,
			}
			prs[0].Status.TaskRuns = make(map[string]*v1beta1.PipelineRunTaskRunStatus)
			prs[0].Status.TaskRuns["hello-world-1"] = prtrs

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
				TaskRuns:     trs,
			}
			prt := newPipelineRunTest(d, t)
			defer prt.Cancel()

			reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-retry-run-with-timeout", []string{}, false)

			if len(reconciledRun.Status.TaskRuns["hello-world-1"].Status.RetriesStatus) != tc.retries {
				t.Fatalf(" %d retry expected but %d ", tc.retries, len(reconciledRun.Status.TaskRuns["hello-world-1"].Status.RetriesStatus))
			}

			if status := reconciledRun.Status.TaskRuns["hello-world-1"].Status.GetCondition(apis.ConditionSucceeded).Status; status != tc.conditionSucceeded {
				t.Fatalf("Succeeded expected to be %s but is %s", tc.conditionSucceeded, status)
			}
		})
	}
}

func TestReconcilePropagateAnnotations(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pipeline-run-with-annotations",
			Namespace:   "foo",
			Annotations: map[string]string{"PipelineRunAnnotation": "PipelineRunValue"},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-with-annotations", []string{}, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]
	expectedTaskRunObjectMeta := taskRunObjectMeta("test-pipeline-run-with-annotations-hello-world-1", "foo", "test-pipeline-run-with-annotations", "test-pipeline", "hello-world-1", false)
	expectedTaskRunObjectMeta.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: expectedTaskRunObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestGetTaskRunTimeout(t *testing.T) {
	prName := "pipelinerun-timeouts"
	ns := "foo"
	p := "pipeline"

	tcs := []struct {
		name     string
		pr       *v1beta1.PipelineRun
		rprt     *resources.ResolvedPipelineRunTask
		expected *metav1.Duration
	}{{
		name: "nil timeout duration",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: nil,
			},
		},
		expected: &metav1.Duration{Duration: 60 * time.Minute},
	}, {
		name: "timeout specified in pr",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: 20 * time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: nil,
			},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "0 timeout duration",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: 0 * time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: nil,
			},
		},
		expected: &metav1.Duration{Duration: 0 * time.Minute},
	}, {
		name: "taskrun being created after timeout expired",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: 1 * time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-2 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: nil,
			},
		},
		expected: &metav1.Duration{Duration: 1 * time.Second},
	}, {
		name: "taskrun being created with timeout for PipelineTask",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: 20 * time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		expected: &metav1.Duration{Duration: 2 * time.Minute},
	}, {
		name: "0 timeout duration for PipelineRun, PipelineTask timeout still applied",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: 0 * time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		expected: &metav1.Duration{Duration: 2 * time.Minute},
	}, {
		name: "taskstimeout specified in pr",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: nil,
			},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "40m timeout duration, 20m taskstimeout duration",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 40 * time.Minute},
					Tasks:    &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: nil,
			},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "taskrun being created with taskstimeout for PipelineTask",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		expected: &metav1.Duration{Duration: 2 * time.Minute},
	}, {
		name: "tasks.timeout < pipeline.tasks[].timeout",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: 1 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		expected: &metav1.Duration{Duration: 1 * time.Minute},
	}, {
		name: "taskrun with elapsed time; timeouts.tasks applies",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-10 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
			TaskRun: &v1beta1.TaskRun{
				Status: v1beta1.TaskRunStatus{
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						StartTime: nil,
					},
				},
			},
		},
		expected: &metav1.Duration{Duration: 10 * time.Minute},
	}, {
		name: "taskrun with elapsed time; task.timeout applies",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Tasks: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-10 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 15 * time.Minute},
			},
			TaskRun: &v1beta1.TaskRun{
				Status: v1beta1.TaskRunStatus{
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						StartTime: nil,
					},
				},
			},
		},
		expected: &metav1.Duration{Duration: 10 * time.Minute},
	}, {
		name: "taskrun with elapsed time; timeouts.pipeline applies",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-10 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 15 * time.Minute},
			},
			TaskRun: &v1beta1.TaskRun{
				Status: v1beta1.TaskRunStatus{
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						StartTime: nil,
					},
				},
			},
		},
		expected: &metav1.Duration{Duration: 10 * time.Minute},
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(getTaskRunTimeout(context.TODO(), tc.pr, tc.rprt, testClock{}), tc.expected); d != "" {
				t.Errorf("Unexpected task run timeout. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetFinallyTaskRunTimeout(t *testing.T) {
	prName := "pipelinerun-finallyTimeouts"
	ns := "foo"
	p := "pipeline"

	tcs := []struct {
		name     string
		pr       *v1beta1.PipelineRun
		rprt     *resources.ResolvedPipelineRunTask
		expected *metav1.Duration
	}{{
		name: "nil timeout duration",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     nil,
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
		},
		expected: &metav1.Duration{Duration: 60 * time.Minute},
	}, {
		name: "timeout specified in pr",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: 20 * time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "taskrun being created after timeout expired",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeout:     &metav1.Duration{Duration: time.Minute},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-2 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
		},
		expected: &metav1.Duration{Duration: 1 * time.Second},
	}, {
		name: "40m timeout duration, 20m taskstimeout duration",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 40 * time.Minute},
					Tasks:    &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "only timeouts.finally set",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Finally: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "40m timeout duration, 20m taskstimeout duration, 20m finallytimeout duration",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 40 * time.Minute},
					Tasks:    &metav1.Duration{Duration: 20 * time.Minute},
					Finally:  &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "use pipeline.finally[].timeout",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		expected: &metav1.Duration{Duration: 2 * time.Minute},
	}, {
		name: "finally timeout < pipeline.finally[].timeout",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Pipeline: &metav1.Duration{Duration: 40 * time.Minute},
					Finally:  &metav1.Duration{Duration: 1 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 2 * time.Minute},
			},
		},
		expected: &metav1.Duration{Duration: 1 * time.Minute},
	}, {
		name: "finally taskrun with elapsed time; tasks.finally applies",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Finally: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-10 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{},
			TaskRun: &v1beta1.TaskRun{
				Status: v1beta1.TaskRunStatus{
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						StartTime: nil,
					},
				},
			},
		},
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "finally taskrun with elapsed time; task.timeout applies",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Finally: &metav1.Duration{Duration: 20 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-10 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 15 * time.Minute},
			},
			TaskRun: &v1beta1.TaskRun{
				Status: v1beta1.TaskRunStatus{
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						StartTime: nil,
					},
				},
			},
		},
		expected: &metav1.Duration{Duration: 15 * time.Minute},
	}, {
		name: "finally taskrun with elapsed time; timeouts.pipeline applies",
		pr: &v1beta1.PipelineRun{
			ObjectMeta: baseObjectMeta(prName, ns),
			Spec: v1beta1.PipelineRunSpec{
				PipelineRef: &v1beta1.PipelineRef{Name: p},
				Timeouts: &v1beta1.TimeoutFields{
					Finally:  &metav1.Duration{Duration: 20 * time.Minute},
					Pipeline: &metav1.Duration{Duration: 21 * time.Minute},
				},
			},
			Status: v1beta1.PipelineRunStatus{
				PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
					StartTime: &metav1.Time{Time: now.Add(-10 * time.Minute)},
				},
			},
		},
		rprt: &resources.ResolvedPipelineRunTask{
			PipelineTask: &v1beta1.PipelineTask{
				Timeout: &metav1.Duration{Duration: 15 * time.Minute},
			},
			TaskRun: &v1beta1.TaskRun{
				Status: v1beta1.TaskRunStatus{
					TaskRunStatusFields: v1beta1.TaskRunStatusFields{
						StartTime: nil,
					},
				},
			},
		},
		expected: &metav1.Duration{Duration: 11 * time.Minute},
	},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(tc.expected, getFinallyTaskRunTimeout(context.TODO(), tc.pr, tc.rprt, testClock{})); d != "" {
				t.Errorf("Unexpected finally task run timeout. Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

// TestReconcileAndPropagateCustomPipelineTaskRunSpec tests that custom PipelineTaskRunSpec declared
// in PipelineRun is propagated to created TaskRuns
func TestReconcileAndPropagateCustomPipelineTaskRunSpec(t *testing.T) {
	names.TestingSeed()
	prName := "test-pipeline-run"
	ps := []*v1beta1.Pipeline{simpleHelloWorldPipeline}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:        prName,
			Namespace:   "foo",
			Annotations: map[string]string{"PipelineRunAnnotation": "PipelineRunValue"},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
				PipelineTaskName:       "hello-world-1",
				TaskServiceAccountName: "custom-sa",
				TaskPodTemplate: &pod.Template{
					NodeSelector: map[string]string{
						"workloadtype": "tekton",
					},
				},
			}},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", prName, []string{}, false)
	actions := clients.Pipeline.Actions()

	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]
	expectedTaskRunObjectMeta := taskRunObjectMeta("test-pipeline-run-hello-world-1", "foo", "test-pipeline-run", "test-pipeline", "hello-world-1", false)
	expectedTaskRunObjectMeta.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: expectedTaskRunObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
			ServiceAccountName: "custom-sa",
			PodTemplate: &pod.Template{
				NodeSelector: map[string]string{
					"workloadtype": "tekton",
				},
			},
			Resources: &v1beta1.TaskRunResources{},
			Timeout:   &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see propagated custom ServiceAccountName and PodTemplate in TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestReconcileCustomTasksWithTaskRunSpec(t *testing.T) {
	names.TestingSeed()
	prName := "test-pipeline-run"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "hello-world-1",
				TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
			}},
		},
	}}

	serviceAccount := "custom-sa"
	podTemplate := &pod.Template{
		NodeSelector: map[string]string{
			"workloadtype": "tekton",
		},
	}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(prName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			TaskRunSpecs: []v1beta1.PipelineTaskRunSpec{{
				PipelineTaskName:       "hello-world-1",
				TaskServiceAccountName: serviceAccount,
				TaskPodTemplate:        podTemplate,
			}},
		},
	}}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"enable-custom-tasks": "true",
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", prName, []string{}, false)

	runName := "test-pipeline-run-hello-world-1"

	actual, err := clients.Pipeline.TektonV1alpha1().Runs("foo").Get(prt.TestAssets.Ctx, runName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Expected a Run %s to be created but it wasn't: %s", runName, err)
	}
	if actual.Spec.ServiceAccountName != serviceAccount {
		t.Errorf("Expected Run %s to have service account %s but it was %s", runName, serviceAccount, actual.Spec.ServiceAccountName)
	}
	if d := cmp.Diff(actual.Spec.PodTemplate, podTemplate); d != "" {
		t.Errorf("Incorrect pod template in Run %s. Diff %s", runName, diff.PrintWantGot(d))
	}

}

func TestReconcileWithConditionChecks(t *testing.T) {
	// TestReconcileWithConditionChecks runs "Reconcile" on a PipelineRun that has a task with
	// multiple conditions. It verifies that reconcile is successful, taskruns are created and
	// the status is updated. It checks that the correct events are sent.
	names.TestingSeed()
	prName := "test-pipeline-run"
	conditions := []*v1alpha1.Condition{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cond-1",
				Namespace: "foo",
				Labels: map[string]string{
					"label-1": "value-1",
					"label-2": "value-2",
				},
				Annotations: map[string]string{
					"annotation-1": "value-1",
				},
			},
			Spec: v1alpha1.ConditionSpec{
				Check: v1alpha1.Step{
					Container: corev1.Container{
						Image: "foo",
						Args:  []string{"bar"},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cond-2",
				Namespace: "foo",
				Labels: map[string]string{
					"label-3": "value-3",
					"label-4": "value-4",
				},
			},
			Spec: v1alpha1.ConditionSpec{
				Check: v1alpha1.Step{
					Container: corev1.Container{
						Image: "foo",
						Args:  []string{"bar"},
					},
				},
			},
		},
	}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "hello-world-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
				Conditions: []v1beta1.PipelineTaskCondition{
					{
						ConditionRef: "cond-1",
					},
					{
						ConditionRef: "cond-2",
					},
				},
			}},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:        prName,
			Namespace:   "foo",
			Annotations: map[string]string{"PipelineRunAnnotation": "PipelineRunValue"},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		Conditions:   conditions,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 0",
	}
	_, clients := prt.reconcileRun("foo", prName, wantEvents, false)

	ccNameBase := prName + "-hello-world-1"
	ccNames := map[string]string{
		"cond-1": ccNameBase + "-cond-1-0",
		"cond-2": ccNameBase + "-cond-2-1",
	}
	expectedConditionChecks := make([]*v1beta1.TaskRun, len(conditions))
	for index, condition := range conditions {
		expectedConditionChecks[index] = makeExpectedTr(condition.Name, ccNames[condition.Name], condition.Labels, condition.Annotations)
	}

	actions := clients.Pipeline.Actions()
	if len(actions) < 3 {
		t.Fatalf("Expected client to have at least three action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)
	if d := cmp.Diff(actual, expectedConditionChecks); d != "" {
		t.Errorf("expected to see 2 ConditionCheck TaskRuns created. Diff %s", diff.PrintWantGot(d))
	}
}

func TestReconcileWithFailingConditionChecks(t *testing.T) {
	// TestReconcileWithFailingConditionChecks runs "Reconcile" on a PipelineRun that has a task with
	// multiple conditions, some that fails. It verifies that reconcile is successful, taskruns are
	// created and the status is updated. It checks that the correct events are sent.
	names.TestingSeed()
	conditions := []*v1alpha1.Condition{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "always-false",
			Namespace: "foo",
		},
		Spec: v1alpha1.ConditionSpec{
			Check: v1alpha1.Step{
				Container: corev1.Container{
					Image: "foo",
					Args:  []string{"bar"},
				},
			},
		},
	}}
	pipelineRunName := "test-pipeline-run-with-conditions"
	prccs := make(map[string]*v1beta1.PipelineRunConditionCheckStatus)

	conditionCheckName := pipelineRunName + "task-2-always-false"
	prccs[conditionCheckName] = &v1beta1.PipelineRunConditionCheckStatus{
		ConditionName: "always-false-0",
		Status:        &v1beta1.ConditionCheckStatus{},
	}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "task-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
				{
					Name: "task-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "always-false",
					}},
				},
				{
					Name: "task-3",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					RunAfter: []string{"task-1"},
				},
			},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pipeline-run-with-conditions",
			Namespace:   "foo",
			Annotations: map[string]string{"PipelineRunAnnotation": "PipelineRunValue"},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  v1beta1.PipelineRunReasonRunning.String(),
						Message: "Not all Tasks in the Pipeline have finished executing",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					pipelineRunName + "task-1": {
						PipelineTaskName: "task-1",
						Status:           &v1beta1.TaskRunStatus{},
					},
					pipelineRunName + "task-2": {
						PipelineTaskName: "task-2",
						Status:           &v1beta1.TaskRunStatus{},
						ConditionChecks:  prccs,
					},
				},
			},
		},
	}}

	ts := []*v1beta1.Task{simpleHelloWorldTask}
	trs := []*v1beta1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineRunName + "task-1",
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "kind",
					Name: "name",
				}},
				Namespace: "foo",
				Labels: map[string]string{
					pipeline.PipelineLabelKey:    "test-pipeine-run-with-conditions",
					pipeline.PipelineRunLabelKey: "test-pipeline",
					pipeline.MemberOfLabelKey:    v1beta1.PipelineTasks,
				},
			},
			Spec: v1beta1.TaskRunSpec{
				TaskRef: &v1beta1.TaskRef{
					Name: "hello-world",
				},
			},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: conditionCheckName,
				OwnerReferences: []metav1.OwnerReference{{
					Kind: "kind",
					Name: "name",
				}},
				Namespace: "foo",
				Labels: map[string]string{
					pipeline.PipelineLabelKey:    "test-pipeine-run-with-conditions",
					pipeline.PipelineRunLabelKey: "test-pipeline",
					pipeline.ConditionCheckKey:   conditionCheckName,
					pipeline.ConditionNameKey:    "always-false",
				},
			},
			Spec: v1beta1.TaskRunSpec{TaskSpec: &v1beta1.TaskSpec{}},
			Status: v1beta1.TaskRunStatus{
				Status: duckv1beta1.Status{
					Conditions: duckv1beta1.Conditions{
						apis.Condition{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		Conditions:   conditions,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 1 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 1",
	}
	_, clients := prt.reconcileRun("foo", pipelineRunName, wantEvents, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 2 {
		t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, actions)[0]
	expectedTaskRunObjectMeta := taskRunObjectMeta("test-pipeline-run-with-conditions-task-3", "foo", "test-pipeline-run-with-conditions", "test-pipeline", "task-3", false)
	expectedTaskRunObjectMeta.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: expectedTaskRunObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see ConditionCheck TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func makeExpectedTr(condName, ccName string, labels, annotations map[string]string) *v1beta1.TaskRun {
	om := taskRunObjectMeta(ccName, "foo", "test-pipeline-run", "test-pipeline", "hello-world-1", false)
	for k, v := range labels {
		om.Labels[k] = v
	}
	om.Labels[pipeline.ConditionCheckKey] = ccName
	om.Labels[pipeline.ConditionNameKey] = condName
	if len(om.Annotations) == 0 {
		om.Annotations = map[string]string{}
	}
	for k, v := range annotations {
		om.Annotations[k] = v
	}
	om.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"

	return &v1beta1.TaskRun{
		ObjectMeta: om,
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Name:  "condition-check-" + condName,
						Image: "foo",
						Args:  []string{"bar"},
					},
				}},
			},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}
}

func ensurePVCCreated(ctx context.Context, t *testing.T, clients test.Clients, name, namespace string) {
	t.Helper()
	_, err := clients.Kube.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Expected PVC %s to be created for VolumeResource but did not exist", name)
	}
	pvcCreated := false
	for _, a := range clients.Kube.Actions() {
		if a.GetVerb() == "create" && a.GetResource().Resource == "persistentvolumeclaims" {
			pvcCreated = true
		}
	}
	if !pvcCreated {
		t.Errorf("Expected to see volume resource PVC created but didn't")
	}
}

func TestReconcileWithWhenExpressionsWithParameters(t *testing.T) {
	names.TestingSeed()
	prName := "test-pipeline-run"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "run",
				Type: v1beta1.ParamTypeString,
			}},
			Tasks: []v1beta1.PipelineTask{
				{
					Name:    "hello-world-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world-1"},
					WhenExpressions: []v1beta1.WhenExpression{
						{
							Input:    "foo",
							Operator: selection.NotIn,
							Values:   []string{"bar"},
						},
						{
							Input:    "$(params.run)",
							Operator: selection.In,
							Values:   []string{"yes"},
						},
					},
				},
				{
					Name:    "hello-world-2",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world-2"},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "$(params.run)",
						Operator: selection.NotIn,
						Values:   []string{"yes"},
					}},
				},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:        prName,
			Namespace:   "foo",
			Annotations: map[string]string{"PipelineRunAnnotation": "PipelineRunValue"},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Params: []v1beta1.Param{{
				Name:  "run",
				Value: *v1beta1.NewArrayOrString("yes"),
			}},
		},
	}}
	ts := []*v1beta1.Task{
		{ObjectMeta: baseObjectMeta("hello-world-1", "foo")},
		{ObjectMeta: baseObjectMeta("hello-world-2", "foo")},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 1",
	}
	pipelineRun, clients := prt.reconcileRun("foo", prName, wantEvents, false)

	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=hello-world-1,tekton.dev/pipelineRun=test-pipeline-run",
		Limit:         1,
	})
	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRun got %d", len(actual.Items))
	}
	expectedTaskRunName := "test-pipeline-run-hello-world-1"
	expectedTaskRunObjectMeta := taskRunObjectMeta(expectedTaskRunName, "foo", "test-pipeline-run", "test-pipeline", "hello-world-1", false)
	expectedTaskRunObjectMeta.Annotations["PipelineRunAnnotation"] = "PipelineRunValue"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: expectedTaskRunObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world-1"},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(&actualTaskRun, expectedTaskRun, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}

	actualWhenExpressionsInTaskRun := pipelineRun.Status.PipelineRunStatusFields.TaskRuns[expectedTaskRunName].WhenExpressions
	expectedWhenExpressionsInTaskRun := []v1beta1.WhenExpression{{
		Input:    "foo",
		Operator: "notin",
		Values:   []string{"bar"},
	}, {
		Input:    "yes",
		Operator: "in",
		Values:   []string{"yes"},
	}}
	if d := cmp.Diff(expectedWhenExpressionsInTaskRun, actualWhenExpressionsInTaskRun); d != "" {
		t.Errorf("expected to see When Expressions %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "hello-world-2",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "yes",
			Operator: "notin",
			Values:   []string{"yes"},
		}},
	}}
	if d := cmp.Diff(actualSkippedTasks, expectedSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	skippedTasks := []string{"hello-world-2"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

func TestReconcileWithWhenExpressionsWithTaskResults(t *testing.T) {
	names.TestingSeed()
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name:    "a-task",
					TaskRef: &v1beta1.TaskRef{Name: "a-task"},
				},
				{
					Name:    "b-task",
					TaskRef: &v1beta1.TaskRef{Name: "b-task"},
					WhenExpressions: []v1beta1.WhenExpression{
						{
							Input:    "$(tasks.a-task.results.aResult)",
							Operator: selection.In,
							Values:   []string{"aResultValue"},
						},
						{
							Input:    "aResultValue",
							Operator: selection.In,
							Values:   []string{"$(tasks.a-task.results.aResult)"},
						},
					},
				},
				{
					Name:    "c-task",
					TaskRef: &v1beta1.TaskRef{Name: "c-task"},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "$(tasks.a-task.results.aResult)",
						Operator: selection.In,
						Values:   []string{"missing"},
					}},
				},
				{
					Name:     "d-task",
					TaskRef:  &v1beta1.TaskRef{Name: "d-task"},
					RunAfter: []string{"c-task"},
				},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
		},
	}}
	ts := []*v1beta1.Task{
		{ObjectMeta: baseObjectMeta("a-task", "foo")},
		{ObjectMeta: baseObjectMeta("b-task", "foo")},
		{ObjectMeta: baseObjectMeta("c-task", "foo")},
		{ObjectMeta: baseObjectMeta("d-task", "foo")},
	}
	trs := []*v1beta1.TaskRun{{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task-xxyyy", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "a-task",
			true),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world"},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "aResult",
					Value: "aResultValue",
				}},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 1 \\(Failed: 0, Cancelled 0\\), Incomplete: 2, Skipped: 1",
	}
	pipelineRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", wantEvents, false)

	expectedTaskRunName := "test-pipeline-run-different-service-accs-b-task"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta(expectedTaskRunName, "foo", "test-pipeline-run-different-service-accs", "test-pipeline", "b-task", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "b-task"},
			ServiceAccountName: "test-sa-0",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}
	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=b-task,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(&actualTaskRun, expectedTaskRun, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}

	actualWhenExpressionsInTaskRun := pipelineRun.Status.PipelineRunStatusFields.TaskRuns[expectedTaskRunName].WhenExpressions
	expectedWhenExpressionsInTaskRun := []v1beta1.WhenExpression{{
		Input:    "aResultValue",
		Operator: "in",
		Values:   []string{"aResultValue"},
	}, {
		Input:    "aResultValue",
		Operator: "in",
		Values:   []string{"aResultValue"},
	}}
	if d := cmp.Diff(expectedWhenExpressionsInTaskRun, actualWhenExpressionsInTaskRun); d != "" {
		t.Errorf("expected to see When Expressions %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "c-task",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "aResultValue",
			Operator: "in",
			Values:   []string{"missing"},
		}},
	}}
	if d := cmp.Diff(actualSkippedTasks, expectedSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	skippedTasks := []string{"c-task"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

func TestReconcileWithWhenExpressionsScopedToTask(t *testing.T) {
	//		(b)
	//		/
	//	(a) ———— (c) ———— (d)
	//		\
	//		(e) ———— (f)
	names.TestingSeed()
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				// a-task is skipped because its when expressions evaluate to false
				{
					Name:    "a-task",
					TaskRef: &v1beta1.TaskRef{Name: "a-task"},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"bar"},
					}},
				},
				// b-task is executed regardless of running after skipped a-task because when expressions are scoped to task
				{
					Name:     "b-task",
					TaskRef:  &v1beta1.TaskRef{Name: "b-task"},
					RunAfter: []string{"a-task"},
				},
				// c-task is skipped because its when expressions evaluate to false (not because it's parent a-task is skipped)
				{
					Name:    "c-task",
					TaskRef: &v1beta1.TaskRef{Name: "c-task"},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "foo",
						Operator: selection.In,
						Values:   []string{"bar"},
					}},
					RunAfter: []string{"a-task"},
				},
				// d-task is executed regardless of running after skipped parent c-task (and skipped grandparent a-task)
				// because when expressions are scoped to task
				{
					Name:     "d-task",
					TaskRef:  &v1beta1.TaskRef{Name: "d-task"},
					RunAfter: []string{"c-task"},
				},
				// e-task is attempted regardless of running after skipped a-task because when expressions are scoped to task
				// but then get skipped because of missing result references from a-task
				{
					Name:    "e-task",
					TaskRef: &v1beta1.TaskRef{Name: "e-task"},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "$(tasks.a-task.results.aResult)",
						Operator: selection.In,
						Values:   []string{"aResultValue"},
					}},
				},
				// f-task is skipped because its parent task e-task is skipped because of missing result reference from a-task
				{
					Name:     "f-task",
					TaskRef:  &v1beta1.TaskRef{Name: "f-task"},
					RunAfter: []string{"e-task"},
				},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
		},
	}}
	// initialize the pipelinerun with the skipped a-task
	prs[0].Status.SkippedTasks = append(prs[0].Status.SkippedTasks, v1beta1.SkippedTask{
		Name: "a-task",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "foo",
			Operator: selection.In,
			Values:   []string{"bar"},
		}},
	})
	// initialize the tasks used in the pipeline
	ts := []*v1beta1.Task{
		{
			ObjectMeta: baseObjectMeta("a-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Results: []v1beta1.TaskResult{{
					Name:        "aResult",
					Description: "a result",
				}},
			},
		},
		{ObjectMeta: baseObjectMeta("b-task", "foo")},
		{ObjectMeta: baseObjectMeta("c-task", "foo")},
		{ObjectMeta: baseObjectMeta("d-task", "foo")},
		{ObjectMeta: baseObjectMeta("e-task", "foo")},
		{ObjectMeta: baseObjectMeta("f-task", "foo")},
	}

	// set the scope of when expressions to task -- execution of dependent tasks is unblocked
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"scope-when-expressions-to-task": "true",
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0 \\(Failed: 0, Cancelled 0\\), Incomplete: 2, Skipped: 4",
	}
	pipelineRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", wantEvents, false)

	taskRunExists := func(taskName string, taskRunName string) {
		expectedTaskRun := &v1beta1.TaskRun{
			ObjectMeta: taskRunObjectMeta(taskRunName, "foo", "test-pipeline-run-different-service-accs",
				"test-pipeline", taskName, false),
			Spec: v1beta1.TaskRunSpec{
				TaskRef:            &v1beta1.TaskRef{Name: taskName},
				ServiceAccountName: "test-sa-0",
				Resources:          &v1beta1.TaskRunResources{},
				Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			},
		}

		actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", taskName),
			Limit:         1,
		})

		if err != nil {
			t.Fatalf("Failure to list TaskRuns %s", err)
		}
		if len(actual.Items) != 1 {
			t.Fatalf("Expected 1 TaskRun got %d", len(actual.Items))
		}
		actualTaskRun := actual.Items[0]
		if d := cmp.Diff(&actualTaskRun, expectedTaskRun, ignoreResourceVersion); d != "" {
			t.Errorf("expected to see TaskRun %v created. Diff %s", taskRunName, diff.PrintWantGot(d))
		}
	}

	taskRunExists("b-task", "test-pipeline-run-different-service-accs-b-task")
	taskRunExists("d-task", "test-pipeline-run-different-service-accs-d-task")

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1beta1.SkippedTask{{
		// its when expressions evaluate to false
		Name: "a-task",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "foo",
			Operator: "in",
			Values:   []string{"bar"},
		}},
	}, {
		// its when expressions evaluate to false
		Name: "c-task",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "foo",
			Operator: "in",
			Values:   []string{"bar"},
		}},
	}, {
		// was attempted, but has missing results references
		Name: "e-task",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "$(tasks.a-task.results.aResult)",
			Operator: "in",
			Values:   []string{"aResultValue"},
		}},
	}, {
		Name: "f-task",
	}}
	if d := cmp.Diff(expectedSkippedTasks, actualSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	// confirm that there are no taskruns created for the skipped tasks
	skippedTasks := []string{"a-task", "c-task", "e-task", "f-task"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

func TestReconcileWithWhenExpressionsScopedToTaskWitResultRefs(t *testing.T) {
	names.TestingSeed()
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				// a-task is executed and produces a result aResult with value aResultValue
				{
					Name:    "a-task",
					TaskRef: &v1beta1.TaskRef{Name: "a-task"},
				},
				// b-task is skipped because it has when expressions, with result reference to a-task, that evaluate to false
				{
					Name:    "b-task",
					TaskRef: &v1beta1.TaskRef{Name: "b-task"},
					WhenExpressions: []v1beta1.WhenExpression{{
						Input:    "$(tasks.a-task.results.aResult)",
						Operator: selection.In,
						Values:   []string{"notResultValue"},
					}},
				},
				// c-task is executed regardless of running after skipped b-task because when expressions are scoped to task
				{
					Name:     "c-task",
					TaskRef:  &v1beta1.TaskRef{Name: "c-task"},
					RunAfter: []string{"b-task"},
				},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
		},
	}}
	ts := []*v1beta1.Task{
		{
			ObjectMeta: baseObjectMeta("a-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Results: []v1beta1.TaskResult{{
					Name:        "aResult",
					Description: "a result",
				}},
			},
		},
		{ObjectMeta: baseObjectMeta("b-task", "foo")},
		{ObjectMeta: baseObjectMeta("c-task", "foo")},
	}
	trs := []*v1beta1.TaskRun{{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task-xxyyy", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "a-task",
			true),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world"},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "aResult",
					Value: "aResultValue",
				}},
			},
		},
	}}
	// set the scope of when expressions to task -- execution of dependent tasks is unblocked
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"scope-when-expressions-to-task": "true",
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 1 \\(Failed: 0, Cancelled 0\\), Incomplete: 1, Skipped: 1",
	}
	pipelineRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", wantEvents, false)

	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=c-task,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRuns %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRun got %d", len(actual.Items))
	}

	actualSkippedTasks := pipelineRun.Status.SkippedTasks
	expectedSkippedTasks := []v1beta1.SkippedTask{{
		// its when expressions evaluate to false
		Name: "b-task",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "aResultValue",
			Operator: "in",
			Values:   []string{"notResultValue"},
		}},
	}}
	if d := cmp.Diff(expectedSkippedTasks, actualSkippedTasks); d != "" {
		t.Errorf("expected to find Skipped Tasks %v. Diff %s", expectedSkippedTasks, diff.PrintWantGot(d))
	}

	// confirm that there are no taskruns created for the skipped tasks
	skippedTasks := []string{"b-task"}
	for _, skippedTask := range skippedTasks {
		labelSelector := fmt.Sprintf("tekton.dev/pipelineTask=%s,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs", skippedTask)
		actualSkippedTask, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
			Limit:         1,
		})
		if err != nil {
			t.Fatalf("Failure to list TaskRun's %s", err)
		}
		if len(actualSkippedTask.Items) != 0 {
			t.Fatalf("Expected 0 TaskRuns got %d", len(actualSkippedTask.Items))
		}
	}
}

// TestReconcileWithAffinityAssistantStatefulSet tests that given a pipelineRun with workspaces,
// an Affinity Assistant StatefulSet is created for each PVC workspace and
// that the Affinity Assistant names is propagated to TaskRuns.
func TestReconcileWithAffinityAssistantStatefulSet(t *testing.T) {
	workspaceName := "ws1"
	workspaceName2 := "ws2"
	emptyDirWorkspace := "emptyDirWorkspace"
	pipelineRunName := "test-pipeline-run"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{
				{Name: workspaceName},
				{Name: workspaceName2},
				{Name: emptyDirWorkspace},
			},
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceName,
					}},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceName2,
					}},
				},
				{
					Name: "hello-world-3",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: emptyDirWorkspace,
					}},
				},
			},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(pipelineRunName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: workspaceName,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myclaim",
						},
						Spec: corev1.PersistentVolumeClaimSpec{},
					},
				},
				{
					Name: workspaceName2,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myclaim2",
						},
						Spec: corev1.PersistentVolumeClaimSpec{},
					},
				},
				{
					Name:     emptyDirWorkspace,
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	// Check that the expected StatefulSet was created
	stsNames := make([]string, 0)
	for _, a := range clients.Kube.Actions() {
		if ca, ok := a.(ktesting.CreateAction); ok {
			obj := ca.GetObject()
			if sts, ok := obj.(*appsv1.StatefulSet); ok {
				stsNames = append(stsNames, sts.Name)
			}
		}
	}

	if len(stsNames) != 2 {
		t.Fatalf("expected one StatefulSet created. %d was created", len(stsNames))
	}

	expectedAffinityAssistantName1 := getAffinityAssistantName(workspaceName, pipelineRunName)
	expectedAffinityAssistantName2 := getAffinityAssistantName(workspaceName2, pipelineRunName)
	expectedAffinityAssistantStsNames := make(map[string]bool)
	expectedAffinityAssistantStsNames[expectedAffinityAssistantName1] = true
	expectedAffinityAssistantStsNames[expectedAffinityAssistantName2] = true
	for _, stsName := range stsNames {
		_, found := expectedAffinityAssistantStsNames[stsName]
		if !found {
			t.Errorf("unexpected StatefulSet created, named %s", stsName)
		}
	}

	taskRuns, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error when listing TaskRuns: %v", err)
	}

	if len(taskRuns.Items) != 3 {
		t.Errorf("expected two TaskRuns created. %d was created", len(taskRuns.Items))
	}

	taskRunsWithPropagatedAffinityAssistantName := 0
	for _, tr := range taskRuns.Items {
		for _, ws := range tr.Spec.Workspaces {
			propagatedAffinityAssistantName := tr.Annotations["pipeline.tekton.dev/affinity-assistant"]
			if ws.PersistentVolumeClaim != nil {

				if propagatedAffinityAssistantName != expectedAffinityAssistantName1 && propagatedAffinityAssistantName != expectedAffinityAssistantName2 {
					t.Fatalf("found taskRun with PVC workspace, but with unexpected AffinityAssistantAnnotation value; expected %s or %s, got %s", expectedAffinityAssistantName1, expectedAffinityAssistantName2, propagatedAffinityAssistantName)
				}
				taskRunsWithPropagatedAffinityAssistantName++
			}

			if ws.PersistentVolumeClaim == nil {
				if propagatedAffinityAssistantName != "" {
					t.Fatalf("found taskRun workspace that is not PVC workspace, but with unexpected AffinityAssistantAnnotation; expected NO AffinityAssistantAnnotation, got %s", propagatedAffinityAssistantName)
				}
			}
		}
	}

	if taskRunsWithPropagatedAffinityAssistantName != 2 {
		t.Errorf("expected only one of two TaskRuns to have Affinity Assistant affinity. %d was detected", taskRunsWithPropagatedAffinityAssistantName)
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

// TestReconcileWithVolumeClaimTemplateWorkspace tests that given a pipeline with volumeClaimTemplate workspace,
// a PVC is created and that the workspace appears as a PersistentVolumeClaim workspace for TaskRuns.
func TestReconcileWithVolumeClaimTemplateWorkspace(t *testing.T) {
	workspaceName := "ws1"
	claimName := "myclaim"
	pipelineRunName := "test-pipeline-run"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceName,
					}},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
				},
			},
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
				Name: workspaceName,
			}},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(pipelineRunName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Workspaces: []v1beta1.WorkspaceBinding{{
				Name: workspaceName,
				VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name: claimName,
					},
					Spec: corev1.PersistentVolumeClaimSpec{},
				},
			}},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	// Check that the expected PVC was created
	pvcNames := make([]string, 0)
	for _, a := range clients.Kube.Actions() {
		if ca, ok := a.(ktesting.CreateAction); ok {
			obj := ca.GetObject()
			if pvc, ok := obj.(*corev1.PersistentVolumeClaim); ok {
				pvcNames = append(pvcNames, pvc.Name)
			}
		}
	}

	if len(pvcNames) != 1 {
		t.Errorf("expected one PVC created. %d was created", len(pvcNames))
	}

	for _, pr := range prs {
		for _, w := range pr.Spec.Workspaces {
			expectedPVCName := volumeclaim.GetPersistentVolumeClaimName(&corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: w.VolumeClaimTemplate.Name,
				},
			}, w, *kmeta.NewControllerRef(pr))
			_, err := clients.Kube.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(prt.TestAssets.Ctx, expectedPVCName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("expected PVC %s to exist but instead got error when getting it: %v", expectedPVCName, err)
			}
		}
	}

	taskRuns, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error when listing TaskRuns: %v", err)
	}

	for _, tr := range taskRuns.Items {
		for _, ws := range tr.Spec.Workspaces {
			if ws.VolumeClaimTemplate != nil {
				t.Fatalf("found volumeClaimTemplate workspace. Did not expect to find any taskruns with volumeClaimTemplate workspaces")
			}

			if ws.PersistentVolumeClaim == nil {
				t.Fatalf("found taskRun workspace that is not PersistentVolumeClaim workspace. Did only expect PersistentVolumeClaims workspaces")
			}
		}
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

// TestReconcileWithVolumeClaimTemplateWorkspaceUsingSubPaths tests that given a pipeline with volumeClaimTemplate workspace and
// multiple instances of the same task, but using different subPaths in the volume - is seen as taskRuns with expected subPaths.
func TestReconcileWithVolumeClaimTemplateWorkspaceUsingSubPaths(t *testing.T) {
	workspaceName := "ws1"
	workspaceNameWithSubPath := "ws2"
	subPath1 := "customdirectory"
	subPath2 := "otherdirecory"
	pipelineRunWsSubPath := "mypath"
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Workspaces: []v1beta1.PipelineWorkspaceDeclaration{
				{Name: workspaceName},
				{Name: workspaceNameWithSubPath},
			},
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceName,
						SubPath:   subPath1,
					}},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceName,
						SubPath:   subPath2,
					}},
				},
				{
					Name: "hello-world-3",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceName,
					}},
				},
				{
					Name: "hello-world-4",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceNameWithSubPath,
					}},
				},
				{
					Name: "hello-world-5",
					TaskRef: &v1beta1.TaskRef{
						Name: "hello-world",
					},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "taskWorkspaceName",
						Workspace: workspaceNameWithSubPath,
						SubPath:   subPath1,
					}},
				},
			},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: workspaceName,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myclaim",
						},
						Spec: corev1.PersistentVolumeClaimSpec{},
					},
				},
				{
					Name:    workspaceNameWithSubPath,
					SubPath: pipelineRunWsSubPath,
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						ObjectMeta: metav1.ObjectMeta{
							Name: "myclaim",
						},
						Spec: corev1.PersistentVolumeClaimSpec{},
					},
				},
			},
		},
	}}
	ts := []*v1beta1.Task{simpleHelloWorldTask}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run", []string{}, false)

	taskRuns, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("unexpected error when listing TaskRuns: %v", err)
	}

	if len(taskRuns.Items) != 5 {
		t.Fatalf("unexpected number of taskRuns found, expected 2, but found %d", len(taskRuns.Items))
	}

	hasSeenWorkspaceWithPipelineTaskSubPath1 := false
	hasSeenWorkspaceWithPipelineTaskSubPath2 := false
	hasSeenWorkspaceWithEmptyPipelineTaskSubPath := false
	hasSeenWorkspaceWithRunSubPathAndEmptyPipelineTaskSubPath := false
	hasSeenWorkspaceWithRunSubPathAndPipelineTaskSubPath1 := false
	for _, tr := range taskRuns.Items {
		for _, ws := range tr.Spec.Workspaces {

			if ws.PersistentVolumeClaim == nil {
				t.Fatalf("found taskRun workspace that is not PersistentVolumeClaim workspace. Did only expect PersistentVolumeClaims workspaces")
			}

			if ws.SubPath == subPath1 {
				hasSeenWorkspaceWithPipelineTaskSubPath1 = true
			}

			if ws.SubPath == subPath2 {
				hasSeenWorkspaceWithPipelineTaskSubPath2 = true
			}

			if ws.SubPath == "" {
				hasSeenWorkspaceWithEmptyPipelineTaskSubPath = true
			}

			if ws.SubPath == pipelineRunWsSubPath {
				hasSeenWorkspaceWithRunSubPathAndEmptyPipelineTaskSubPath = true
			}

			if ws.SubPath == fmt.Sprintf("%s/%s", pipelineRunWsSubPath, subPath1) {
				hasSeenWorkspaceWithRunSubPathAndPipelineTaskSubPath1 = true
			}
		}
	}

	if !hasSeenWorkspaceWithPipelineTaskSubPath1 {
		t.Fatalf("did not see a taskRun with a workspace using pipelineTask subPath1")
	}

	if !hasSeenWorkspaceWithPipelineTaskSubPath2 {
		t.Fatalf("did not see a taskRun with a workspace using pipelineTask subPath2")
	}

	if !hasSeenWorkspaceWithEmptyPipelineTaskSubPath {
		t.Fatalf("did not see a taskRun with a workspace using empty pipelineTask subPath")
	}

	if !hasSeenWorkspaceWithRunSubPathAndEmptyPipelineTaskSubPath {
		t.Fatalf("did not see a taskRun with workspace using empty pipelineTask subPath and a subPath from pipelineRun")
	}

	if !hasSeenWorkspaceWithRunSubPathAndPipelineTaskSubPath1 {
		t.Fatalf("did not see a taskRun with workspace using pipelineTaks subPath1 and a subPath from pipelineRun")
	}

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileWithTaskResults(t *testing.T) {
	names.TestingSeed()
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "a-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "a-task",
					},
				},
				{
					Name: "b-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "b-task",
					},
					Params: []v1beta1.Param{{
						Name:  "bParam",
						Value: *v1beta1.NewArrayOrString("$(tasks.a-task.results.aResult)"),
					}},
				},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
		},
	}}
	ts := []*v1beta1.Task{
		{ObjectMeta: baseObjectMeta("a-task", "foo")},
		{
			ObjectMeta: baseObjectMeta("b-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "bParam",
					Type: v1beta1.ParamTypeString,
				}},
			},
		},
	}
	trs := []*v1beta1.TaskRun{{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task-xxyyy", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "a-task",
			true),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world"},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "aResult",
					Value: "aResultValue",
				}},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	expectedTaskRunName := "test-pipeline-run-different-service-accs-b-task"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta(expectedTaskRunName, "foo", "test-pipeline-run-different-service-accs",
			"test-pipeline", "b-task", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "b-task"},
			ServiceAccountName: "test-sa-0",
			Params: []v1beta1.Param{{
				Name:  "bParam",
				Value: *v1beta1.NewArrayOrString("aResultValue"),
			}},
			Resources: &v1beta1.TaskRunResources{},
			Timeout:   &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}
	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=b-task,tekton.dev/pipelineRun=test-pipeline-run-different-service-accs",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(&actualTaskRun, expectedTaskRun, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}
}

func TestReconcileWithTaskResultsEmbeddedNoneStarted(t *testing.T) {
	names.TestingSeed()
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "foo",
					Type: v1beta1.ParamTypeString,
				}},
				Tasks: []v1beta1.PipelineTask{
					{
						Name:    "a-task",
						TaskRef: &v1beta1.TaskRef{Name: "a-task"},
					},
					{
						Name:    "b-task",
						TaskRef: &v1beta1.TaskRef{Name: "b-task"},
						Params: []v1beta1.Param{{
							Name:  "bParam",
							Value: *v1beta1.NewArrayOrString("$(params.foo)/baz@$(tasks.a-task.results.A_RESULT)"),
						}},
					},
				},
			},
			Params: []v1beta1.Param{{
				Name:  "foo",
				Value: *v1beta1.NewArrayOrString("bar"),
			}},
			ServiceAccountName: "test-sa-0",
		},
	}}
	ts := []*v1beta1.Task{
		{
			ObjectMeta: baseObjectMeta("a-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Results: []v1beta1.TaskResult{{
					Name: "A_RESULT",
				}},
			},
		},
		{
			ObjectMeta: baseObjectMeta("b-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "bParam",
					Type: v1beta1.ParamTypeString,
				}},
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Tasks:        ts,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun to be running, but condition status is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Since b-task is dependent on a-task, via the results, only a-task should run
	expectedTaskRunName := "test-pipeline-run-different-service-accs-a-task"
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta(expectedTaskRunName, "foo", "test-pipeline-run-different-service-accs",
			"test-pipeline-run-different-service-accs", "a-task", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "a-task",
				Kind: v1beta1.NamespacedTaskKind,
			},
			ServiceAccountName: "test-sa-0",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}
	// Check that the expected TaskRun was created (only)
	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(expectedTaskRun, &actualTaskRun, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}
}

func TestReconcileWithPipelineResults(t *testing.T) {
	names.TestingSeed()
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "a-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "a-task",
					},
				},
				{
					Name: "b-task",
					TaskRef: &v1beta1.TaskRef{
						Name: "b-task",
					},
					Params: []v1beta1.Param{{
						Name:  "bParam",
						Value: *v1beta1.NewArrayOrString("$(tasks.a-task.results.aResult)"),
					}},
				},
			},
			Results: []v1beta1.PipelineResult{{
				Name:        "result",
				Value:       "$(tasks.a-task.results.aResult)",
				Description: "pipeline result",
			}},
		},
	}}
	trs := []*v1beta1.TaskRun{{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-different-service-accs-a-task", "foo",
			"test-pipeline-run-different-service-accs", "test-pipeline", "a-task",
			true),
		Spec: v1beta1.TaskRunSpec{
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world"},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{
					Name:  "aResult",
					Value: "aResultValue",
				}},
			},
		},
	}}
	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-different-service-accs", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionTrue,
						Reason:  v1beta1.PipelineRunReasonSuccessful.String(),
						Message: "All Tasks have completed executing",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				PipelineResults: []v1beta1.PipelineRunResult{{
					Name:  "result",
					Value: "aResultValue",
				}},
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					trs[0].Name: {
						PipelineTaskName: "a-task",
						Status:           &trs[0].Status,
					},
				},
				StartTime:      &metav1.Time{Time: now.AddDate(0, 0, -1)},
				CompletionTime: &metav1.Time{Time: now},
			},
		},
	}}
	ts := []*v1beta1.Task{
		{ObjectMeta: baseObjectMeta("a-task", "foo")},
		{
			ObjectMeta: baseObjectMeta("b-task", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "bParam",
					Type: v1beta1.ParamTypeString,
				}},
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, _ := prt.reconcileRun("foo", "test-pipeline-run-different-service-accs", []string{}, false)

	if d := cmp.Diff(&reconciledRun, &prs[0], ignoreResourceVersion); d != "" {
		t.Errorf("expected to see pipeline run results created. Diff %s", diff.PrintWantGot(d))
	}
}

func Test_storePipelineSpec(t *testing.T) {
	labels := map[string]string{"lbl": "value"}
	annotations := map[string]string{"io.annotation": "value"}
	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "foo",
			Labels:      labels,
			Annotations: annotations,
		},
	}

	ps := v1beta1.PipelineSpec{Description: "foo-pipeline"}
	ps1 := v1beta1.PipelineSpec{Description: "bar-pipeline"}

	want := pr.DeepCopy()
	want.Status = v1beta1.PipelineRunStatus{
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			PipelineSpec: ps.DeepCopy(),
		},
	}
	want.ObjectMeta.Labels["tekton.dev/pipeline"] = pr.ObjectMeta.Name

	// The first time we set it, it should get copied.
	if err := storePipelineSpecAndMergeMeta(pr, &ps, &pr.ObjectMeta); err != nil {
		t.Errorf("storePipelineSpec() error = %v", err)
	}
	if d := cmp.Diff(pr, want); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}

	// The next time, it should not get overwritten
	if err := storePipelineSpecAndMergeMeta(pr, &ps1, &metav1.ObjectMeta{}); err != nil {
		t.Errorf("storePipelineSpec() error = %v", err)
	}
	if d := cmp.Diff(pr, want); d != "" {
		t.Fatalf(diff.PrintWantGot(d))
	}
}

func TestReconcileOutOfSyncPipelineRun(t *testing.T) {
	// It may happen that a PipelineRun creates one or more TaskRuns during reconcile
	// but it fails to sync the update on the status back. This test verifies that
	// the reconciler is able to coverge back to a consistent state with the orphaned
	// TaskRuns back in the PipelineRun status.
	// For more details, see https://github.com/tektoncd/pipeline/issues/2558
	prOutOfSyncName := "test-pipeline-run-out-of-sync"
	helloWorldTask := simpleHelloWorldTask

	// Condition checks for the third task
	prccs3 := make(map[string]*v1beta1.PipelineRunConditionCheckStatus)
	conditionCheckName3 := prOutOfSyncName + "-hello-world-3-always-true"
	prccs3[conditionCheckName3] = &v1beta1.PipelineRunConditionCheckStatus{
		ConditionName: "always-true-0",
		Status: &v1beta1.ConditionCheckStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}
	// Condition checks for the fourth task
	prccs4 := make(map[string]*v1beta1.PipelineRunConditionCheckStatus)
	conditionCheckName4 := prOutOfSyncName + "-hello-world-4-always-true"
	prccs4[conditionCheckName4] = &v1beta1.PipelineRunConditionCheckStatus{
		ConditionName: "always-true-0",
		Status: &v1beta1.ConditionCheckStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}
	testPipeline := &v1beta1.Pipeline{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "hello-world-1",
					TaskRef: &v1beta1.TaskRef{
						Name: helloWorldTask.Name,
					},
				},
				{
					Name: "hello-world-2",
					TaskRef: &v1beta1.TaskRef{
						Name: helloWorldTask.Name,
					},
				},
				{
					Name: "hello-world-3",
					TaskRef: &v1beta1.TaskRef{
						Name: helloWorldTask.Name,
					},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "always-true",
					}},
				},
				{
					Name: "hello-world-4",
					TaskRef: &v1beta1.TaskRef{
						Name: helloWorldTask.Name,
					},
					Conditions: []v1beta1.PipelineTaskCondition{{
						ConditionRef: "always-true",
					}},
				},
				{
					Name:    "hello-world-5",
					TaskRef: &v1beta1.TaskRef{APIVersion: "example.dev/v0", Kind: "Example"},
				},
			},
		},
	}

	// This taskrun is in the pipelinerun status. It completed successfully.
	taskRunDone := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-out-of-sync-hello-world-1", "foo", prOutOfSyncName, testPipeline.Name, "hello-world-1", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	// This taskrun is *not* in the pipelinerun status. It's still running.
	taskRunOrphaned := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-out-of-sync-hello-world-2", "foo", prOutOfSyncName, testPipeline.Name, "hello-world-2", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	// This taskrun has a condition attached. The condition is in the pipelinerun, but the taskrun
	// itself is *not* in the pipelinerun status. It's still running.
	taskRunWithCondition := &v1beta1.TaskRun{
		ObjectMeta: taskRunObjectMeta("test-pipeline-run-out-of-sync-hello-world-3", "foo", prOutOfSyncName, testPipeline.Name, "hello-world-3", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	taskRunForConditionOfOrphanedTaskRunObjectMeta := taskRunObjectMeta(conditionCheckName3, "foo", prOutOfSyncName, testPipeline.Name, "hello-world-3", false)
	taskRunForConditionOfOrphanedTaskRunObjectMeta.Labels[pipeline.ConditionCheckKey] = conditionCheckName3
	taskRunForConditionOfOrphanedTaskRunObjectMeta.Labels[pipeline.ConditionNameKey] = "always-true"

	taskRunForConditionOfOrphanedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: taskRunForConditionOfOrphanedTaskRunObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "always-true-0",
			},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	// This taskrun has a condition attached. The condition is *not* the in pipelinerun, and it's still
	// running. The taskrun itself was not created yet.
	taskRunWithOrphanedConditionName := "test-pipeline-run-out-of-sync-hello-world-4"
	taskRunForOrphanedConditionObjectMeta := taskRunObjectMeta(conditionCheckName4, "foo", prOutOfSyncName, testPipeline.Name, "hello-world-4", false)
	taskRunForOrphanedConditionObjectMeta.Labels[pipeline.ConditionCheckKey] = conditionCheckName4
	taskRunForOrphanedConditionObjectMeta.Labels[pipeline.ConditionNameKey] = "always-true"

	taskRunForOrphanedCondition := &v1beta1.TaskRun{
		ObjectMeta: taskRunForOrphanedConditionObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "always-true-0",
			},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	orphanedRun := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-out-of-sync-hello-world-5",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "PipelineRun",
				Name: prOutOfSyncName,
			}},
			Labels: map[string]string{
				pipeline.PipelineLabelKey:     testPipeline.Name,
				pipeline.PipelineRunLabelKey:  prOutOfSyncName,
				pipeline.PipelineTaskLabelKey: "hello-world-5",
			},
			Annotations: map[string]string{},
		},
		Spec: v1alpha1.RunSpec{
			Ref: &v1beta1.TaskRef{
				APIVersion: "example.dev/v0",
				Kind:       "Example",
			},
		},
		Status: v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	prOutOfSync := &v1beta1.PipelineRun{
		ObjectMeta: baseObjectMeta(prOutOfSyncName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: testPipeline.Name},
			ServiceAccountName: "test-sa",
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  corev1.ConditionUnknown,
						Reason:  "",
						Message: "",
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
					taskRunDone.Name: {
						PipelineTaskName: "hello-world-1",
						Status:           &v1beta1.TaskRunStatus{},
					},
					taskRunWithCondition.Name: {
						PipelineTaskName: "hello-world-3",
						Status:           nil,
						ConditionChecks:  prccs3,
					},
				},
			},
		},
	}
	prs := []*v1beta1.PipelineRun{prOutOfSync}
	ps := []*v1beta1.Pipeline{testPipeline}
	ts := []*v1beta1.Task{helloWorldTask}
	trs := []*v1beta1.TaskRun{taskRunDone, taskRunOrphaned, taskRunWithCondition,
		taskRunForOrphanedCondition, taskRunForConditionOfOrphanedTaskRun}
	runs := []*v1alpha1.Run{orphanedRun}
	cs := []*v1alpha1.Condition{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "always-true",
			Namespace: "foo",
		},
		Spec: v1alpha1.ConditionSpec{
			Check: v1alpha1.Step{
				Container: corev1.Container{
					Image: "foo",
					Args:  []string{"bar"},
				},
			},
		},
	}}

	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"enable-custom-tasks": "true",
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
		Conditions:   cs,
		Runs:         runs,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", prOutOfSync.Name, []string{}, false)

	actions := clients.Pipeline.Actions()
	if len(actions) < 3 {
		t.Fatalf("Expected client to have at least three action implementation but it has %d", len(actions))
	}

	_ = getPipelineRunUpdates(t, actions)
	pipelineUpdates := 0
	for _, action := range actions {
		if action != nil {
			switch {
			case action.Matches("create", "taskruns"):
				t.Errorf("Expected client to not have created a TaskRun, but it did")
			case action.Matches("create", "runs"):
				t.Errorf("Expected client to not have created a Run, but it did")
			case action.Matches("update", "pipelineruns"):
				pipelineUpdates++
			case action.Matches("patch", "pipelineruns"):
				pipelineUpdates++
			default:
				continue
			}
		}
	}

	// We actually expect three update calls because the first status update fails due to
	// optimistic concurrency (due to the label update) and is retried after reloading via
	// the client.
	if got, want := pipelineUpdates, 3; got != want {
		// If only the pipelinerun status changed, we expect one update
		t.Fatalf("Expected client to have updated the pipelinerun %d times, but it did %d times", want, got)
	}

	// This PipelineRun should still be running and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be running, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	expectedTaskRunsStatus := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
	expectedRunsStatus := make(map[string]*v1beta1.PipelineRunRunStatus)
	// taskRunDone did not change
	expectedTaskRunsStatus[taskRunDone.Name] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-1",
		Status: &v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
	// taskRunOrphaned was recovered into the status
	expectedTaskRunsStatus[taskRunOrphaned.Name] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-2",
		Status: &v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}
	// taskRunWithCondition was recovered into the status. The condition did not change.
	expectedTaskRunsStatus[taskRunWithCondition.Name] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-3",
		Status: &v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
		ConditionChecks: prccs3,
	}
	// taskRunWithOrphanedConditionName had the condition recovered into the status. No taskrun.
	expectedTaskRunsStatus[taskRunWithOrphanedConditionName] = &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-4",
		ConditionChecks:  prccs4,
	}
	// orphanedRun was recovered into the status
	expectedRunsStatus[orphanedRun.Name] = &v1beta1.PipelineRunRunStatus{
		PipelineTaskName: "hello-world-5",
		Status: &v1alpha1.RunStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}

	if d := cmp.Diff(reconciledRun.Status.TaskRuns, expectedTaskRunsStatus); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", d)
	}
	if d := cmp.Diff(reconciledRun.Status.Runs, expectedRunsStatus); d != "" {
		t.Fatalf("Expected PipelineRun status to match Run(s) status, but got a mismatch: %s", d)
	}
}

func TestUpdatePipelineRunStatusFromInformer(t *testing.T) {
	names.TestingSeed()

	pr := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run",
			Namespace: "foo",
			Labels:    map[string]string{"mylabel": "myvale"},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "unit-test-task-spec",
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: v1beta1.TaskSpec{
							Steps: []v1beta1.Step{{Container: corev1.Container{
								Name:  "mystep",
								Image: "myimage"}}},
						},
					},
				}},
			},
		},
	}

	d := test.Data{
		PipelineRuns: []*v1beta1.PipelineRun{pr},
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}

	// Reconcile the PipelineRun.  This creates a Taskrun.
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run", wantEvents, false)

	// Save the name of the TaskRun that was created.
	taskRunName := ""
	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Fatalf("Expected 1 TaskRun but got %d", len(reconciledRun.Status.TaskRuns))
	}
	for k := range reconciledRun.Status.TaskRuns {
		taskRunName = k
		break
	}

	// Add a label to the PipelineRun.  This tests a scenario in issue 3126 which could prevent the reconciler
	// from finding TaskRuns that are missing from the status.
	reconciledRun.ObjectMeta.Labels["bah"] = "humbug"
	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").Update(prt.TestAssets.Ctx, reconciledRun, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("unexpected error when updating status: %v", err)
	}

	// The label update triggers another reconcile.  Depending on timing, the PipelineRun passed to the reconcile may or may not
	// have the updated status with the name of the created TaskRun.  Clear the status because we want to test the case where the
	// status does not have the TaskRun.
	reconciledRun.Status = v1beta1.PipelineRunStatus{}
	if _, err := clients.Pipeline.TektonV1beta1().PipelineRuns("foo").UpdateStatus(prt.TestAssets.Ctx, reconciledRun, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("unexpected error when updating status: %v", err)
	}

	reconciledRun, _ = prt.reconcileRun("foo", "test-pipeline-run", wantEvents, false)

	// Verify that the reconciler found the existing TaskRun instead of creating a new one.
	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Fatalf("Expected 1 TaskRun after label change but got %d", len(reconciledRun.Status.TaskRuns))
	}
	for k := range reconciledRun.Status.TaskRuns {
		if k != taskRunName {
			t.Fatalf("Status has unexpected taskrun %s", k)
		}
	}
}

func TestUpdatePipelineRunStatusFromTaskRuns(t *testing.T) {

	prUID := types.UID("11111111-1111-1111-1111-111111111111")
	otherPrUID := types.UID("22222222-2222-2222-2222-222222222222")

	// PipelineRunConditionCheckStatus recovered by updatePipelineRunStatusFromTaskRuns
	// It does not include the status, which is then retrieved via the regular reconcile
	prccs2Recovered := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-2-running-condition-check-xxyyy": {
			ConditionName: "running-condition-0",
		},
	}
	prccs3Recovered := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-3-successful-condition-check-xxyyy": {
			ConditionName: "successful-condition-0",
		},
	}
	prccs4Recovered := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-4-failed-condition-check-xxyyy": {
			ConditionName: "failed-condition-0",
		},
	}

	// PipelineRunConditionCheckStatus full is used to test the behaviour of updatePipelineRunStatusFromTaskRuns
	// when no orphan TaskRuns are found, to check we don't alter good ones
	prccs2Full := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-2-running-condition-check-xxyyy": {
			ConditionName: "running-condition-0",
			Status: &v1beta1.ConditionCheckStatus{
				ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
					Check: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionUnknown}},
				},
			},
		},
	}
	prccs3Full := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-3-successful-condition-check-xxyyy": {
			ConditionName: "successful-condition-0",
			Status: &v1beta1.ConditionCheckStatus{
				ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
					Check: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}},
				},
			},
		},
	}
	prccs4Full := map[string]*v1beta1.PipelineRunConditionCheckStatus{
		"pr-task-4-failed-condition-check-xxyyy": {
			ConditionName: "failed-condition-0",
			Status: &v1beta1.ConditionCheckStatus{
				ConditionCheckStatusFields: v1beta1.ConditionCheckStatusFields{
					Check: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 127},
					},
				},
				Status: duckv1beta1.Status{
					Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}},
				},
			},
		},
	}

	prRunningStatus := duckv1beta1.Status{
		Conditions: []apis.Condition{
			{
				Type:    "Succeeded",
				Status:  "Unknown",
				Reason:  "Running",
				Message: "Not all Tasks in the Pipeline have finished executing",
			},
		},
	}

	prStatusWithCondition := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
				"pr-task-1-xxyyy": {
					PipelineTaskName: "task-1",
					Status:           &v1beta1.TaskRunStatus{},
				},
				"pr-task-2-xxyyy": {
					PipelineTaskName: "task-2",
					Status:           nil,
					ConditionChecks:  prccs2Full,
				},
				"pr-task-3-xxyyy": {
					PipelineTaskName: "task-3",
					Status:           &v1beta1.TaskRunStatus{},
					ConditionChecks:  prccs3Full,
				},
				"pr-task-4-xxyyy": {
					PipelineTaskName: "task-4",
					Status:           nil,
					ConditionChecks:  prccs4Full,
				},
			},
		},
	}

	prStatusWithEmptyTaskRuns := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{},
		},
	}

	prStatusWithOrphans := v1beta1.PipelineRunStatus{
		Status: duckv1beta1.Status{
			Conditions: []apis.Condition{
				{
					Type:    "Succeeded",
					Status:  "Unknown",
					Reason:  "Running",
					Message: "Not all Tasks in the Pipeline have finished executing",
				},
			},
		},
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{},
		},
	}

	prStatusRecovered := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
				"pr-task-1-xxyyy": {
					PipelineTaskName: "task-1",
					Status:           &v1beta1.TaskRunStatus{},
				},
				"orphaned-taskruns-pr-task-2-xxyyy": {
					PipelineTaskName: "task-2",
					Status:           nil,
					ConditionChecks:  prccs2Recovered,
				},
				"pr-task-3-xxyyy": {
					PipelineTaskName: "task-3",
					Status:           &v1beta1.TaskRunStatus{},
					ConditionChecks:  prccs3Recovered,
				},
				"orphaned-taskruns-pr-task-4-xxyyy": {
					PipelineTaskName: "task-4",
					Status:           nil,
					ConditionChecks:  prccs4Recovered,
				},
			},
		},
	}

	prStatusRecoveredSimple := v1beta1.PipelineRunStatus{
		Status: prRunningStatus,
		PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
			TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
				"pr-task-1-xxyyy": {
					PipelineTaskName: "task-1",
					Status:           &v1beta1.TaskRunStatus{},
				},
			},
		},
	}

	allTaskRuns := []*v1beta1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-1",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-2-running-condition-check-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-2",
					pipeline.ConditionCheckKey:    "pr-task-2-running-condition-check-xxyyy",
					pipeline.ConditionNameKey:     "running-condition",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-3-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-3",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-3-successful-condition-check-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-3",
					pipeline.ConditionCheckKey:    "pr-task-3-successful-condition-check-xxyyy",
					pipeline.ConditionNameKey:     "successful-condition",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-4-failed-condition-check-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-4",
					pipeline.ConditionCheckKey:    "pr-task-4-failed-condition-check-xxyyy",
					pipeline.ConditionNameKey:     "failed-condition",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
			},
		},
	}

	taskRunsFromAnotherPR := []*v1beta1.TaskRun{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pr-task-1-xxyyy",
				Labels: map[string]string{
					pipeline.PipelineTaskLabelKey: "task-1",
				},
				OwnerReferences: []metav1.OwnerReference{{UID: otherPrUID}},
			},
		},
	}

	tcs := []struct {
		prName           string
		prStatus         v1beta1.PipelineRunStatus
		trs              []*v1beta1.TaskRun
		expectedPrStatus v1beta1.PipelineRunStatus
	}{
		{
			prName:           "no-status-no-taskruns",
			prStatus:         v1beta1.PipelineRunStatus{},
			trs:              nil,
			expectedPrStatus: v1beta1.PipelineRunStatus{},
		}, {
			prName:           "status-no-taskruns",
			prStatus:         prStatusWithCondition,
			trs:              nil,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:   "status-nil-taskruns",
			prStatus: prStatusWithEmptyTaskRuns,
			trs: []*v1beta1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-task-1-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "task-1",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
			},
			expectedPrStatus: prStatusRecoveredSimple,
		}, {
			prName:   "status-missing-taskruns",
			prStatus: prStatusWithCondition,
			trs: []*v1beta1.TaskRun{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-task-3-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "task-3",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pr-task-3-successful-condition-check-xxyyy",
						Labels: map[string]string{
							pipeline.PipelineTaskLabelKey: "task-3",
							pipeline.ConditionCheckKey:    "pr-task-3-successful-condition-check-xxyyy",
							pipeline.ConditionNameKey:     "successful-condition",
						},
						OwnerReferences: []metav1.OwnerReference{{UID: prUID}},
					},
				},
			},
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:           "status-matching-taskruns-pr",
			prStatus:         prStatusWithCondition,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusWithCondition,
		}, {
			prName:           "orphaned-taskruns-pr",
			prStatus:         prStatusWithOrphans,
			trs:              allTaskRuns,
			expectedPrStatus: prStatusRecovered,
		}, {
			prName:           "tr-from-another-pr",
			prStatus:         prStatusWithEmptyTaskRuns,
			trs:              taskRunsFromAnotherPR,
			expectedPrStatus: prStatusWithEmptyTaskRuns,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.prName, func(t *testing.T) {
			logger := logtesting.TestLogger(t)

			pr := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: tc.prName, UID: prUID},
				Status:     tc.prStatus,
			}

			updatePipelineRunStatusFromTaskRuns(logger, pr, tc.trs)
			actualPrStatus := pr.Status

			// The TaskRun keys for recovered taskruns will contain a new random key, appended to the
			// base name that we expect. Replace the random part so we can diff the whole structure
			actualTaskRuns := actualPrStatus.PipelineRunStatusFields.TaskRuns
			if actualTaskRuns != nil {
				fixedTaskRuns := make(map[string]*v1beta1.PipelineRunTaskRunStatus)
				re := regexp.MustCompile(`^[a-z\-]*?-task-[0-9]`)
				for k, v := range actualTaskRuns {
					newK := re.FindString(k)
					fixedTaskRuns[newK+"-xxyyy"] = v
				}
				actualPrStatus.PipelineRunStatusFields.TaskRuns = fixedTaskRuns
			}

			if d := cmp.Diff(tc.expectedPrStatus, actualPrStatus); d != "" {
				t.Errorf("expected the PipelineRun status to match %#v. Diff %s", tc.expectedPrStatus, diff.PrintWantGot(d))
			}
		})
	}
}

func TestReconcilePipeline_FinalTasks(t *testing.T) {
	tests := []struct {
		name                     string
		pipelineRunName          string
		prs                      []*v1beta1.PipelineRun
		ps                       []*v1beta1.Pipeline
		ts                       []*v1beta1.Task
		trs                      []*v1beta1.TaskRun
		expectedTaskRuns         map[string]*v1beta1.PipelineRunTaskRunStatus
		pipelineRunStatusUnknown bool
		pipelineRunStatusFalse   bool
	}{{
		// pipeline run should result in error when a dag task is executed and resulted in failure but final task is executed successfully

		// pipelineRunName - "pipeline-run-dag-task-failing"
		// pipelineName - "pipeline-dag-task-failing"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task" and "task-run-final-task"
		// taskName - "hello-world"

		name: "Test 01 - Pipeline run should result in error when a dag task fails but final task is executed successfully.",

		pipelineRunName: "pipeline-run-dag-task-failing",

		prs: getPipelineRun(
			"pipeline-run-dag-task-failing",
			"pipeline-dag-task-failing",
			corev1.ConditionFalse,
			v1beta1.PipelineRunReasonFailed.String(),
			"Tasks Completed: 2 (Failed: 1, Cancelled 0), Skipped: 0",
			map[string]string{
				"dag-task-1":   "task-run-dag-task",
				"final-task-1": "task-run-final-task",
			},
		),

		ps: getPipeline(
			"pipeline-dag-task-failing",
			v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1beta1.Task{simpleHelloWorldTask},

		trs: []*v1beta1.TaskRun{
			getTaskRun(
				"task-run-dag-task",
				"pipeline-run-dag-task-failing",
				"pipeline-dag-task-failing",
				"dag-task-1",
				corev1.ConditionFalse,
			),
			getTaskRun(
				"task-run-final-task",
				"pipeline-run-dag-task-failing",
				"pipeline-dag-task-failing",
				"final-task-1",
				"",
			),
		},

		expectedTaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"task-run-dag-task":   getTaskRunStatus("dag-task-1", corev1.ConditionFalse),
			"task-run-final-task": getTaskRunStatus("final-task-1", ""),
		},

		pipelineRunStatusFalse: true,
	}, {

		// pipeline run should result in error when a dag task is successful but the final task fails

		// pipelineRunName - "pipeline-run-with-dag-successful-but-final-failing"
		// pipelineName - "pipeline-with-dag-successful-but-final-failing"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task" and "task-run-final-task"
		// taskName - "hello-world"

		name: "Test 02 - Pipeline run should result in error when a dag task is successful but final task fails.",

		pipelineRunName: "pipeline-run-with-dag-successful-but-final-failing",

		prs: getPipelineRun(
			"pipeline-run-with-dag-successful-but-final-failing",
			"pipeline-with-dag-successful-but-final-failing",
			corev1.ConditionFalse,
			v1beta1.PipelineRunReasonFailed.String(),
			"Tasks Completed: 2 (Failed: 1, Cancelled 0), Skipped: 0",
			map[string]string{
				"dag-task-1":   "task-run-dag-task",
				"final-task-1": "task-run-final-task",
			},
		),

		ps: getPipeline(
			"pipeline-with-dag-successful-but-final-failing",
			v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1beta1.Task{simpleHelloWorldTask},

		trs: []*v1beta1.TaskRun{
			getTaskRun(
				"task-run-dag-task",
				"pipeline-run-with-dag-successful-but-final-failing",
				"pipeline-with-dag-successful-but-final-failing",
				"dag-task-1",
				"",
			),
			getTaskRun(
				"task-run-final-task",
				"pipeline-run-with-dag-successful-but-final-failing",
				"pipeline-with-dag-successful-but-final-failing",
				"final-task-1",
				corev1.ConditionFalse,
			),
		},

		expectedTaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"task-run-dag-task":   getTaskRunStatus("dag-task-1", ""),
			"task-run-final-task": getTaskRunStatus("final-task-1", corev1.ConditionFalse),
		},

		pipelineRunStatusFalse: true,
	}, {

		// pipeline run should result in error when a dag task and final task both are executed and resulted in failure

		// pipelineRunName - "pipeline-run-with-dag-and-final-failing"
		// pipelineName - "pipeline-with-dag-and-final-failing"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task" and "task-run-final-task"
		// taskName - "hello-world"

		name: "Test 03 - Pipeline run should result in error when both dag task and final task fail.",

		pipelineRunName: "pipeline-run-with-dag-and-final-failing",

		prs: getPipelineRun(
			"pipeline-run-with-dag-and-final-failing",
			"pipeline-with-dag-and-final-failing",
			corev1.ConditionFalse,
			v1beta1.PipelineRunReasonFailed.String(),
			"Tasks Completed: 2 (Failed: 2, Cancelled 0), Skipped: 0",
			map[string]string{
				"dag-task-1":   "task-run-dag-task",
				"final-task-1": "task-run-final-task",
			},
		),

		ps: getPipeline(
			"pipeline-with-dag-and-final-failing",
			v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1beta1.Task{simpleHelloWorldTask},

		trs: []*v1beta1.TaskRun{
			getTaskRun(
				"task-run-dag-task",
				"pipeline-run-with-dag-and-final-failing",
				"pipeline-with-dag-and-final-failing",
				"dag-task-1",
				corev1.ConditionFalse,
			),
			getTaskRun(
				"task-run-final-task",
				"pipeline-run-with-dag-and-final-failing",
				"pipeline-with-dag-and-final-failing",
				"final-task-1",
				corev1.ConditionFalse,
			),
		},

		expectedTaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"task-run-dag-task":   getTaskRunStatus("dag-task-1", corev1.ConditionFalse),
			"task-run-final-task": getTaskRunStatus("final-task-1", corev1.ConditionFalse),
		},

		pipelineRunStatusFalse: true,
	}, {

		// pipeline run should not schedule final tasks until dag tasks are done i.e.
		// dag task 1 fails but dag task 2 is still running, pipeline run should not schedule and create task run for final task

		// pipelineRunName - "pipeline-run-with-dag-running"
		// pipelineName - "pipeline-with-dag-running"
		// pipelineTasks - "dag-task-1", "dag-task-2" and "final-task-1"
		// taskRunNames - "task-run-dag-task-1" and "task-run-dag-task-2" - no task run for final task
		// taskName - "hello-world"

		name: "Test 04 - Pipeline run should not schedule final tasks while dag tasks are still running.",

		pipelineRunName: "pipeline-run-with-dag-running",

		prs: getPipelineRun(
			"pipeline-run-with-dag-running",
			"pipeline-with-dag-running",
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonRunning.String(),
			"Tasks Completed: 1 (Failed: 1, Cancelled 0), Incomplete: 2, Skipped: 0",
			map[string]string{
				"dag-task-1": "task-run-dag-task-1",
				"dag-task-2": "task-run-dag-task-2",
			},
		),

		ps: getPipeline(
			"pipeline-with-dag-running",
			v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{
					{
						Name:    "dag-task-1",
						TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
					},
					{
						Name:    "dag-task-2",
						TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
					},
				},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1beta1.Task{simpleHelloWorldTask},

		trs: []*v1beta1.TaskRun{
			getTaskRun(
				"task-run-dag-task-1",
				"pipeline-run-with-dag-running",
				"pipeline-with-dag-running",
				"dag-task-1",
				corev1.ConditionFalse,
			),
			getTaskRun(
				"task-run-dag-task-2",
				"pipeline-run-with-dag-running",
				"pipeline-with-dag-running",
				"dag-task-2",
				corev1.ConditionUnknown,
			),
		},

		expectedTaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"task-run-dag-task-1": getTaskRunStatus("dag-task-1", corev1.ConditionFalse),
			"task-run-dag-task-2": getTaskRunStatus("dag-task-2", corev1.ConditionUnknown),
		},

		pipelineRunStatusUnknown: true,
	}, {

		// pipeline run should not schedule final tasks until dag tasks are done i.e.
		// dag task is still running and no other dag task available to schedule,
		// pipeline run should not schedule and create task run for final task

		// pipelineRunName - "pipeline-run-dag-task-running"
		// pipelineName - "pipeline-dag-task-running"
		// pipelineTasks - "dag-task-1" and "final-task-1"
		// taskRunNames - "task-run-dag-task-1" - no task run for final task
		// taskName - "hello-world"

		name: "Test 05 - Pipeline run should not schedule final tasks while dag tasks are still running and no other dag task available to schedule.",

		pipelineRunName: "pipeline-run-dag-task-running",

		prs: getPipelineRun(
			"pipeline-run-dag-task-running",
			"pipeline-dag-task-running",
			corev1.ConditionUnknown,
			v1beta1.PipelineRunReasonRunning.String(),
			"Tasks Completed: 0 (Failed: 0, Cancelled 0), Incomplete: 1, Skipped: 0",
			map[string]string{
				"dag-task-1": "task-run-dag-task-1",
			},
		),

		ps: getPipeline(
			"pipeline-dag-task-running",
			v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name:    "dag-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
				Finally: []v1beta1.PipelineTask{{
					Name:    "final-task-1",
					TaskRef: &v1beta1.TaskRef{Name: "hello-world"},
				}},
			}),

		ts: []*v1beta1.Task{simpleHelloWorldTask},

		trs: []*v1beta1.TaskRun{
			getTaskRun(
				"task-run-dag-task-1",
				"pipeline-run-dag-task-running",
				"pipeline-dag-task-running",
				"dag-task-1",
				corev1.ConditionUnknown,
			),
		},

		expectedTaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{
			"task-run-dag-task-1": getTaskRunStatus("dag-task-1", corev1.ConditionUnknown),
		},

		pipelineRunStatusUnknown: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := test.Data{
				PipelineRuns: tt.prs,
				Pipelines:    tt.ps,
				Tasks:        tt.ts,
				TaskRuns:     tt.trs,
			}
			prt := newPipelineRunTest(d, t)
			defer prt.Cancel()

			reconciledRun, clients := prt.reconcileRun("foo", tt.pipelineRunName, []string{}, false)

			actions := clients.Pipeline.Actions()
			if len(actions) < 2 {
				t.Fatalf("Expected client to have at least two action implementation but it has %d", len(actions))
			}

			// The first update action should be updating the PipelineRun.
			var actual *v1beta1.PipelineRun
			for _, action := range actions {
				if actualPrime, ok := action.(ktesting.UpdateAction); ok {
					actual = actualPrime.GetObject().(*v1beta1.PipelineRun)
					break
				}
			}

			if actual == nil {
				t.Errorf("Expected a PipelineRun to be updated, but it wasn't for %s", tt.name)
			}

			for _, action := range actions {
				if action != nil {
					resource := action.GetResource().Resource
					if resource == "taskruns" {
						t.Fatalf("Expected client to not have created a TaskRun for the PipelineRun, but it did for %s", tt.name)
					}
				}
			}

			if tt.pipelineRunStatusFalse {
				// This PipelineRun should still be failed and the status should reflect that
				if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
					t.Errorf("Expected PipelineRun status to be failed, but was %v for %s",
						reconciledRun.Status.GetCondition(apis.ConditionSucceeded), tt.name)
				}
			} else if tt.pipelineRunStatusUnknown {
				// This PipelineRun should still be running and the status should reflect that
				if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
					t.Errorf("Expected PipelineRun status to be unknown (running), but was %v for %s",
						reconciledRun.Status.GetCondition(apis.ConditionSucceeded), tt.name)
				}
			}

			if d := cmp.Diff(reconciledRun.Status.TaskRuns, tt.expectedTaskRuns); d != "" {
				t.Fatalf("Expected PipelineRunTaskRun status to match TaskRun(s) status, but got a mismatch for %s: %s", tt.name, d)
			}

		})
	}
}

func getPipelineRun(pr, p string, status corev1.ConditionStatus, reason string, m string, tr map[string]string) []*v1beta1.PipelineRun {
	pRun := &v1beta1.PipelineRun{
		ObjectMeta: baseObjectMeta(pr, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: p},
			ServiceAccountName: "test-sa",
		},
		Status: v1beta1.PipelineRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:    apis.ConditionSucceeded,
						Status:  status,
						Reason:  reason,
						Message: m,
					},
				},
			},
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{TaskRuns: map[string]*v1beta1.PipelineRunTaskRunStatus{}},
		},
	}
	for k, v := range tr {
		pRun.Status.TaskRuns[v] = &v1beta1.PipelineRunTaskRunStatus{PipelineTaskName: k, Status: &v1beta1.TaskRunStatus{}}
	}
	return []*v1beta1.PipelineRun{pRun}
}

func getPipeline(p string, spec v1beta1.PipelineSpec) []*v1beta1.Pipeline {
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta(p, "foo"),
		Spec:       spec,
	}}
	return ps
}

func getTaskRun(tr, pr, p, t string, status corev1.ConditionStatus) *v1beta1.TaskRun {
	return createHelloWorldTaskRunWithStatusTaskLabel(tr, "foo", pr, p, "", t,
		apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: status,
		})
}

func getTaskRunStatus(t string, status corev1.ConditionStatus) *v1beta1.PipelineRunTaskRunStatus {
	return &v1beta1.PipelineRunTaskRunStatus{
		PipelineTaskName: t,
		Status: &v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					{Type: apis.ConditionSucceeded, Status: status},
				},
			},
		},
	}
}

// TestReconcile_CloudEvents runs reconcile with a cloud event sink configured
// to ensure that events are sent in different cases
func TestReconcile_CloudEvents(t *testing.T) {
	names.TestingSeed()

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipelinerun",
			Namespace: "foo",
			SelfLink:  "/pipeline/1234",
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
		},
	}}
	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "test-1",
				TaskRef: &v1beta1.TaskRef{
					Name: "test-task",
				},
			}},
		},
	}}
	ts := []*v1beta1.Task{{
		ObjectMeta: baseObjectMeta("test-task", "foo"),
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{
					Image:   "foo",
					Name:    "simple-step",
					Command: []string{"/mycmd"},
					Env: []corev1.EnvVar{{
						Name:  "foo",
						Value: "bar",
					}},
				},
			}},
		},
	}}
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetDefaultsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"default-cloud-events-sink": "http://synk:8080",
			},
		},
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		ConfigMaps:   cms,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipelinerun", wantEvents, false)

	// This PipelineRun is in progress now and the status should reflect that
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %v", condition)
	}
	if condition != nil && condition.Reason != v1beta1.PipelineRunReasonRunning.String() {
		t.Errorf("Expected reason %q but was %s", v1beta1.PipelineRunReasonRunning.String(), condition.Reason)
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to include the TaskRun status items that can run immediately: %v", reconciledRun.Status.TaskRuns)
	}

	wantCloudEvents := []string{
		`(?s)dev.tekton.event.pipelinerun.started.v1.*test-pipelinerun`,
		`(?s)dev.tekton.event.pipelinerun.running.v1.*test-pipelinerun`,
	}
	ceClient := clients.CloudEvents.(cloudevent.FakeClient)
	err := eventstest.CheckEventsUnordered(t, ceClient.Events, "reconcile-cloud-events", wantCloudEvents)
	if !(err == nil) {
		t.Errorf(err.Error())
	}
}

// this test validates taskSpec metadata is embedded into task run
func TestReconcilePipeline_TaskSpecMetadata(t *testing.T) {
	names.TestingSeed()

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta("test-pipeline-run-success", "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "test-pipeline"},
		},
	}}

	ts := v1beta1.TaskSpec{
		Steps: []v1beta1.Step{{Container: corev1.Container{
			Name:  "mystep",
			Image: "myimage"}}},
	}

	labels := map[string]string{"label1": "labelvalue1", "label2": "labelvalue2"}
	annotations := map[string]string{"annotation1": "value1", "annotation2": "value2"}

	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta("test-pipeline", "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{
				{
					Name: "task-without-metadata",
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: ts,
					},
				},
				{
					Name: "task-with-metadata",
					TaskSpec: &v1beta1.EmbeddedTask{
						TaskSpec: ts,
						Metadata: v1beta1.PipelineTaskMetadata{
							Labels:      labels,
							Annotations: annotations,
						},
					},
				},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-success", []string{}, false)

	actions := clients.Pipeline.Actions()
	if len(actions) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	actualTaskRun := make(map[string]*v1beta1.TaskRun)
	for _, a := range actions {
		if a.GetResource().Resource == "taskruns" {
			t := a.(ktesting.CreateAction).GetObject().(*v1beta1.TaskRun)
			actualTaskRun[t.Name] = t
		}
	}

	// Check that the expected TaskRun was created
	if len(actualTaskRun) != 2 {
		t.Errorf("Expected two TaskRuns to be created, but found %d TaskRuns.", len(actualTaskRun))
	}

	expectedTaskRun := make(map[string]*v1beta1.TaskRun)
	expectedTaskRun["test-pipeline-run-success-task-with-metadata"] = getTaskRunWithTaskSpec(
		"test-pipeline-run-success-task-with-metadata",
		"test-pipeline-run-success",
		"test-pipeline",
		"task-with-metadata",
		labels,
		annotations,
	)

	expectedTaskRun["test-pipeline-run-success-task-without-metadata"] = getTaskRunWithTaskSpec(
		"test-pipeline-run-success-task-without-metadata",
		"test-pipeline-run-success",
		"test-pipeline",
		"task-without-metadata",
		map[string]string{},
		map[string]string{},
	)

	if d := cmp.Diff(actualTaskRun, expectedTaskRun); d != "" {
		t.Fatalf("Expected TaskRuns to match, but got a mismatch: %s", d)
	}

	if len(reconciledRun.Status.TaskRuns) != 2 {
		t.Errorf("Expected PipelineRun status to include both TaskRun status items that can run immediately: %v", reconciledRun.Status.TaskRuns)
	}
}

func TestReconciler_ReconcileKind_PipelineTaskContext(t *testing.T) {
	names.TestingSeed()

	pipelineName := "p-pipelinetask-status"
	pipelineRunName := "pr-pipelinetask-status"

	ps := []*v1beta1.Pipeline{{
		ObjectMeta: baseObjectMeta(pipelineName, "foo"),
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name: "task1",
				TaskRef: &v1beta1.TaskRef{
					Name: "mytask",
				},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name: "finaltask",
				TaskRef: &v1beta1.TaskRef{
					Name: "finaltask",
				},
				Params: []v1beta1.Param{{
					Name:  "pipelineRun-tasks-task1",
					Value: *v1beta1.NewArrayOrString("$(tasks.task1.status)"),
				}},
			}},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: baseObjectMeta(pipelineRunName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: pipelineName},
			ServiceAccountName: "test-sa",
		},
	}}

	ts := []*v1beta1.Task{
		{ObjectMeta: baseObjectMeta("mytask", "foo")},
		{
			ObjectMeta: baseObjectMeta("finaltask", "foo"),
			Spec: v1beta1.TaskSpec{
				Params: []v1beta1.ParamSpec{{
					Name: "pipelineRun-tasks-task1",
					Type: v1beta1.ParamTypeString,
				}},
			},
		},
	}

	trs := []*v1beta1.TaskRun{{
		ObjectMeta: taskRunObjectMeta(pipelineRunName+"-task1-xxyy", "foo", pipelineRunName, pipelineName, "task1", false),
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "mytask",
			},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{
					apis.Condition{
						Type:   apis.ConditionSucceeded,
						Status: corev1.ConditionFalse,
						Reason: v1beta1.TaskRunReasonFailed.String(),
					},
				},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	_, clients := prt.reconcileRun("foo", pipelineRunName, []string{}, false)

	expectedTaskRunName := pipelineRunName + "-finaltask"
	expectedTaskRunObjectMeta := taskRunObjectMeta(expectedTaskRunName, "foo", pipelineRunName, pipelineName, "finaltask", false)
	expectedTaskRunObjectMeta.Labels[pipeline.MemberOfLabelKey] = v1beta1.PipelineFinallyTasks
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: expectedTaskRunObjectMeta,
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "finaltask",
			},
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			Params: []v1beta1.Param{{
				Name:  "pipelineRun-tasks-task1",
				Value: *v1beta1.NewArrayOrString("Failed"),
			}},
		},
	}
	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=finaltask,tekton.dev/pipelineRun=" + pipelineRunName,
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(&actualTaskRun, expectedTaskRun, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}
}

func TestReconcileWithTaskResultsInFinalTasks(t *testing.T) {
	names.TestingSeed()

	ps := []*v1beta1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline", Namespace: "foo"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "dag-task-1",
				TaskRef: &v1beta1.TaskRef{Name: "dag-task"},
			}, {
				Name:    "dag-task-2",
				TaskRef: &v1beta1.TaskRef{Name: "dag-task"},
			}},
			Finally: []v1beta1.PipelineTask{{
				Name:    "final-task-1",
				TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				Params: []v1beta1.Param{{
					Name: "finalParam",
					Value: v1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "$(tasks.dag-task-1.results.aResult)",
					}},
				},
			}, {
				Name:    "final-task-2",
				TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				Params: []v1beta1.Param{{
					Name: "finalParam",
					Value: v1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "$(tasks.dag-task-2.results.aResult)",
					}},
				},
			}, {
				// final task skipped because when expressions evaluated to false
				Name:    "final-task-3",
				TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				Params: []v1beta1.Param{{
					Name: "finalParam",
					Value: v1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "param",
					}},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(tasks.dag-task-1.results.aResult)",
					Operator: selection.NotIn,
					Values:   []string{"aResultValue"},
				}},
			}, {
				// final task executed because when expressions evaluated to true
				Name:    "final-task-4",
				TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				Params: []v1beta1.Param{{
					Name: "finalParam",
					Value: v1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "param",
					}},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(tasks.dag-task-1.results.aResult)",
					Operator: selection.In,
					Values:   []string{"aResultValue"},
				}},
			}, {
				// final task skipped because of missing result reference in when expressions
				Name:    "final-task-5",
				TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				Params: []v1beta1.Param{{
					Name: "finalParam",
					Value: v1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "param",
					}},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(tasks.dag-task-2.results.aResult)",
					Operator: selection.NotIn,
					Values:   []string{"aResultValue"},
				}},
			}, {
				// final task skipped because of missing result reference in params
				// even though its when expressions has a valid result that would have evaluated to true
				Name:    "final-task-6",
				TaskRef: &v1beta1.TaskRef{Name: "final-task"},
				Params: []v1beta1.Param{{
					Name: "finalParam",
					Value: v1beta1.ArrayOrString{
						Type:      "string",
						StringVal: "$(tasks.dag-task-2.results.aResult)",
					}},
				},
				WhenExpressions: v1beta1.WhenExpressions{{
					Input:    "$(tasks.dag-task-1.results.aResult)",
					Operator: selection.In,
					Values:   []string{"aResultValue"},
				}},
			}},
		},
	}}

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-final-task-results", Namespace: "foo"},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa-0",
		},
	}}

	ts := []*v1beta1.Task{{
		ObjectMeta: metav1.ObjectMeta{Name: "dag-task", Namespace: "foo"},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "final-task", Namespace: "foo"},
		Spec: v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "finalParam",
				Type: v1beta1.ParamTypeString,
			}},
		},
	}}

	trs := []*v1beta1.TaskRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-final-task-results-dag-task-1-xxyyy",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				Kind:       "PipelineRun",
				Name:       "test-pipeline-run-final-task-results",
				APIVersion: "tekton.dev/v1beta1",
			}},
			Labels: map[string]string{
				"tekton.dev/pipeline":     "test-pipeline",
				"tekton.dev/pipelineRun":  "test-pipeline-run-final-task-results",
				"tekton.dev/pipelineTask": "dag-task-1",
			},
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: "test-sa",
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world"},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: v1beta1.PipelineRunReasonSuccessful.String(),
				}},
			},
			TaskRunStatusFields: v1beta1.TaskRunStatusFields{
				TaskRunResults: []v1beta1.TaskRunResult{{Name: "aResult", Value: "aResultValue"}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-final-task-results-dag-task-2-xxyyy",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				Kind:       "PipelineRun",
				Name:       "test-pipeline-run-final-task-results",
				APIVersion: "tekton.dev/v1beta1",
			}},
			Labels: map[string]string{
				"tekton.dev/pipeline":     "test-pipeline",
				"tekton.dev/pipelineRun":  "test-pipeline-run-final-task-results",
				"tekton.dev/pipelineTask": "dag-task-2",
			},
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: "test-sa",
			TaskRef:            &v1beta1.TaskRef{Name: "hello-world"},
		},
		Status: v1beta1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: duckv1beta1.Conditions{apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}
	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-final-task-results", []string{}, false)

	expectedTaskRunName := "test-pipeline-run-final-task-results-final-task-1"
	expectedTaskRun := v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      expectedTaskRunName,
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				Kind:               "PipelineRun",
				Name:               "test-pipeline-run-final-task-results",
				APIVersion:         "tekton.dev/v1beta1",
				Controller:         &trueb,
				BlockOwnerDeletion: &trueb,
			}},
			Labels: map[string]string{
				"tekton.dev/pipeline":     "test-pipeline",
				"tekton.dev/pipelineRun":  "test-pipeline-run-final-task-results",
				"tekton.dev/pipelineTask": "final-task-1",
				pipeline.MemberOfLabelKey: v1beta1.PipelineFinallyTasks,
			},
			Annotations: map[string]string{},
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: "test-sa-0",
			TaskRef:            &v1beta1.TaskRef{Name: "final-task"},
			Params:             []v1beta1.Param{{Name: "finalParam", Value: v1beta1.ArrayOrString{Type: "string", StringVal: "aResultValue"}}},
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}

	// Check that the expected TaskRun was created
	actual, err := clients.Pipeline.TektonV1beta1().TaskRuns("foo").List(prt.TestAssets.Ctx, metav1.ListOptions{
		LabelSelector: "tekton.dev/pipelineTask=final-task-1,tekton.dev/pipelineRun=test-pipeline-run-final-task-results",
		Limit:         1,
	})

	if err != nil {
		t.Fatalf("Failure to list TaskRun's %s", err)
	}
	if len(actual.Items) != 1 {
		t.Fatalf("Expected 1 TaskRuns got %d", len(actual.Items))
	}
	actualTaskRun := actual.Items[0]
	if d := cmp.Diff(expectedTaskRun, actualTaskRun, ignoreResourceVersion); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRunName, diff.PrintWantGot(d))
	}
	expectedSkippedTasks := []v1beta1.SkippedTask{{
		Name: "final-task-2",
	}, {
		Name: "final-task-3",
		WhenExpressions: v1beta1.WhenExpressions{{
			Input:    "aResultValue",
			Operator: "notin",
			Values:   []string{"aResultValue"},
		}},
	}, {
		Name: "final-task-5",
	}, {
		Name: "final-task-6",
	}}

	if d := cmp.Diff(expectedSkippedTasks, reconciledRun.Status.SkippedTasks); d != "" {
		t.Fatalf("Didn't get the expected list of skipped tasks. Diff: %s", diff.PrintWantGot(d))
	}
}

// newPipelineRunTest returns PipelineRunTest with a new PipelineRun controller created with specified state through data
// This PipelineRunTest can be reused for multiple PipelineRuns by calling reconcileRun for each pipelineRun
func newPipelineRunTest(data test.Data, t *testing.T) *PipelineRunTest {
	t.Helper()
	testAssets, cancel := getPipelineRunController(t, data)
	return &PipelineRunTest{
		Data:       data,
		Test:       t,
		TestAssets: testAssets,
		Cancel:     cancel,
	}
}

func (prt PipelineRunTest) reconcileRun(namespace, pipelineRunName string, wantEvents []string, permanentError bool) (*v1beta1.PipelineRun, test.Clients) {
	prt.Test.Helper()
	c := prt.TestAssets.Controller
	clients := prt.TestAssets.Clients

	reconcileError := c.Reconciler.Reconcile(prt.TestAssets.Ctx, namespace+"/"+pipelineRunName)
	if permanentError {
		// When a PipelineRun is invalid and can't run, we expect a permanent error that will
		// tell the Reconciler to not keep trying to reconcile.
		if reconcileError == nil {
			prt.Test.Fatalf("Expected an error to be returned by Reconcile, got nil instead")
		}
		if controller.IsPermanentError(reconcileError) != permanentError {
			prt.Test.Fatalf("Expected the error to be permanent: %v but got: %v", permanentError, controller.IsPermanentError(reconcileError))
		}
	} else if ok, _ := controller.IsRequeueKey(reconcileError); ok {
		// This is normal, it happens for timeouts when we otherwise successfully reconcile.
	} else if reconcileError != nil {
		prt.Test.Fatalf("Error reconciling: %s", reconcileError)
	}
	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.TektonV1beta1().PipelineRuns(namespace).Get(prt.TestAssets.Ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		prt.Test.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// Check generated events match what's expected
	if len(wantEvents) > 0 {
		if err := eventstest.CheckEventsOrdered(prt.Test, prt.TestAssets.Recorder.Events, pipelineRunName, wantEvents); err != nil {
			prt.Test.Errorf(err.Error())
		}
	}

	return reconciledRun, clients
}

func TestReconcile_RemotePipelineRef(t *testing.T) {
	names.TestingSeed()

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	cfg.OnConfigChanged(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName()},
		Data: map[string]string{
			"enable-tekton-oci-bundles": "true",
		},
	})
	ctx = cfg.ToContext(ctx)

	// Set up a fake registry to push an image to.
	s := httptest.NewServer(registry.New())
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	ref := u.Host + "/testreconcile_remotepipelineref"

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-success", Namespace: "foo"},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "test-sa",
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline", Bundle: ref},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}}
	ps := &v1beta1.Pipeline{
		TypeMeta:   metav1.TypeMeta{APIVersion: "tekton.dev/v1beta1", Kind: "Pipeline"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline", Namespace: "foo"},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: &v1beta1.TaskRef{Name: "unit-test-task", Bundle: ref}},
			},
		},
	}
	cms := []*corev1.ConfigMap{
		{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				"enable-tekton-oci-bundles": "true",
			},
		},
	}

	// This task will be uploaded along with the pipeline definition.
	remoteTask := &v1beta1.Task{
		ObjectMeta: baseObjectMeta("unit-test-task", "foo"),
		TypeMeta: metav1.TypeMeta{
			APIVersion: "tekton.dev/v1beta1",
			Kind:       "Task",
		},
		Spec: v1beta1.TaskSpec{},
	}

	// Create a bundle from our pipeline and tasks.
	if _, err := test.CreateImage(ref, ps, remoteTask); err != nil {
		t.Fatalf("failed to create image in pipeline renconcile: %s", err.Error())
	}

	// Unlike the tests above, we do *not* locally define our pipeline or unit-test task.
	d := test.Data{
		PipelineRuns: prs,
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: prs[0].Spec.ServiceAccountName, Namespace: "foo"},
		}},
		ConfigMaps: cms,
	}

	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	wantEvents := []string{
		"Normal Started",
		"Normal Running Tasks Completed: 0",
	}
	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-success", wantEvents, false)

	if len(clients.Pipeline.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, clients.Pipeline.Actions())[0]
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pipeline-run-success-unit-test-1",
			Namespace:   "foo",
			Annotations: map[string]string{},
			Labels: map[string]string{
				"tekton.dev/pipeline":         "test-pipeline",
				"tekton.dev/pipelineRun":      "test-pipeline-run-success",
				pipeline.PipelineTaskLabelKey: "unit-test-1",
				pipeline.MemberOfLabelKey:     v1beta1.PipelineTasks,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "tekton.dev/v1beta1",
				Kind:               "PipelineRun",
				Name:               "test-pipeline-run-success",
				Controller:         &trueb,
				BlockOwnerDeletion: &trueb,
			}},
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			TaskRef: &v1beta1.TaskRef{
				Kind:   "Task",
				Name:   "unit-test-task",
				Bundle: ref,
			},
		},
	}

	if d := cmp.Diff(expectedTaskRun, actual, cmpopts.SortSlices(func(x, y v1beta1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// This PipelineRun is in progress now and the status should reflect that
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %v", condition)
	}
	if condition != nil && condition.Reason != v1beta1.PipelineRunReasonRunning.String() {
		t.Errorf("Expected reason %q but was %s", v1beta1.PipelineRunReasonRunning.String(), condition.Reason)
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to include the TaskRun status item that ran immediately: %v", reconciledRun.Status.TaskRuns)
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-1"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}
}

// TestReconcile_OptionalWorkspacesOmitted checks that an optional workspace declared by
// a Task and a Pipeline can be omitted by a PipelineRun and the run will still start
// successfully without an error.
func TestReconcile_OptionalWorkspacesOmitted(t *testing.T) {
	names.TestingSeed()

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	ctx = cfg.ToContext(ctx)

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pipeline-run-success", Namespace: "foo"},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "test-sa",
			PipelineSpec: &v1beta1.PipelineSpec{
				Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
					Name:     "optional-workspace",
					Optional: true,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name: "unit-test-1",
					TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
						Workspaces: []v1beta1.WorkspaceDeclaration{{
							Name:     "ws",
							Optional: true,
						}},
						Steps: []v1beta1.Step{{
							Container: corev1.Container{
								Image: "foo:latest",
							},
						}},
					}},
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "ws",
						Workspace: "optional-workspace",
					}},
				}},
			},
		},
	}}

	// Unlike the tests above, we do *not* locally define our pipeline or unit-test task.
	d := test.Data{
		PipelineRuns: prs,
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: prs[0].Spec.ServiceAccountName, Namespace: "foo"},
		}},
	}

	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	reconciledRun, clients := prt.reconcileRun("foo", "test-pipeline-run-success", nil, false)

	// Check that the expected TaskRun was created
	actual := getTaskRunCreations(t, clients.Pipeline.Actions())[0]
	expectedTaskRun := &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pipeline-run-success-unit-test-1",
			Namespace:   "foo",
			Annotations: map[string]string{},
			Labels: map[string]string{
				"tekton.dev/pipeline":         "test-pipeline-run-success",
				"tekton.dev/pipelineRun":      "test-pipeline-run-success",
				pipeline.PipelineTaskLabelKey: "unit-test-1",
				pipeline.MemberOfLabelKey:     v1beta1.PipelineTasks,
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "tekton.dev/v1beta1",
				Kind:       "PipelineRun",
				Name:       "test-pipeline-run-success", Controller: &trueb,
				BlockOwnerDeletion: &trueb,
			}},
		},
		Spec: v1beta1.TaskRunSpec{
			ServiceAccountName: "test-sa",
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
			TaskSpec: &v1beta1.TaskSpec{
				Workspaces: []v1beta1.WorkspaceDeclaration{{
					Name:     "ws",
					Optional: true,
				}},
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Image: "foo:latest",
					},
				}},
			},
		},
	}

	if d := cmp.Diff(expectedTaskRun, actual, cmpopts.SortSlices(func(x, y v1beta1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, diff.PrintWantGot(d))
	}

	// This PipelineRun is in progress now and the status should reflect that
	condition := reconciledRun.Status.GetCondition(apis.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %v", condition)
	}
	if condition != nil && condition.Reason != v1beta1.PipelineRunReasonRunning.String() {
		t.Errorf("Expected reason %q but was %s", v1beta1.PipelineRunReasonRunning.String(), condition.Reason)
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to include the TaskRun status item that ran immediately: %v", reconciledRun.Status.TaskRuns)
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-1"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}
}

func TestReconcile_DependencyValidationsImmediatelyFailPipelineRun(t *testing.T) {
	names.TestingSeed()

	ctx := context.Background()
	cfg := config.NewStore(logtesting.TestLogger(t))
	ctx = cfg.ToContext(ctx)

	prs := []*v1beta1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun-param-invalid-result-variable",
			Namespace: "foo",
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "test-sa",
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "pt0",
					TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{
								Image: "foo:latest",
							},
						}},
					}},
				}, {
					Name: "pt1",
					Params: []v1beta1.Param{{
						Name:  "p",
						Value: *v1beta1.NewArrayOrString("$(tasks.pt0.results.r1)"),
					}},
					TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
						Params: []v1beta1.ParamSpec{{
							Name: "p",
						}},
						Steps: []v1beta1.Step{{
							Container: corev1.Container{
								Image: "foo:latest",
							},
						}},
					}},
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun-pipeline-result-invalid-result-variable",
			Namespace: "foo",
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "test-sa",
			PipelineSpec: &v1beta1.PipelineSpec{
				Tasks: []v1beta1.PipelineTask{{
					Name: "pt0",
					TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{
								Image: "foo:latest",
							},
						}},
					}},
				}, {
					Name: "pt1",
					TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
						Steps: []v1beta1.Step{{
							Container: corev1.Container{
								Image: "foo:latest",
							},
						}},
					}},
				}},
				Results: []v1beta1.PipelineResult{{
					Name:  "pr",
					Value: "$(tasks.pt0.results.r)",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun-with-optional-workspace-validation",
			Namespace: "foo",
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "test-sa",
			PipelineSpec: &v1beta1.PipelineSpec{
				Workspaces: []v1beta1.PipelineWorkspaceDeclaration{{
					Name:     "optional-workspace",
					Optional: true,
				}},
				Tasks: []v1beta1.PipelineTask{{
					Name: "unit-test-1",
					Workspaces: []v1beta1.WorkspacePipelineTaskBinding{{
						Name:      "ws",
						Workspace: "optional-workspace",
					}},
					TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
						Workspaces: []v1beta1.WorkspaceDeclaration{{
							Name:     "ws",
							Optional: false,
						}},
						Steps: []v1beta1.Step{{
							Container: corev1.Container{
								Image: "foo:latest",
							},
						}},
					}},
				}},
			},
		},
	}}

	d := test.Data{
		PipelineRuns: prs,
		ServiceAccounts: []*corev1.ServiceAccount{{
			ObjectMeta: metav1.ObjectMeta{Name: prs[0].Spec.ServiceAccountName, Namespace: "foo"},
		}},
	}

	prt := newPipelineRunTest(d, t)
	defer prt.Cancel()

	run1, _ := prt.reconcileRun("foo", "pipelinerun-param-invalid-result-variable", nil, true)
	run2, _ := prt.reconcileRun("foo", "pipelinerun-pipeline-result-invalid-result-variable", nil, true)
	run3, _ := prt.reconcileRun("foo", "pipelinerun-with-optional-workspace-validation", nil, true)

	cond1 := run1.Status.GetCondition(apis.ConditionSucceeded)
	cond2 := run2.Status.GetCondition(apis.ConditionSucceeded)
	cond3 := run3.Status.GetCondition(apis.ConditionSucceeded)

	for _, c := range []*apis.Condition{cond1, cond2, cond3} {
		if c.Status != corev1.ConditionFalse {
			t.Errorf("expected Succeeded/False condition but saw: %v", c)
		}
	}

	if cond1.Reason != ReasonInvalidTaskResultReference {
		t.Errorf("expected invalid task reference condition but saw: %v", cond1)
	}

	if cond2.Reason != ReasonInvalidTaskResultReference {
		t.Errorf("expected invalid task reference condition but saw: %v", cond2)
	}

	if cond3.Reason != ReasonRequiredWorkspaceMarkedOptional {
		t.Errorf("expected optional workspace not supported condition but saw: %v", cond3)
	}
}

func getTaskRunWithTaskSpec(tr, pr, p, t string, labels, annotations map[string]string) *v1beta1.TaskRun {
	om := taskRunObjectMeta(tr, "foo", pr, p, t, false)
	for k, v := range labels {
		om.Labels[k] = v
	}
	om.Annotations = annotations

	return &v1beta1.TaskRun{
		ObjectMeta: om,
		Spec: v1beta1.TaskRunSpec{
			TaskSpec: &v1beta1.TaskSpec{
				Steps: []v1beta1.Step{{
					Container: corev1.Container{
						Name:  "mystep",
						Image: "myimage",
					},
				}},
			},
			ServiceAccountName: config.DefaultServiceAccountValue,
			Resources:          &v1beta1.TaskRunResources{},
			Timeout:            &metav1.Duration{Duration: config.DefaultTimeoutMinutes * time.Minute},
		},
	}
}

func baseObjectMeta(name, ns string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   ns,
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	}
}

func taskRunObjectMeta(trName, ns, prName, pipelineName, pipelineTaskName string, skipMemberOfLabel bool) metav1.ObjectMeta {
	om := metav1.ObjectMeta{
		Name:      trName,
		Namespace: ns,
		OwnerReferences: []metav1.OwnerReference{{
			Kind:               "PipelineRun",
			Name:               prName,
			APIVersion:         "tekton.dev/v1beta1",
			Controller:         &trueb,
			BlockOwnerDeletion: &trueb,
		}},
		Labels: map[string]string{
			pipeline.PipelineLabelKey:     pipelineName,
			pipeline.PipelineRunLabelKey:  prName,
			pipeline.PipelineTaskLabelKey: pipelineTaskName,
		},
		Annotations: map[string]string{},
	}
	if !skipMemberOfLabel {
		om.Labels[pipeline.MemberOfLabelKey] = v1beta1.PipelineTasks
	}
	return om
}

func createHelloWorldTaskRunWithStatus(trName, ns, prName, pName, podName string, condition apis.Condition) *v1beta1.TaskRun {
	p := createHelloWorldTaskRun(trName, ns, prName, pName)
	p.Status = v1beta1.TaskRunStatus{
		Status: duckv1beta1.Status{
			Conditions: duckv1beta1.Conditions{condition},
		},
		TaskRunStatusFields: v1beta1.TaskRunStatusFields{
			PodName: podName,
		},
	}
	return p
}

func createHelloWorldTaskRunWithStatusTaskLabel(trName, ns, prName, pName, podName, taskLabel string, condition apis.Condition) *v1beta1.TaskRun {
	p := createHelloWorldTaskRunWithStatus(trName, ns, prName, pName, podName, condition)
	p.Labels[pipeline.PipelineTaskLabelKey] = taskLabel

	return p
}

func createHelloWorldTaskRun(trName, ns, prName, pName string) *v1beta1.TaskRun {
	return &v1beta1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      trName,
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "kind",
				Name: "name",
			}},
			Labels: map[string]string{
				pipeline.PipelineLabelKey:    pName,
				pipeline.PipelineRunLabelKey: prName,
			},
		},
		Spec: v1beta1.TaskRunSpec{
			TaskRef: &v1beta1.TaskRef{
				Name: "hello-world",
			},
			ServiceAccountName: "test-sa",
		},
	}
}

func createCancelledPipelineRun(prName string, specStatus v1beta1.PipelineRunSpecStatus) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: baseObjectMeta(prName, "foo"),
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef:        &v1beta1.PipelineRef{Name: "test-pipeline"},
			ServiceAccountName: "test-sa",
			Status:             specStatus,
		},
		Status: v1beta1.PipelineRunStatus{
			PipelineRunStatusFields: v1beta1.PipelineRunStatusFields{
				StartTime: &metav1.Time{Time: now},
			},
		},
	}
}
