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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/knative/pkg/apis"
	duckv1beta1 "github.com/knative/pkg/apis/duck/v1beta1"
	"github.com/knative/pkg/configmap"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	taskrunresources "github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/system"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"github.com/tektoncd/pipeline/test/names"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
)

func getRunName(pr *v1alpha1.PipelineRun) string {
	return strings.Join([]string{pr.Namespace, pr.Name}, "/")
}

// getPipelineRunController returns an instance of the PipelineRun controller/reconciler that has been seeded with
// d, where d represents the state of the system (existing resources) needed for the test.
// recorder can be a fake recorder that is used for testing
func getPipelineRunController(t *testing.T, d test.Data, recorder record.EventRecorder) test.TestAssets {
	c, i := test.SeedTestData(t, d)
	observer, logs := observer.New(zap.InfoLevel)
	stopCh := make(chan struct{})
	configMapWatcher := configmap.NewInformedWatcher(c.Kube, system.GetNamespace())
	logger := zap.New(observer).Sugar()
	th := reconciler.NewTimeoutHandler(c.Kube, c.Pipeline, stopCh, logger)
	return test.TestAssets{
		Controller: NewController(
			reconciler.Options{
				Logger:            logger,
				KubeClientSet:     c.Kube,
				PipelineClientSet: c.Pipeline,
				Recorder:          recorder,
				ConfigMapWatcher:  configMapWatcher,
			},
			i.PipelineRun,
			i.Pipeline,
			i.Task,
			i.ClusterTask,
			i.TaskRun,
			i.PipelineResource,
			th,
		),
		Logs:      logs,
		Clients:   c,
		Informers: i,
	}
}

func TestReconcile(t *testing.T) {
	names.TestingSeed()

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("test-pipeline-run-success", "foo",
			tb.PipelineRunSpec("test-pipeline",
				tb.PipelineRunServiceAccount("test-sa"),
				tb.PipelineRunResourceBinding("git-repo", tb.PipelineResourceBindingRef("some-repo")),
				tb.PipelineRunResourceBinding("best-image", tb.PipelineResourceBindingRef("some-image")),
				tb.PipelineRunParam("bar", "somethingmorefun"),
			),
		),
	}
	funParam := tb.PipelineTaskParam("foo", "somethingfun")
	moreFunParam := tb.PipelineTaskParam("bar", "${params.bar}")
	templatedParam := tb.PipelineTaskParam("templatedparam", "${inputs.workspace.${params.rev-param}}")
	ps := []*v1alpha1.Pipeline{
		tb.Pipeline("test-pipeline", "foo",
			tb.PipelineSpec(
				tb.PipelineDeclaredResource("git-repo", "git"),
				tb.PipelineDeclaredResource("best-image", "image"),
				tb.PipelineParam("pipeline-param", tb.PipelineParamDefault("somethingdifferent")),
				tb.PipelineParam("rev-param", tb.PipelineParamDefault("revision")),
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
				tb.InputsParam("foo"), tb.InputsParam("bar"), tb.InputsParam("templatedparam"),
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
				tb.InputsParam("foo"), tb.InputsParam("bar"), tb.InputsParam("templatedparam"),
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

	// create fake recorder for testing
	fr := record.NewFakeRecorder(1)

	testAssets := getPipelineRunController(t, d, fr)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-success"); err != nil {
		t.Fatalf("Error reconciling: %s", err)
	}

	// make sure there is no failed events
	validateNoEvents(t, fr)

	if len(clients.Pipeline.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

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
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("unit-test-task"),
			tb.TaskRunServiceAccount("test-sa"),
			tb.TaskRunInputs(
				tb.TaskRunInputsParam("foo", "somethingfun"),
				tb.TaskRunInputsParam("bar", "somethingmorefun"),
				tb.TaskRunInputsParam("templatedparam", "${inputs.workspace.revision}"),
				tb.TaskRunInputsResource("workspace", tb.TaskResourceBindingRef("some-repo")),
			),
			tb.TaskRunOutputs(
				tb.TaskRunOutputsResource("image-to-use", tb.TaskResourceBindingRef("some-image"),
					tb.TaskResourceBindingPaths("/pvc/unit-test-1/image-to-use"),
				),
				tb.TaskRunOutputsResource("workspace", tb.TaskResourceBindingRef("some-repo"),
					tb.TaskResourceBindingPaths("/pvc/unit-test-1/workspace"),
				),
			),
		),
	)

	// ignore IgnoreUnexported ignore both after and before steps fields
	if d := cmp.Diff(actual, expectedTaskRun, cmpopts.SortSlices(func(x, y v1alpha1.TaskResourceBinding) bool { return x.Name < y.Name })); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
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
}

func TestReconcile_InvalidPipelineRuns(t *testing.T) {
	ts := []*v1alpha1.Task{
		tb.Task("a-task-that-exists", "foo"),
		tb.Task("a-task-that-needs-params", "foo", tb.TaskSpec(
			tb.TaskInputs(tb.InputsParam("some-param")))),
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
	}
	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun("invalid-pipeline", "foo", tb.PipelineRunSpec("pipeline-not-exist")),
		tb.PipelineRun("pipelinerun-missing-tasks", "foo", tb.PipelineRunSpec("pipeline-missing-tasks")),
		tb.PipelineRun("pipeline-params-dont-exist", "foo", tb.PipelineRunSpec("a-pipeline-without-params")),
		tb.PipelineRun("pipeline-resources-not-bound", "foo", tb.PipelineRunSpec("a-fine-pipeline")),
		tb.PipelineRun("pipeline-resources-dont-exist", "foo", tb.PipelineRunSpec("a-fine-pipeline",
			tb.PipelineRunResourceBinding("a-resource", tb.PipelineResourceBindingRef("missing-resource")))),
		tb.PipelineRun("pipeline-resources-not-declared", "foo", tb.PipelineRunSpec("a-pipeline-that-should-be-caught-by-admission-control")),
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
		},
	}

	// Create fake recorder for testing
	fr := record.NewFakeRecorder(1)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := getPipelineRunController(t, d, fr)
			c := testAssets.Controller

			if err := c.Reconciler.Reconcile(context.Background(), getRunName(tc.pipelineRun)); err != nil {
				t.Fatalf("Error reconciling: %s", err)
			}
			// When a PipelineRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.

			// make sure there is no failed events
			validateNoEvents(t, fr)

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

	// create fake recorder for testing
	fr := record.NewFakeRecorder(1)

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := getPipelineRunController(t, test.Data{}, fr)
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
		tb.Condition(apis.Condition{Type: apis.ConditionSucceeded}),
		tb.StepState(tb.StateTerminated(0)),
	))

	expectedTaskRunsStatus := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	expectedTaskRunsStatus["test-pipeline-run-success-unit-test-1"] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "unit-test-1",
		Status: &v1alpha1.TaskRunStatus{
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				},
			}},
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{Type: apis.ConditionSucceeded}},
			},
		},
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
	prtrs := make(map[string]*v1alpha1.PipelineRunTaskRunStatus)
	taskRunName := "test-pipeline-run-completed-hello-world"
	prtrs[taskRunName] = &v1alpha1.PipelineRunTaskRunStatus{
		PipelineTaskName: "hello-world-1",
		Status:           &v1alpha1.TaskRunStatus{},
	}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-completed", "foo",
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa")),
		tb.PipelineRunStatus(tb.PipelineRunStatusCondition(apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Reason:  resources.ReasonSucceeded,
			Message: "All Tasks have completed executing",
		}),
			tb.PipelineRunTaskRunsStatus(prtrs),
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
				tb.Condition(apis.Condition{
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

	// create fake recorder for testing
	fr := record.NewFakeRecorder(1)

	testAssets := getPipelineRunController(t, d, fr)
	c := testAssets.Controller
	clients := testAssets.Clients

	if err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-completed"); err != nil {
		t.Fatalf("Error reconciling: %s", err)
	}

	// make sure there is no failed events
	validateNoEvents(t, fr)

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
		PipelineTaskName: prtrs[taskRunName].PipelineTaskName,
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

func validateNoEvents(t *testing.T, r *record.FakeRecorder) {
	t.Helper()
	timer := time.NewTimer(1 * time.Second)

	select {
	case event := <-r.Events:
		t.Errorf("Expected no event but got %s", event)
	case <-timer.C:
		return
	}
}

func TestReconcileOnCancelledPipelineRun(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-cancelled", "foo",
		tb.PipelineRunSpec("test-pipeline", tb.PipelineRunServiceAccount("test-sa"),
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
				tb.TaskRunServiceAccount("test-sa"),
			),
		),
	}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
		TaskRuns:     trs,
	}

	// create fake recorder for testing
	fr := record.NewFakeRecorder(1)

	testAssets := getPipelineRunController(t, d, fr)
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
			tb.PipelineRunServiceAccount("test-sa"),
			tb.PipelineRunTimeout(&metav1.Duration{Duration: 12 * time.Hour}),
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

	// create fake recorder for testing
	fr := record.NewFakeRecorder(2)

	testAssets := getPipelineRunController(t, d, fr)
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

	// create fake recorder for testing
	fr := record.NewFakeRecorder(2)

	testAssets := getPipelineRunController(t, d, fr)
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

	ps := []*v1alpha1.Pipeline{tb.Pipeline("test-pipeline", "foo", tb.PipelineSpec(
		tb.PipelineTask("hello-world-1", "hello-world"),
	))}
	prs := []*v1alpha1.PipelineRun{tb.PipelineRun("test-pipeline-run-with-labels", "foo",
		tb.PipelineRunLabel("PipelineRunLabel", "PipelineRunValue"),
		tb.PipelineRunLabel("tekton.dev/pipeline", "WillNotBeUsed"),
		tb.PipelineRunSpec("test-pipeline",
			tb.PipelineRunServiceAccount("test-sa"),
		),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	// create fake recorder for testing
	fr := record.NewFakeRecorder(2)

	testAssets := getPipelineRunController(t, d, fr)
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
	expectedTaskRun := tb.TaskRun("test-pipeline-run-with-labels-hello-world-1-9l9zj", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-with-labels",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-labels"),
		tb.TaskRunLabel("PipelineRunLabel", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccount("test-sa"),
		),
	)

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
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
					tb.PipelineRunServiceAccount("test-sa"),
					tb.PipelineRunTimeout(&metav1.Duration{Duration: 12 * time.Hour}),
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
						tb.Condition(apis.Condition{
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

			fr := record.NewFakeRecorder(2)

			testAssets := getPipelineRunController(t, d, fr)
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
			tb.PipelineRunServiceAccount("test-sa"),
		),
	)}
	ts := []*v1alpha1.Task{tb.Task("hello-world", "foo")}

	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	// create fake recorder for testing
	fr := record.NewFakeRecorder(2)

	testAssets := getPipelineRunController(t, d, fr)
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
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-annotations"),
		tb.TaskRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccount("test-sa"),
		),
	)

	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
	}
}
