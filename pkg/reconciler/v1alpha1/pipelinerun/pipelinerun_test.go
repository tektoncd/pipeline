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

package pipelinerun_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	taskrunresources "github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	"github.com/knative/build-pipeline/test"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktesting "k8s.io/client-go/testing"
)

func getRunName(pr *v1alpha1.PipelineRun) string {
	return strings.Join([]string{pr.Namespace, pr.Name}, "/")
}

func TestReconcile(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-success",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
				Inputs: []v1alpha1.TaskResourceBinding{{
					Name: "workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-repo",
					},
				}},
				Outputs: []v1alpha1.TaskResourceBinding{{
					Name: "image-to-use",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-image",
					},
				}, {
					Name: "workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-repo",
					},
				}},
			}, {
				Name: "unit-test-2",
				Inputs: []v1alpha1.TaskResourceBinding{{
					Name: "workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-repo",
					},
				}},
			}, {
				Name: "unit-test-cluster-task",
				Inputs: []v1alpha1.TaskResourceBinding{{
					Name: "workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-repo",
					},
				}},
				Outputs: []v1alpha1.TaskResourceBinding{{
					Name: "image-to-use",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-image",
					},
				}, {
					Name: "workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-repo",
					},
				}},
			}},
			ServiceAccount: "test-sa",
		},
	}}
	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
				Params: []v1alpha1.Param{{
					Name:  "foo",
					Value: "somethingfun",
				}, {
					Name:  "bar",
					Value: "somethingmorefun",
				}, {
					Name:  "templatedparam",
					Value: "${inputs.workspace.revision}",
				}},
			}, {
				Name:    "unit-test-2",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-followup-task"},
				ResourceDependencies: []v1alpha1.ResourceDependency{{
					Name:       "workspace",
					ProvidedBy: []string{"unit-test-1"},
				}},
			}, {
				Name: "unit-test-cluster-task",
				TaskRef: v1alpha1.TaskRef{
					Name: "unit-test-cluster-task",
					Kind: "ClusterTask",
				},
				Params: []v1alpha1.Param{{
					Name:  "foo",
					Value: "somethingfun",
				}, {
					Name:  "bar",
					Value: "somethingmorefun",
				}, {
					Name:  "templatedparam",
					Value: "${inputs.workspace.revision}",
				}},
			}},
		},
	}}
	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: "git",
				}},
				Params: []v1alpha1.TaskParam{{
					Name:        "foo",
					Description: "foo",
				}, {
					Name:        "bar",
					Description: "bar",
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "image-to-use",
					Type: "image",
				}, {
					Name: "workspace",
					Type: "git",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-followup-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: "git",
				}},
			},
		},
	}}
	clusterTasks := []*v1alpha1.ClusterTask{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unit-test-cluster-task",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: "git",
				}},
				Params: []v1alpha1.TaskParam{{
					Name:        "foo",
					Description: "foo",
				}, {
					Name:        "bar",
					Description: "bar",
				}},
			},
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "image-to-use",
					Type: "image",
				}, {
					Name: "workspace",
					Type: "git",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-followup-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: "git",
				}},
			},
		},
	}}
	rs := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-repo",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "git",
			Params: []v1alpha1.Param{{
				Name:  "url",
				Value: "http://github.com/kristoff/reindeer",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-image",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "image",
			Params: []v1alpha1.Param{{
				Name:  "url",
				Value: "gcr.io/sven",
			}},
		},
	}}
	d := test.Data{
		PipelineRuns:      prs,
		Pipelines:         ps,
		Tasks:             ts,
		ClusterTasks:      clusterTasks,
		PipelineResources: rs,
	}

	testAssets := test.GetPipelineRunController(d)
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
	trueB := true
	expectedTaskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-success-unit-test-1",
			Namespace: "foo",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "pipeline.knative.dev/v1alpha1",
				Kind:               "PipelineRun",
				Name:               "test-pipeline-run-success",
				Controller:         &trueB,
				BlockOwnerDeletion: &trueB,
			}},
			Labels: map[string]string{
				"pipeline.knative.dev/pipeline":    "test-pipeline",
				"pipeline.knative.dev/pipelineRun": "test-pipeline-run-success",
			},
		},
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "test-sa",
			TaskRef: &v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{
					{
						Name:  "foo",
						Value: "somethingfun",
					},
					{
						Name:  "bar",
						Value: "somethingmorefun",
					},
					{
						Name:  "templatedparam",
						Value: "${inputs.workspace.revision}",
					},
				},
				Resources: []v1alpha1.TaskResourceBinding{{
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "some-repo",
					},
					Name: "workspace",
				}},
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					Name:        "image-to-use",
					ResourceRef: v1alpha1.PipelineResourceRef{Name: "some-image"},
					Paths:       []string{"/pvc/unit-test-1/image-to-use"},
				}, {
					Name:        "workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{Name: "some-repo"},
					Paths:       []string{"/pvc/unit-test-1/workspace"},
				}},
			},
		},
	}

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
	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "a-task-that-exists",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{},
	}}
	ps := []*v1alpha1.Pipeline{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pipeline-missing-tasks",
				Namespace: "foo",
			},
			Spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{{
				Name:    "myspecialtask",
				TaskRef: v1alpha1.TaskRef{Name: "sometask"},
			}},
			}},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "a-fine-pipeline",
				Namespace: "foo",
			},
			Spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{{
				Name:    "some-task",
				TaskRef: v1alpha1.TaskRef{Name: "a-task-that-exists"},
			}},
			}},
	}
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "pipeline-not-exist",
			},
		}}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun-missing-tasks",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "pipeline-missing-tasks",
			},
		}}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipelinerun-params-dont-exist",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "a-fine-pipeline",
			},
		}},
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
			reason:      pipelinerun.ReasonCouldntGetPipeline,
		}, {
			name:        "invalid-pipeline-run-missing-tasks-shd-stop-reconciling",
			pipelineRun: prs[1],
			reason:      pipelinerun.ReasonCouldntGetTask,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			testAssets := test.GetPipelineRunController(d)
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
			testAssets := test.GetPipelineRunController(test.Data{})
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
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline",
			},
		},
	}
	pipelineTask := v1alpha1.PipelineTask{
		Name:    "unit-test-1",
		TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
	}
	task := &v1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "workspace",
					Type: "git",
				}},
			},
		},
	}
	taskrun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-success-unit-test-1",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "test-sa",
			TaskRef: &v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
		},
		Status: v1alpha1.TaskRunStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type: duckv1alpha1.ConditionSucceeded,
			}},
			Steps: []v1alpha1.StepState{{
				ContainerState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
				},
			}},
		},
	}

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
	pipelinerun.UpdateTaskRunsStatus(pr, state)
	if d := cmp.Diff(pr.Status.TaskRuns, expectedPipelineRunStatus.TaskRuns); d != "" {
		t.Fatalf("Expected PipelineRun status to match TaskRun(s) status, but got a mismatch: %s", d)
	}

}

func TestReconcileOnCompletedPipelineRun(t *testing.T) {
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-completed",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline",
			},
			ServiceAccount: "test-sa",
		},
		Status: v1alpha1.PipelineRunStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:    duckv1alpha1.ConditionSucceeded,
				Status:  corev1.ConditionTrue,
				Reason:  resources.ReasonSucceeded,
				Message: "All Tasks have completed executing",
			}},
		},
	}}
	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "hello-world-1",
				TaskRef: v1alpha1.TaskRef{Name: "hello-world"},
			}},
		},
	}}
	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hello-world",
			Namespace: "foo",
		},
	}}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
		Tasks:        ts,
	}

	testAssets := test.GetPipelineRunController(d)
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
