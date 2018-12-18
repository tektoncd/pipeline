/*
Copyright 2018 The Knative Authors
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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	taskrunresources "github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/knative/build-pipeline/test"
	tb "github.com/knative/build-pipeline/test/builder"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktesting "k8s.io/client-go/testing"
)

func getRunName(pr *v1alpha1.PipelineRun) string {
	return strings.Join([]string{pr.Namespace, pr.Name}, "/")
}

// getPipelineRunController returns an instance of the PipelineRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
func getPipelineRunController(d test.Data) test.TestAssets {
	c, i := test.SeedTestData(d)
	observer, logs := observer.New(zap.InfoLevel)
	return test.TestAssets{
		Controller: NewController(
			reconciler.Options{
				Logger:            zap.New(observer).Sugar(),
				KubeClientSet:     c.Kube,
				PipelineClientSet: c.Pipeline,
			},
			i.PipelineRun,
			i.Pipeline,
			i.Task,
			i.ClusterTask,
			i.TaskRun,
			i.PipelineResource,
		),
		Logs:      logs,
		Clients:   c,
		Informers: i,
	}
}

func TestReconcile(t *testing.T) {
	workspaceInput := tb.PipelineTaskResourceInputs("workspace", tb.ResourceBindingRef("some-repo"))
	imageOutput := tb.PipelineTaskResourceOutputs("image-to-use", tb.ResourceBindingRef("some-image"))
	workspaceOutput := tb.PipelineTaskResourceOutputs("workspace", tb.ResourceBindingRef("some-repo"))
	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("test-pipeline-run-success", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunServiceAccount("test-sa"),
				tb.PipelineRunTaskResource("unit-test-1",
					workspaceInput, imageOutput, workspaceOutput,
				),
				tb.PipelineRunTaskResource("unit-test-2",
					workspaceInput,
				),
				tb.PipelineRunTaskResource("unit-test-cluster-task",
					workspaceInput, imageOutput, workspaceOutput,
				),
			),
		),
	}
	funParam := tb.PipelineTaskParam("foo", "somethingfun")
	moreFunParam := tb.PipelineTaskParam("bar", "somethingmorefun")
	templatedParam := tb.PipelineTaskParam("templatedparam", "${inputs.workspace.revision}")
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineTask("unit-test-1", "unit-test-task",
					funParam, moreFunParam, templatedParam,
				),
				tb.PipelineTask("unit-test-2", "unit-test-followup-task",
					tb.PipelineTaskResourceDependency("workspace", tb.ProvidedBy("unit-test-1")),
				),
				tb.PipelineTask("unit-test-cluster-task", "unit-test-cluster-task",
					tb.PipelineTaskRefKind(v1alpha1.ClusterTaskKind),
					funParam, moreFunParam, templatedParam,
				),
			),
		),
	}
	ts := []*v1alpha1.Task{
		tb.Task("unit-test-task", "foo", tb.TaskSpec(
			tb.TaskInputs(
				tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit),
				tb.InputsParam("foo"), tb.InputsParam("bar"),
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
				tb.InputsParam("foo"), tb.InputsParam("bar"),
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
		tb.PipelineResource("some-image", "foo", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeImage,
			tb.PipelineResourceSpecParam("url", "gcr.io/sven"),
		)),
	}
	d := test.Data{
		PipelineRuns:      prs,
		Pipelines:         ps,
		Tasks:             ts,
		ClusterTasks:      clusterTasks,
		PipelineResources: rs,
	}

	testAssets := getPipelineRunController(d)
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-success")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling valid Pipeline but saw %s", err)
	}
	if len(clients.Pipeline.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Pipeline().PipelineRuns("foo").Get("test-pipeline-run-success", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}

	// Check that the expected TaskRun was created
	actual := clients.Pipeline.Actions()[0].(ktesting.CreateAction).GetObject()
	expectedTaskRun := tb.TaskRun("test-pipeline-run-success-unit-test-1", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-success",
			tb.OwnerReferenceAPIVersion("pipeline.knative.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("pipeline.knative.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel("pipeline.knative.dev/pipelineRun", "test-pipeline-run-success"),
		tb.TaskRunSpec(tb.TaskRunTaskRef("unit-test-task"),
			tb.TaskRunServiceAccount("test-sa"),
			tb.TaskRunInputs(
				tb.TaskRunInputsParam("foo", "somethingfun"),
				tb.TaskRunInputsParam("bar", "somethingmorefun"),
				tb.TaskRunInputsParam("templatedparam", "${inputs.workspace.revision}"),
				tb.TaskRunInputsResource("workspace", tb.ResourceBindingRef("some-repo")),
			),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource("image-to-use", tb.ResourceBindingRef("some-image"),
					tb.ResourceBindingPaths("/pvc/unit-test-1/image-to-use"),
				),
				tb.TaskRunOutputsResource("workspace", tb.ResourceBindingRef("some-repo"),
					tb.ResourceBindingPaths("/pvc/unit-test-1/workspace"),
				),
			),
		),
	)

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
	}
	// test taskrun is able to recreate correct pipeline-pvc-name
	if expectedTaskRun.GetPipelineRunPVCName() != "test-pipeline-run-success-pvc" {
		t.Errorf("expected to see TaskRun PVC name set to %q created but got %s", "test-pipeline-run-success-pvc", expectedTaskRun.GetPipelineRunPVCName())
	}

	// This PipelineRun is in progress now and the status should reflect that
	condition := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %v", condition)
	}
	if condition != nil && condition.Reason != resources.ReasonRunning {
		t.Errorf("Expected reason %q but was %s", resources.ReasonRunning, condition.Reason)
	}

	if len(reconciledRun.Status.TaskRuns) != 1 {
		t.Errorf("Expected PipelineRun status to include only one TaskRun status item")
	}
	if _, exists := reconciledRun.Status.TaskRuns["test-pipeline-run-success-unit-test-1"]; exists == false {
		t.Errorf("Expected PipelineRun status to include TaskRun status")
	}
}

func TestReconcile_InvalidPipelineRuns(t *testing.T) {
	ts := []*v1alpha1.Task{tb.Task("a-task-that-exists", "foo")}
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("pipeline-missing-tasks", "foo", tb.PipelineSpec(
			tb.PipelineTask("myspecialtask", "sometask"),
		)),
		tb.Pipeline("a-fine-pipeline", "foo", tb.PipelineSpec(
			tb.PipelineTask("some-task", "a-task-that-exists"),
		)),
	}
	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("invalid-pipeline", "foo", tb.PipelineRunSpec("pipeline-not-exist")),
		tb.PipelineRun("pipelinerun-missing-tasks", "foo", tb.PipelineRunSpec("pipeline-missing-tasks")),
		tb.PipelineRun("pipeline-params-dont-exist", "foo", tb.PipelineRunSpec("a-fine-pipeline")),
	}
	d := test.Data{
		Tasks:        ts,
		Pipelines:    ps,
		PipelineRuns: prs,
	}
	tcs := []struct {
		name        string
		pipelineRun *v1alpha1.PipelineRun
		reason      string
	}{
		{
			name:        "invalid-pipeline-shd-be-stop-reconciling",
			pipelineRun: prs[0],
			reason:      ReasonCouldntGetPipeline,
		}, {
			name:        "invalid-pipeline-run-missing-tasks-shd-stop-reconciling",
			pipelineRun: prs[1],
			reason:      ReasonCouldntGetTask,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := getPipelineRunController(d)
			c := testAssets.Controller

			err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.pipelineRun))
			// When a PipelineRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid PipelineRun but saw %q", err)
			}
			// Since the PipelineRun is invalid, the status should say it has failed
			condition := tc.pipelineRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected status to be failed on invalid PipelineRun but was: %v", condition)
			}
			if condition != nil && condition.Reason != tc.reason {
				t.Errorf("Expected failure to be because of reason %q but was %s", tc.reason, condition.Reason)
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
		log         string
	}{
		{
			name:        "invalid-pipeline-run-shd-stop-reconciling",
			pipelineRun: invalidNames[0],
			log:         "pipeline run \"foo/test-pipeline-run-doesnot-exist\" in work queue no longer exists",
		}, {
			name:        "invalid-pipeline-run-name-shd-stop-reconciling",
			pipelineRun: invalidNames[1],
			log:         "invalid resource key: test/invalidformat/t",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := getPipelineRunController(test.Data{})
			c := testAssets.Controller
			logs := testAssets.Logs

			err := c.Reconciler.Reconcile(context.Background(), tc.pipelineRun)
			// No reason to keep reconciling something that doesnt or can't exist
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid PipelineRun but saw %q", err)
			}
			if logs.FilterMessage(tc.log).Len() == 0 {
				m := test.GetLogMessages(logs)
				t.Errorf("Log lines diff %s", cmp.Diff(tc.log, m))
			}
		})
	}
}

func TestUpdateTaskRunsState(t *testing.T) {
	pr := tb.PipelineRun("test-pipeline-run", "foo", tb.PipelineRunSpec("test-pipeline"))
	pipelineTask := v1alpha1.PipelineTask{
		Name:    "unit-test-1",
		TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
	}
	task := tb.Task("unit-test-task", "foo", tb.TaskSpec(
		tb.TaskInputs(tb.InputsResource("workspace", v1alpha1.PipelineResourceTypeGit)),
	))
	taskrun := tb.TaskRun("test-pipeline-run-success-unit-test-1", "foo", tb.TaskRunSpec(
		tb.TaskRunTaskRef("unit-test-task"),
		tb.TaskRunServiceAccount("test-sa"),
	), tb.TaskRunStatus(
		tb.Condition(duckv1alpha1.Condition{Type: duckv1alpha1.ConditionSucceeded}),
		tb.StepState(tb.StateTerminated(0)),
	))

	expectedTaskRunsStatus := make(map[string]v1alpha1.TaskRunStatus)
	expectedTaskRunsStatus["test-pipeline-run-success-unit-test-1"] = v1alpha1.TaskRunStatus{
		Steps: []v1alpha1.StepState{{
			ContainerState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
			},
		}},
		Conditions: []duckv1alpha1.Condition{{
			Type: duckv1alpha1.ConditionSucceeded,
		}},
	}
	expectedPipelineRunStatus := v1alpha1.PipelineRunStatus{
		TaskRuns: expectedTaskRunsStatus,
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
	updateTaskRunsStatus(pr, state)
	if d := cmp.Diff(pr.Status.TaskRuns, expectedPipelineRunStatus.TaskRuns); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", d)
	}

}

func TestReconcileOnCompletedPipelineRun(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-completed", "foo",
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa")),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(duckv1alpha1.Condition{
			Type:    duckv1alpha1.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  resources.ReasonSucceeded,
			Message: "All Tasks have completed executing",
		})),
	)}
	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hellow-world"),
	))}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets := getPipelineRunController(d)
	c := testAssets.Controller
	clients := testAssets.Clients

	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-completed")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling completed PipelineRun but saw %s", err)
	}
	if len(clients.Pipeline.Actions()) != 0 {
		t.Fatalf("Expected client to not have created a TaskRun for the completed PipelineRun, but it did")
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := clients.Pipeline.Pipeline().PipelineRuns("foo").Get("test-pipeline-run-completed", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting completed reconciled run out of fake client: %s", err)
	}

	// This PipelineRun should still be complete and the status should reflect that
	if reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsUnknown() {
		t.Errorf("Expected PipelineRun status to be complete, but was %v", reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded))
	}
}
