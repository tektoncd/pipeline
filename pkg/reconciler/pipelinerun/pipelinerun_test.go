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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	taskrunresources "github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktesting "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/configmap"
)

var (
	ignoreLastTransitionTime = cmpopts.IgnoreTypes(apis.Condition{}.LastTransitionTime.Inner.Time)
	images                   = pipeline.Images{
		EntrypointImage:          "override-with-entrypoint:latest",
		NopImage:                 "tianon/true",
		GitImage:                 "override-with-git:latest",
		CredsImage:               "override-with-creds:latest",
		KubeconfigWriterImage:    "override-with-kubeconfig-writer:latest",
		ShellImage:               "busybox",
		GsutilImage:              "google/cloud-sdk",
		BuildGCSFetcherImage:     "gcr.io/cloud-builders/gcs-fetcher:latest",
		PRImage:                  "override-with-pr:latest",
		ImageDigestExporterImage: "override-with-imagedigest-exporter-image:latest",
	}
)

func getRunName(pr *v1alpha1.PipelineRun) string {
	return strings.Join([]string{pr.Namespace, pr.Name}, "/")
}

// getPipelineRunController returns an instance of the PipelineRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getPipelineRunController(t *testing.T, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	c, _ := test.SeedTestData(t, ctx, d)
	configMapWatcher := configmap.NewInformedWatcher(c.Kube, system.GetNamespace())
	ctx, cancel := context.WithCancel(ctx)
	return test.Assets{
		Controller: NewController(images)(ctx, configMapWatcher),
		Clients:    c,
	}, cancel
}

// conditionCheckFromTaskRun converts takes a pointer to a TaskRun and wraps it into a ConditionCheck
func conditionCheckFromTaskRun(tr *v1alpha1.TaskRun) *v1alpha1.ConditionCheck {
	cc := v1alpha1.ConditionCheck(*tr)
	return &cc
}

func TestReconcile(t *testing.T) {
	names.TestingSeed()

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("test-pipeline-run-success", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunServiceAccountName("test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("best-image", tb.PipelineResourceBindingResourceSpec(
					&v1alpha1.PipelineResourceSpec{
						Type: v1alpha1.PipelineResourceTypeImage,
						Params: []v1alpha1.ResourceParam{{
							Name:  "url",
							Value: "gcr.io/sven",
						}},
					},
				)),
				tb.PipelineRunParam("bar", "somethingmorefun"),
			),
		),
	}
	funParam := tb.PipelineTaskParam("foo", "somethingfun")
	moreFunParam := tb.PipelineTaskParam("bar", "$(params.bar)")
	templatedParam := tb.PipelineTaskParam("templatedparam", "$(inputs.workspace.$(params.rev-param))")
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("best-image", "image"),
				tb.PipelineParamSpec("pipeline-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("somethingdifferent")),
				tb.PipelineParamSpec("rev-param", v1alpha1.ParamTypeString, tb.ParamSpecDefault("revision")),
				tb.PipelineParamSpec("bar", v1alpha1.ParamTypeString),
				// unit-test-3 uses runAfter to indicate it should run last
				tb.PipelineTask("unit-test-3", "unit-test-task",
					funParam, moreFunParam, templatedParam,
					tb.RunAfter("unit-test-2"),
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
				// unit-test-1 can run right away because it has no dependencies
				tb.PipelineTask("unit-test-1", "unit-test-task",
					funParam, moreFunParam, templatedParam,
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
				// unit-test-2 uses `from` to indicate it should run after `unit-test-1`
				tb.PipelineTask("unit-test-2", "unit-test-followup-task",
					tb.PipelineTaskInputResource("workspace", "git-repo", tb.From("unit-test-1")),
				),
				// unit-test-cluster-task can run right away because it has no dependencies
				tb.PipelineTask("unit-test-cluster-task", "unit-test-cluster-task",
					tb.PipelineTaskRefKind(v1alpha1.ClusterTaskKind),
					funParam, moreFunParam, templatedParam,
					tb.PipelineTaskInputResource("workspace", "git-repo"),
					tb.PipelineTaskOutputResource("image-to-use", "best-image"),
					tb.PipelineTaskOutputResource("workspace", "git-repo"),
				),
			),
		),
	}
	ts := []*v1alpha1.Task{
		tb.Task("unit-test-task", "foo", tb.TaskSpec(
			tb.TaskInputs(
				tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
				tb.InputsParamSpec("foo", v1alpha1.ParamTypeString), tb.InputsParamSpec("bar", v1alpha1.ParamTypeString), tb.InputsParamSpec("templatedparam", v1alpha1.ParamTypeString),
			),
			tb.TaskOutputs(
				tb.OutputsResource("image-to-use", v1alpha1.PipelineResourceTypeImage),
				tb.OutputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
			),
		)),
		tb.Task("unit-test-followup-task", "foo", tb.TaskSpec(
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
		)),
	}
	clusterTasks := []*v1alpha1.ClusterTask{
		tb.ClusterTask("unit-test-cluster-task", tb.ClusterTaskSpec(
			tb.TaskInputs(
				tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
				tb.InputsParamSpec("foo", v1alpha1.ParamTypeString), tb.InputsParamSpec("bar", v1alpha1.ParamTypeString), tb.InputsParamSpec("templatedparam", v1alpha1.ParamTypeString),
			),
			tb.TaskOutputs(
				tb.OutputsResource("image-to-use", v1alpha1.PipelineResourceTypeImage),
				tb.OutputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
			),
		)),
		tb.ClusterTask("unit-test-followup-task", tb.ClusterTaskSpec(
			tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
		)),
	}
	rs := []*v1alpha1.PipelineResource{
		tb.PipelineResource("some-repo", "foo", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeGit,
			tb.PipelineResourceSpecParam("url", "https://github.com/kristoff/reindeer"),
		)),
	}

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

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-success"); err != nil {
		t.Fatalf("Error reconciling: %s", err)
	}

	if len(clients.Pipeline.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	t.Log("actions", clients.Pipeline.Actions())

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-success", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject()
	expectedTaskRun := tb.TaskRun("test-pipeline-run-success-unit-test-1-mz4c7", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-success",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-success"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "unit-test-1"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("unit-test-task"),
			tb.TaskRunServiceAccountName("test-sa"),
			tb.TaskRunInputs(
				tb.TaskRunInputsParam("foo", "somethingfun"),
				tb.TaskRunInputsParam("bar", "somethingmorefun"),
				tb.TaskRunInputsParam("templatedparam", "$(inputs.workspace.revision)"),
				tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingRef("some-repo")),
			),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource("image-to-use", tb.TaskResourceBindingResourceSpec(
					&v1alpha1.PipelineResourceSpec{
						Type: v1alpha1.PipelineResourceTypeImage,
						Params: []v1alpha1.ResourceParam{{
							Name:  "url",
							Value: "gcr.io/sven",
						}},
					},
				),
					tb.TaskResourceBindingPaths("/pvc/unit-test-1/image-to-use"),
				),
				tb.TaskRunOutputsResource("workspace", tb.TaskResourceBindingRef("some-repo"),
					tb.TaskResourceBindingPaths("/pvc/unit-test-1/workspace"),
				),
			),
		),
	)

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, cmpopts.SortSlices(func(x, y v1alpha1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff (-want, +got): %s", expectedTaskRun, d)
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
	if condition != nil && condition.Reason != resources.ReasonRunning {
		t.Errorf("Expected reason %q but was %s", resources.ReasonRunning, condition.Reason)
	}

	if len(reconciledRun.Status.TaskRuns) != 2 {
		t.Errorf("Expected PipelineRun status to include both TaskRun status items that can run immediately: %v", reconciledRun.Status.TaskRuns)
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-1-mz4c7"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-cluster-task-78c5n"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}

	// A PVC should have been created to deal with output -> input linking
	ensurePVCCreated(t, clients, expectedTaskRun.GetPipelineRunPVCName(), "foo")
}

func TestReconcile_PipelineSpecTaskSpec(t *testing.T) {
	names.TestingSeed()

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("test-pipeline-run-success", "foo",
			tb.PipelineRunSpec("test-pipeline"),
		),
	}
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineTask("unit-test-task-spec", "", tb.PipelineTaskSpec(&v1alpha1.TaskSpec{
					Steps: []v1alpha1.Step{{Container: corev1.Container{
						Name:  "mystep",
						Image: "myimage"}}},
				})),
			),
		),
	}

	d := test.Data{
		PipelineRuns:      prs,
		Pipelines:         ps,
		Tasks:             nil,
		ClusterTasks:      nil,
		PipelineResources: nil,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-success"); err != nil {
		t.Fatalf("Error reconciling: %s", err)
	}

	if len(clients.Pipeline.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	t.Log("actions", clients.Pipeline.Actions())

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-success", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject()
	expectedTaskRun := tb.TaskRun("test-pipeline-run-success-unit-test-task-spec-9l9zj", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-success",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-success"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "unit-test-task-spec"),
		tb.TaskRunSpec(tb.TaskRunTaskSpec(tb.Step("myimage", tb.StepName("mystep")))),
	)

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(expectedTaskRun, actual, cmpopts.SortSlices(func(x, y v1alpha1.TaskSpec) bool { return len(x.Steps) == len(y.Steps) })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff (-want, +got): %s", expectedTaskRun, d)
	}

	// test taskrun is able to recreate correct pipeline-pvc-name
	if expectedTaskRun.GetPipelineRunPVCName() != "test-pipeline-run-success-pvc" {
		t.Errorf("expected to see TaskRun PVC name set to %q created but got %s", "test-pipeline-run-success-pvc", expectedTaskRun.GetPipelineRunPVCName())
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to include both TaskRun status items that can run immediately: %v", reconciledRun.Status.TaskRuns)
	}

	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-task-spec-9l9zj"]; !exists {
		t.Errorf("Expected PipelineRun status to include TaskRun status but was %v", reconciledRun.Status.TaskRuns)
	}
}

func TestReconcile_InvalidPipelineRuns(t *testing.T) {
	ts := []*v1alpha1.Task{
		tb.Task("a-task-that-exists", "foo"),
		tb.Task("a-task-that-needs-params", "foo", tb.TaskSpec(
			tb.TaskInputs(tb.InputsParamSpec("some-param", v1alpha1.ParamTypeString)))),
		tb.Task("a-task-that-needs-array-params", "foo", tb.TaskSpec(
			tb.TaskInputs(tb.InputsParamSpec("some-param", v1alpha1.ParamTypeArray)))),
		tb.Task("a-task-that-needs-a-resource", "ns", tb.TaskSpec(
			tb.TaskInputs(tb.InputsResource("workspace", "git")),
		)),
	}
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("pipeline-missing-tasks", "foo", tb.PipelineSpec(
			tb.PipelineTask("myspecialtask", "sometask"),
		)),
		tb.Pipeline("a-pipeline-without-params", "foo", tb.PipelineSpec(
			tb.PipelineTask("some-task", "a-task-that-needs-params"))),
		tb.Pipeline("a-fine-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineDeclaredResource("a-resource", v1alpha1.PipelineResourceTypeGit),
			tb.PipelineTask("some-task", "a-task-that-exists",
				tb.PipelineTaskInputResource("needed-resource", "a-resource")))),
		tb.Pipeline("a-pipeline-that-should-be-caught-by-admission-control", "foo", tb.PipelineSpec(
			tb.PipelineTask("some-task", "a-task-that-exists",
				tb.PipelineTaskInputResource("needed-resource", "a-resource")))),
		tb.Pipeline("a-pipeline-with-array-params", "foo", tb.PipelineSpec(
			tb.PipelineParamSpec("some-param", v1alpha1.ParamTypeArray),
			tb.PipelineTask("some-task", "a-task-that-needs-array-params"))),
		tb.Pipeline("a-pipeline-with-missing-conditions", "foo", tb.PipelineSpec(tb.PipelineTask("some-task", "a-task-that-exists", tb.PipelineTaskCondition("condition-does-not-exist")))),
	}
	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("invalid-pipeline", "foo", tb.PipelineRunSpec("pipeline-not-exist")),
		tb.PipelineRun("pipelinerun-missing-tasks", "foo", tb.PipelineRunSpec("pipeline-missing-tasks")),
		tb.PipelineRun("pipeline-params-dont-exist", "foo", tb.PipelineRunSpec("a-pipeline-without-params")),
		tb.PipelineRun("pipeline-resources-not-bound", "foo", tb.PipelineRunSpec("a-fine-pipeline")),
		tb.PipelineRun("pipeline-resources-dont-exist", "foo", tb.PipelineRunSpec("a-fine-pipeline",
			tb.PipelineRunResourceBinding("a-resource", tb.PipelineResourceBindingRef("missing-resource")))),
		tb.PipelineRun("pipeline-resources-not-declared", "foo", tb.PipelineRunSpec("a-pipeline-that-should-be-caught-by-admission-control")),
		tb.PipelineRun("pipeline-mismatching-param-type", "foo", tb.PipelineRunSpec("a-pipeline-with-array-params", tb.PipelineRunParam("some-param", "stringval"))),
		tb.PipelineRun("pipeline-conditions-missing", "foo", tb.PipelineRunSpec("a-pipeline-with-missing-conditions")),
		tb.PipelineRun("embedded-pipeline-resources-not-bound", "foo", tb.PipelineRunSpec("", tb.PipelineRunPipelineSpec(
			tb.PipelineTask("some-task", "a-task-that-needs-a-resource"),
			tb.PipelineDeclaredResource("workspace", "git"),
		))),
		tb.PipelineRun("embedded-pipeline-invalid", "foo", tb.PipelineRunSpec("", tb.PipelineRunPipelineSpec(
			tb.PipelineTask("bad-t@$k", "b@d-t@$k"),
		))),
		tb.PipelineRun("embedded-pipeline-mismatching-param-type", "foo", tb.PipelineRunSpec("", tb.PipelineRunPipelineSpec(
			tb.PipelineParamSpec("some-param", v1alpha1.ParamTypeArray),
			tb.PipelineTask("some-task", "a-task-that-needs-array-params")),
			tb.PipelineRunParam("some-param", "stringval"),
		)),
	}
	d := test.Data{
		Tasks:        ts,
		Pipelines:    ps,
		PipelineRuns: prs,
	}
	tcs := []struct {
		name               string
		pipelineRun        *v1alpha1.PipelineRun
		reason             string
		hasNoDefaultLabels bool
	}{
		{
			name:               "invalid-pipeline-shd-be-stop-reconciling",
			pipelineRun:        prs[0],
			reason:             ReasonCouldntGetPipeline,
			hasNoDefaultLabels: true,
		}, {
			name:        "invalid-pipeline-run-missing-tasks-shd-stop-reconciling",
			pipelineRun: prs[1],
			reason:      ReasonCouldntGetTask,
		}, {
			name:        "invalid-pipeline-run-params-dont-exist-shd-stop-reconciling",
			pipelineRun: prs[2],
			reason:      ReasonFailedValidation,
		}, {
			name:        "invalid-pipeline-run-resources-not-bound-shd-stop-reconciling",
			pipelineRun: prs[3],
			reason:      ReasonInvalidBindings,
		}, {
			name:        "invalid-pipeline-run-missing-resource-shd-stop-reconciling",
			pipelineRun: prs[4],
			reason:      ReasonCouldntGetResource,
		}, {
			name:        "invalid-pipeline-missing-declared-resource-shd-stop-reconciling",
			pipelineRun: prs[5],
			reason:      ReasonFailedValidation,
		}, {
			name:        "invalid-pipeline-mismatching-parameter-types",
			pipelineRun: prs[6],
			reason:      ReasonParameterTypeMismatch,
		}, {
			name:        "invalid-pipeline-missing-conditions-shd-stop-reconciling",
			pipelineRun: prs[7],
			reason:      ReasonCouldntGetCondition,
		}, {
			name:        "invalid-embedded-pipeline-resources-bot-bound-shd-stop-reconciling",
			pipelineRun: prs[8],
			reason:      ReasonInvalidBindings,
		}, {
			name:        "invalid-embedded-pipeline-bad-name-shd-stop-reconciling",
			pipelineRun: prs[9],
			reason:      ReasonFailedValidation,
		}, {
			name:        "invalid-embedded-pipeline-mismatching-parameter-types",
			pipelineRun: prs[10],
			reason:      ReasonParameterTypeMismatch,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets, cancel := getPipelineRunController(t, d)
			defer cancel()
			c := testAssets.Controller

			if err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.pipelineRun)); err != nil {
				t.Fatalf("Error reconciling: %s", err)
			}
			// When a PipelineRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.

			if tc.pipelineRun.Status.CompletionTime == nil {
				t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
			}

			// Since the PipelineRun is invalid, the status should say it has failed
			condition := tc.pipelineRun.Status.GetCondition(apis.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected status to be failed on invalid PipelineRun but was: %v", condition)
			}
			if condition != nil && condition.Reason != tc.reason {
				t.Errorf("Expected failure to be because of reason %q but was %s", tc.reason, condition.Reason)
			}
			if !tc.hasNoDefaultLabels {
				expectedPipelineLabel := tc.pipelineRun.Name
				// Embedded pipelines use the pipelinerun name
				if tc.pipelineRun.Spec.PipelineRef != nil {
					expectedPipelineLabel = tc.pipelineRun.Spec.PipelineRef.Name
				}
				expectedLabels := map[string]string{pipeline.GroupName + pipeline.PipelineLabelKey: expectedPipelineLabel}
				if len(tc.pipelineRun.ObjectMeta.Labels) != len(expectedLabels) {
					t.Errorf("Expected labels : %v, got %v", expectedLabels, tc.pipelineRun.ObjectMeta.Labels)
				}
				for k, ev := range expectedLabels {
					if v, ok := tc.pipelineRun.ObjectMeta.Labels[k]; ok {
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

			err := c.Reconciler.Reconcile(context.Background(), tc.pipelineRun)
			// No reason to keep reconciling something that doesnt or can't exist
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid PipelineRun but saw %q", err)
			}
		})
	}
}

func TestUpdateTaskRunsState(t *testing.T) {
	pr := tb.PipelineRun("test-pipeline-run", "foo", tb.PipelineRunSpec("test-pipeline"))
	pipelineTask := v1alpha1.PipelineTask{
		Name:    "unit-test-1",
		TaskRef: &v1alpha1.TaskRef{Name: "unit-test-task"},
	}
	task := tb.Task("unit-test-task", "foo", tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
	))
	taskrun := tb.TaskRun("test-pipeline-run-success-unit-test-1", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef("unit-test-task"),
		tb.TaskRunServiceAccountName("test-sa"),
	), tb.TaskRunStatus(
		tb.StatusCondition(apis.Condition{Type: apis.ConditionSucceeded}),
		tb.StepState(tb.StateTerminated(0)),
	))

	expectedTaskRunsStatus := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	expectedTaskRunsStatus["test-pipeline-run-success-unit-test-1"] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "unit-test-1",
		Status: &v1alpha1.TaskRunStatus{
			TaskRunStatusFields: v1alpha1.TaskRunStatusFields{
				Steps: []v1alpha1.StepState{{
					ContainerState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
					},
				}}},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}
	expectedPipelineRunStatus := v1alpha1.PipelineRunStatus{
		PipelineRunStatusFields: v1alpha1.PipelineRunStatusFields{
			TaskRuns: expectedTaskRunsStatus,
		},
	}

	state := []*resources.ResolvedPipelineRunTask{{
		PipelineTask: &pipelineTask,
		TaskRunName:  "test-pipeline-run-success-unit-test-1",
		TaskRun:      taskrun,
		ResolvedTaskResources: &taskrunresources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
	}}
	pr.Status.InitializeConditions()
	status := getTaskRunsStatus(pr, state)
	if d := cmp.Diff(status, expectedPipelineRunStatus.TaskRuns); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", d)
	}

}

func TestUpdateTaskRunStateWithConditionChecks(t *testing.T) {
	taskrunName := "task-run"
	successConditionCheckName := "success-condition"
	failingConditionCheckName := "fail-condition"

	successCondition := tb.Condition("cond-1", "foo")
	failingCondition := tb.Condition("cond-2", "foo")

	pipelineTask := v1alpha1.PipelineTask{
		TaskRef: &v1alpha1.TaskRef{Name: "unit-test-task"},
		Conditions: []v1alpha1.PipelineTaskCondition{{
			ConditionRef: successCondition.Name,
		}, {
			ConditionRef: failingCondition.Name,
		}},
	}

	successConditionCheck := conditionCheckFromTaskRun(tb.TaskRun(successConditionCheckName, "foo", tb.TaskRunStatus(
		tb.StatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		}),
		tb.StepState(tb.StateTerminated(0)),
	)))
	failingConditionCheck := conditionCheckFromTaskRun(tb.TaskRun(failingConditionCheckName, "foo", tb.TaskRunStatus(
		tb.StatusCondition(apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		}),
		tb.StepState(tb.StateTerminated(127)),
	)))

	successConditionCheckStatus := &v1alpha1.PipelineRunConditionCheckStatus{
		ConditionName: successCondition.Name,
		Status: &v1alpha1.ConditionCheckStatus{
			ConditionCheckStatusFields: v1alpha1.ConditionCheckStatusFields{
				Check: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				},
			},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionTrue}},
			},
		},
	}
	failingConditionCheckStatus := &v1alpha1.PipelineRunConditionCheckStatus{
		ConditionName: failingCondition.Name,
		Status: &v1alpha1.ConditionCheckStatus{
			ConditionCheckStatusFields: v1alpha1.ConditionCheckStatusFields{
				Check: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 127},
				},
			},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded, Status: corev1.ConditionFalse}},
			},
		},
	}

	successrcc := resources.ResolvedConditionCheck{
		ConditionCheckName: successConditionCheckName,
		Condition:          successCondition,
		ConditionCheck:     successConditionCheck,
	}
	failingrcc := resources.ResolvedConditionCheck{
		ConditionCheckName: failingConditionCheckName,
		Condition:          failingCondition,
		ConditionCheck:     failingConditionCheck,
	}

	failedTaskRunStatus := v1alpha1.TaskRunStatus{
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
		expectedStatus v1alpha1.PipelineRunTaskRunStatus
	}{{
		name: "success-condition-checks",
		rcc:  resources.TaskConditionCheckState{&successrcc},
		expectedStatus: v1alpha1.PipelineRunTaskRunStatus{
			ConditionChecks: map[string]*v1alpha1.PipelineRunConditionCheckStatus{
				successrcc.ConditionCheck.Name: successConditionCheckStatus,
			},
		},
	}, {
		name: "failing-condition-checks",
		rcc:  resources.TaskConditionCheckState{&failingrcc},
		expectedStatus: v1alpha1.PipelineRunTaskRunStatus{
			Status: &failedTaskRunStatus,
			ConditionChecks: map[string]*v1alpha1.PipelineRunConditionCheckStatus{
				failingrcc.ConditionCheck.Name: failingConditionCheckStatus,
			},
		},
	}, {
		name: "multiple-condition-checks",
		rcc:  resources.TaskConditionCheckState{&successrcc, &failingrcc},
		expectedStatus: v1alpha1.PipelineRunTaskRunStatus{
			Status: &failedTaskRunStatus,
			ConditionChecks: map[string]*v1alpha1.PipelineRunConditionCheckStatus{
				successrcc.ConditionCheck.Name: successConditionCheckStatus,
				failingrcc.ConditionCheck.Name: failingConditionCheckStatus,
			},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			pr := tb.PipelineRun("test-pipeline-run", "foo", tb.PipelineRunSpec("test-pipeline"))

			state := []*resources.ResolvedPipelineRunTask{{
				PipelineTask:            &pipelineTask,
				TaskRunName:             taskrunName,
				ResolvedConditionChecks: tc.rcc,
			}}
			pr.Status.InitializeConditions()
			status := getTaskRunsStatus(pr, state)
			expected := map[string]*v1alpha1.PipelineRunTaskRunStatus{
				taskrunName: &tc.expectedStatus,
			}
			if d := cmp.Diff(status, expected, ignoreLastTransitionTime); d != "" {
				t.Fatalf("Did not get expected status for %s : %s", tc.name, d)
			}
		})
	}
}

func TestReconcileOnCompletedPipelineRun(t *testing.T) {
	taskRunName := "test-pipeline-run-completed-hello-world"
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-completed", "foo",
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccountName("test-sa")),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  resources.ReasonSucceeded,
			Message: "All Tasks have completed executing",
		}),
			tb.PipelineRunTaskRunsStatus(taskRunName, &v1alpha1.PipelineRunTaskRunStatus{
				PipelineTaskName: "hello-world-1",
				Status:           &v1alpha1.TaskRunStatus{},
			}),
		),
	)}
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world")))}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}
	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(taskRunName, "foo",
			tb.TaskRunOwnerReference("kind", "name"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineLabelKey, "test-pipeline-run-completed"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineRunLabelKey, "test-pipeline"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("hello-world")),
			tb.TaskRunStatus(
				tb.StatusCondition(apis.Condition{
					Type: apis.ConditionSucceeded,
				}),
			),
		),
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-completed"); err != nil {
		t.Fatalf("Error reconciling: %s", err)
	}

	if len(clients.Pipeline.Actions()) != 1 {
		t.Fatalf("Expected client to have updated the TaskRun status for a completed PipelineRun, but it did not")
	}

	actual := clients.Pipeline.Actions()[0].(ktesting.UpdateAction).GetObject().(*v1alpha1.PipelineRun)
	if actual == nil {
		t.Errorf("Expected a PipelineRun to be updated, but it wasn't.")
	}
	actions := clients.Pipeline.Actions()
	for _, action := range actions {
		if action != nil {
			resource := action.GetResource().Resource
			if resource != "pipelineruns" {
				t.Fatalf("Expected client to not have created a TaskRun for the completed PipelineRun, but it did")
			}
		}
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-completed", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	// This PipelineRun should still be complete and the status should reflect that
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	expectedTaskRunsStatus := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	expectedTaskRunsStatus[taskRunName] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-1",
		Status: &v1alpha1.TaskRunStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
	}

	if d := cmp.Diff(reconciledRun.Status.TaskRuns, expectedTaskRunsStatus); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", d)
	}
}

func TestReconcileOnCancelledPipelineRun(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-cancelled", "foo",
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccountName("test-sa"),
			tb.PipelineRunCancelled,
		),
	)}
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}
	trs := []*v1alpha1.TaskRun{
		tb.TaskRun("test-pipeline-run-cancelled-hello-world", "foo",
			tb.TaskRunOwnerReference("kind", "name"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineLabelKey, "test-pipeline-run-cancelled"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineRunLabelKey, "test-pipeline"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("hello-world"),
				tb.TaskRunServiceAccountName("test-sa"),
			),
		),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-cancelled")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-cancelled", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// This PipelineRun should still be complete and false, and the status should reflect that
	if !reconciledRun.Status.GetCondition(apis.ConditionSucceeded).IsFalse() {
		t.Errorf("Expected PipelineRun status to be complete and false, but was %v", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}
}

func TestReconcileWithTimeout(t *testing.T) {
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-with-timeout", "foo",
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa"),
			tb.PipelineRunTimeout(12*time.Hour),
		),
		tb.PipelineRunStatus(
			tb.PipelineRunStartTime(time.Now().AddDate(0, 0, -1))),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-with-timeout")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-with-timeout", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	if reconciledRun.Status.CompletionTime == nil {
		t.Errorf("Expected a CompletionTime on invalid PipelineRun but was nil")
	}

	// The PipelineRun should be timed out.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != resources.ReasonTimedOut {
		t.Errorf("Expected PipelineRun to be timed out, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject().(*v1alpha1.TaskRun)
	if actual == nil {
		t.Errorf("Expected a TaskRun to be created, but it wasn't.")
	}

	// The TaskRun timeout should be less than or equal to the PipelineRun timeout.
	if actual.Spec.Timeout.Duration > prs[0].Spec.Timeout.Duration {
		t.Errorf("TaskRun timeout %s should be less than or equal to PipelineRun timeout %s", actual.Spec.Timeout.Duration.String(), prs[0].Spec.Timeout.Duration.String())
	}
}

func TestReconcileWithoutPVC(t *testing.T) {
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
		tb.PipelineTask("hello-world-2", "hello-world"),
	))}

	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run", "foo",
		tb.PipelineRunSpec("test-pipeline")),
	}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	for _, a := range clients.Kube.Actions() {
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

func TestReconcileCancelledPipelineRun(t *testing.T) {
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world", tb.Retries(1)),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-with-timeout", "foo",
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunCancelled,
		),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-with-timeout")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-with-timeout", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	// The PipelineRun should be still cancelled.
	if reconciledRun.Status.GetCondition(apis.ConditionSucceeded).Reason != "PipelineRunCancelled" {
		t.Errorf("Expected PipelineRun to be cancelled, but condition reason is %s", reconciledRun.Status.GetCondition(apis.ConditionSucceeded))
	}

	// Check that no TaskRun is created or run
	actions := clients.Pipeline.Actions()
	for _, action := range actions {
		actionType := fmt.Sprintf("%T", action)
		if !(actionType == "testing.UpdateActionImpl" || actionType == "testing.GetActionImpl") {
			t.Errorf("Expected a TaskRun to be get/updated, but it was %s", actionType)
		}
	}
}

func TestReconcilePropagateLabels(t *testing.T) {
	names.TestingSeed()
	taskName := "hello-world-1"

	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask(taskName, "hello-world"),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-with-labels", "foo",
		tb.PipelineRunLabel("PipelineRunLabel", "PipelineRunValue"),
		tb.PipelineRunLabel("tekton.dev/pipeline", "WillNotBeUsed"),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa"),
		),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	expected := tb.TaskRun("test-pipeline-run-with-labels-hello-world-1-9l9zj", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-with-labels",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "hello-world-1"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-labels"),
		tb.TaskRunLabel("PipelineRunLabel", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccountName("test-sa"),
		),
	)

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-with-labels")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	_, err = clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-with-labels", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject().(*v1alpha1.TaskRun)
	if actual == nil {
		t.Errorf("Expected a TaskRun to be created, but it wasn't.")
	}

	if d := cmp.Diff(actual, expected); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expected, d)
	}
}

func TestReconcileWithDifferentServiceAccounts(t *testing.T) {
	names.TestingSeed()

	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-0", "hello-world-task"),
		tb.PipelineTask("hello-world-1", "hello-world-task"),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-different-service-accs", "foo",
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa-0"),
			tb.PipelineRunServiceAccountNameTask("hello-world-1", "test-sa-1"),
		),
	)}
	ts := []*v1alpha1.Task{
		tb.Task("hello-world-task", "foo"),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-different-service-accs")

	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	_, err = clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-different-service-accs", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}
	taskRunNames := []string{"test-pipeline-run-different-service-accs-hello-world-0-9l9zj", "test-pipeline-run-different-service-accs-hello-world-1-mz4c7"}

	expectedTaskRuns := []*v1alpha1.TaskRun{
		tb.TaskRun(taskRunNames[0], "foo",
			tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-different-service-accs",
				tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
				tb.Controller, tb.BlockOwnerDeletion,
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("hello-world-task"),
				tb.TaskRunServiceAccountName("test-sa-0"),
			),
			tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
			tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-different-service-accs"),
			tb.TaskRunLabel("tekton.dev/pipelineTask", "hello-world-0"),
		),
		tb.TaskRun(taskRunNames[1], "foo",
			tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-different-service-accs",
				tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
				tb.Controller, tb.BlockOwnerDeletion,
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef("hello-world-task"),
				tb.TaskRunServiceAccountName("test-sa-1"),
			),
			tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
			tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-different-service-accs"),
			tb.TaskRunLabel("tekton.dev/pipelineTask", "hello-world-1"),
		),
	}
	for i := range ps[0].Spec.Tasks {
		// Check that the expected TaskRun was created
		actual, err := clients.Pipeline.Tekton().TaskRuns("foo").Get(taskRunNames[i], metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Expected a TaskRun to be created, but it wasn't: %s", err)
		}
		if d := cmp.Diff(actual, expectedTaskRuns[i]); d != "" {
			t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRuns[i], d)
		}

	}

}

func TestReconcileWithTimeoutAndRetry(t *testing.T) {

	tcs := []struct {
		name               string
		retries            int
		conditionSucceeded corev1.ConditionStatus
	}{
		{
			name:               "One try has to be done",
			retries:            1,
			conditionSucceeded: corev1.ConditionFalse,
		},
		{
			name:               "No more retries are needed",
			retries:            2,
			conditionSucceeded: corev1.ConditionUnknown,
		},
	}

	for _, tc := range tcs {

		t.Run(tc.name, func(t *testing.T) {
			ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline-retry", "foo", tb.PipelineSpec(
				tb.PipelineTask("hello-world-1", "hello-world", tb.Retries(tc.retries)),
			))}
			prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-retry-run-with-timeout", "foo",
				tb.PipelineRunSpec("test-pipeline-retry",
					tb.PipelineRunServiceAccountName("test-sa"),
					tb.PipelineRunTimeout(12*time.Hour),
				),
				tb.PipelineRunStatus(
					tb.PipelineRunStartTime(time.Now().AddDate(0, 0, -1))),
			)}

			ts := []*v1alpha1.Task{
				tb.Task("hello-world", "foo"),
			}
			trs := []*v1alpha1.TaskRun{
				tb.TaskRun("hello-world-1", "foo",
					tb.TaskRunStatus(
						tb.PodName("my-pod-name"),
						tb.StatusCondition(apis.Condition{
							Type:   apis.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						}),
						tb.Retry(v1alpha1.TaskRunStatus{
							Status: duckv1beta1.Status{
								Conditions: []apis.Condition{{
									Type:   apis.ConditionSucceeded,
									Status: corev1.ConditionFalse,
								}},
							},
						}),
					)),
			}

			prtrs := &v1alpha1.PipelineRunTaskRunStatus{
				PipelineTaskName: "hello-world-1",
				Status:           &trs[0].Status,
			}
			prs[0].Status.TaskRuns = make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
			prs[0].Status.TaskRuns["hello-world-1"] = prtrs

			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
				TaskRuns:     trs,
			}

			testAssets, cancel := getPipelineRunController(t, d)
			defer cancel()
			c := testAssets.Controller
			clients := testAssets.Clients

			err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-retry-run-with-timeout")
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
			}

			// Check that the PipelineRun was reconciled correctly
			reconciledRun, err := clients.Pipeline.TektonV1alpha1().PipelineRuns("foo").Get("test-pipeline-retry-run-with-timeout", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
			}

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

	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-with-annotations", "foo",
		tb.PipelineRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa"),
		),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-with-annotations")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	_, err = clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-with-annotations", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject().(*v1alpha1.TaskRun)
	if actual == nil {
		t.Errorf("Expected a TaskRun to be created, but it wasn't.")
	}
	expectedTaskRun := tb.TaskRun("test-pipeline-run-with-annotations-hello-world-1-9l9zj", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-with-annotations",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "hello-world-1"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-annotations"),
		tb.TaskRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccountName("test-sa"),
		),
	)

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
	}
}

func TestGetTaskRunTimeout(t *testing.T) {
	prName := "pipelinerun-timeouts"
	ns := "foo"
	p := "pipeline"

	tcs := []struct {
		name     string
		pr       *v1alpha1.PipelineRun
		expected *metav1.Duration
	}{{
		name: "nil timeout duration",
		pr: tb.PipelineRun(prName, ns,
			tb.PipelineRunSpec(p, tb.PipelineRunNilTimeout),
			tb.PipelineRunStatus(tb.PipelineRunStartTime(time.Now())),
		),
		expected: &metav1.Duration{Duration: 60 * time.Minute},
	}, {
		name: "timeout specified in pr",
		pr: tb.PipelineRun(prName, ns,
			tb.PipelineRunSpec(p, tb.PipelineRunTimeout(20*time.Minute)),
			tb.PipelineRunStatus(tb.PipelineRunStartTime(time.Now())),
		),
		expected: &metav1.Duration{Duration: 20 * time.Minute},
	}, {
		name: "0 timeout duration",
		pr: tb.PipelineRun(prName, ns,
			tb.PipelineRunSpec(p, tb.PipelineRunTimeout(0*time.Minute)),
			tb.PipelineRunStatus(tb.PipelineRunStartTime(time.Now())),
		),
		expected: &metav1.Duration{Duration: 0 * time.Minute},
	}, {
		name: "taskrun being created after timeout expired",
		pr: tb.PipelineRun(prName, ns,
			tb.PipelineRunSpec(p, tb.PipelineRunTimeout(1*time.Minute)),
			tb.PipelineRunStatus(tb.PipelineRunStartTime(time.Now().Add(-2*time.Minute)))),
		expected: &metav1.Duration{Duration: 1 * time.Second},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if d := cmp.Diff(getTaskRunTimeout(tc.pr), tc.expected); d != "" {
				t.Errorf("Unexpected task run timeout. Diff %s", d)
			}
		})
	}
}

func TestReconcileWithConditionChecks(t *testing.T) {
	names.TestingSeed()
	prName := "test-pipeline-run"
	conditions := []*v1alpha1.Condition{
		tb.Condition("cond-1", "foo",
			tb.ConditionLabels(
				map[string]string{
					"label-1": "value-1",
					"label-2": "value-2",
				}),
			tb.ConditionSpec(
				tb.ConditionSpecCheck("", "foo", tb.Args("bar")),
			)),
		tb.Condition("cond-2", "foo",
			tb.ConditionLabels(
				map[string]string{
					"label-3": "value-3",
					"label-4": "value-4",
				}),
			tb.ConditionSpec(
				tb.ConditionSpecCheck("", "foo", tb.Args("bar")),
			)),
	}
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world",
			tb.PipelineTaskCondition("cond-1"),
			tb.PipelineTaskCondition("cond-2")),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun(prName, "foo",
		tb.PipelineRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa"),
		),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		Conditions:   conditions,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/"+prName)
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	_, err = clients.Pipeline.Tekton().PipelineRuns("foo").Get(prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}
	ccNameBase := prName + "-hello-world-1-9l9zj"
	ccNames := map[string]string{
		"cond-1": ccNameBase + "-cond-1-mz4c7",
		"cond-2": ccNameBase + "-cond-2-mssqb",
	}
	expectedConditionChecks := make([]*v1alpha1.TaskRun, len(conditions))
	for index, condition := range conditions {
		expectedConditionChecks[index] = makeExpectedTr(condition.Name, ccNames[condition.Name], condition.Labels)
	}

	// Check that the expected TaskRun was created
	condCheck0 := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject().(*v1alpha1.TaskRun)
	condCheck1 := clients.Pipeline.Actions()[1].(ktesting.CreateAction).GetObject().(*v1alpha1.TaskRun)
	if condCheck0 == nil || condCheck1 == nil {
		t.Errorf("Expected two ConditionCheck TaskRuns to be created, but it wasn't.")
	}

	actual := []*v1alpha1.TaskRun{condCheck0, condCheck1}
	if d := cmp.Diff(actual, expectedConditionChecks); d != "" {
		t.Errorf("expected to see 2 ConditionCheck TaskRuns created. Diff %s", d)
	}
}

func TestReconcileWithFailingConditionChecks(t *testing.T) {
	names.TestingSeed()
	conditions := []*v1alpha1.Condition{
		tb.Condition("always-false", "foo", tb.ConditionSpec(
			tb.ConditionSpecCheck("", "foo", tb.Args("bar")),
		)),
	}
	pipelineRunName := "test-pipeline-run-with-conditions"
	prccs := make(map[string]*v1alpha1.PipelineRunConditionCheckStatus)

	conditionCheckName := pipelineRunName + "task-2-always-false-xxxyyy"
	prccs[conditionCheckName] = &v1alpha1.PipelineRunConditionCheckStatus{
		ConditionName: "always-false",
		Status:        &v1alpha1.ConditionCheckStatus{},
	}
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("task-1", "hello-world"),
		tb.PipelineTask("task-2", "hello-world", tb.PipelineTaskCondition("always-false")),
		tb.PipelineTask("task-3", "hello-world", tb.RunAfter("task-1")),
	))}

	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-with-conditions", "foo",
		tb.PipelineRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccountName("test-sa"),
		),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  resources.ReasonRunning,
			Message: "Not all Tasks in the Pipeline have finished executing",
		}), tb.PipelineRunTaskRunsStatus(pipelineRunName+"task-1", &v1alpha1.PipelineRunTaskRunStatus{
			PipelineTaskName: "task-1",
			Status:           &v1alpha1.TaskRunStatus{},
		}), tb.PipelineRunTaskRunsStatus(pipelineRunName+"task-2", &v1alpha1.PipelineRunTaskRunStatus{
			PipelineTaskName: "task-2",
			Status:           &v1alpha1.TaskRunStatus{},
			ConditionChecks:  prccs,
		})),
	)}

	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}
	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(pipelineRunName+"task-1", "foo",
			tb.TaskRunOwnerReference("kind", "name"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineLabelKey, "test-pipeline-run-with-conditions"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineRunLabelKey, "test-pipeline"),
			tb.TaskRunSpec(tb.TaskRunTaskRef("hello-world")),
			tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			}),
			),
		),
		tb.TaskRun(conditionCheckName, "foo",
			tb.TaskRunOwnerReference("kind", "name"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineLabelKey, "test-pipeline-run-with-conditions"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineRunLabelKey, "test-pipeline"),
			tb.TaskRunLabel(pipeline.GroupName+pipeline.ConditionCheckKey, conditionCheckName),
			tb.TaskRunSpec(tb.TaskRunTaskSpec()),
			tb.TaskRunStatus(tb.StatusCondition(apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			}),
			),
		),
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		Conditions:   conditions,
		TaskRuns:     trs,
	}

	testAssets, cancel := getPipelineRunController(t, d)
	defer cancel()
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-with-conditions")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}

	// Check that the PipelineRun was reconciled correctly
	_, err = clients.Pipeline.Tekton().PipelineRuns("foo").Get("test-pipeline-run-with-conditions", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject().(*v1alpha1.TaskRun)
	if actual == nil {
		t.Errorf("Expected a ConditionCheck TaskRun to be created, but it wasn't.")
	}
	expectedTaskRun := tb.TaskRun("test-pipeline-run-with-conditions-task-3-9l9zj", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-with-conditions",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "task-3"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-conditions"),
		tb.TaskRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccountName("test-sa"),
		),
	)

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see ConditionCheck TaskRun %v created. Diff %s", expectedTaskRun, d)
	}
}

func makeExpectedTr(condName, ccName string, labels map[string]string) *v1alpha1.TaskRun {
	return tb.TaskRun(ccName, "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "hello-world-1"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run"),
		tb.TaskRunLabel("tekton.dev/conditionCheck", ccName),
		tb.TaskRunLabels(labels),
		tb.TaskRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskSpec(
				tb.Step("foo", tb.StepName("condition-check-"+condName), tb.StepArgs("bar")),
				tb.TaskInputs(),
			),
			tb.TaskRunServiceAccountName("test-sa"),
		),
	)
}

func ensurePVCCreated(t *testing.T, clients test.Clients, name, namespace string) {
	t.Helper()
	_, err := clients.Kube.CoreV1().PersistentVolumeClaims(namespace).Get(name, metav1.GetOptions{})
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
