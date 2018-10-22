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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/test"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktesting "k8s.io/client-go/testing"
)

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
			PipelineParamsRef: v1alpha1.PipelineParamsRef{
				Name: "unit-test-pp",
			},
		},
	}}
	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{{
			Name:    "unit-test-1",
			TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
			Params: []v1alpha1.Param{{
				Name:  "foo",
				Value: "somethingfun",
			}},
		}},
		}},
	}
	ts := []*v1alpha1.Task{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-task",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Params: []v1alpha1.TaskParam{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
		},
	}}
	pp := []*v1alpha1.PipelineParams{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-test-pp",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineParamsSpec{
			ServiceAccount: "test-sa",
		},
	}}
	d := test.Data{
		PipelineRuns:   prs,
		Pipelines:      ps,
		Tasks:          ts,
		PipelineParams: pp,
	}
	c, _, client := test.GetPipelineRunController(d)
	err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run-success")
	if err != nil {
		t.Errorf("Did not expect to see error when reconciling valid Pipeline but saw %s", err)
	}
	if len(client.Actions()) == 0 {
		t.Fatalf("Expected client to have been used to create a TaskRun but it wasn't")
	}

	// Check that the PipelineRun was reconciled correctly
	reconciledRun, err := client.Pipeline().PipelineRuns("foo").Get("test-pipeline-run-success", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Somehow had error getting reconciled run out of fake client: %s", err)
	}
	condition := reconciledRun.Status.GetCondition(duckv1alpha1.ConditionSucceeded)
	if condition == nil || condition.Status != corev1.ConditionUnknown {
		t.Errorf("Expected PipelineRun status to be in progress, but was %s", condition)
	}

	// Check that the expected TaskRun was created
	actual := client.Actions()[0].(ktesting.CreateAction).GetObject()
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
		},
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "test-sa",
			TaskRef: v1alpha1.TaskRef{
				Name: "unit-test-task",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Params: []v1alpha1.Param{{
					Name:  "foo",
					Value: "somethingfun",
				}},
			},
		},
	}
	if d := cmp.Diff(actual, expectedTaskRun); d != "" {
		t.Errorf("expected to see TaskRun %v created. Diff %s", expectedTaskRun, d)
	}
}

func TestReconcile_InvalidPipelineRuns(t *testing.T) {
	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-missing-tasks",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{Tasks: []v1alpha1.PipelineTask{{
			Name:    "myspecialtask",
			TaskRef: v1alpha1.TaskRef{Name: "sometask"},
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
		}},
	}
	d := test.Data{
		PipelineRuns: prs,
		Pipelines:    ps,
	}
	tcs := []struct {
		name        string
		pipelineRun string
		log         string
	}{
		{
			name:        "invalid-pipeline-run-shd-stop-reconciling",
			pipelineRun: "foo/test-pipeline-run-doesnot-exist",
			log:         "pipeline run \"foo/test-pipeline-run-doesnot-exist\" in work queue no longer exists",
		}, {
			name:        "invalid-pipeline-shd-be-stop-reconciling",
			pipelineRun: "foo/invalid-pipeline",
			log:         "\"foo/invalid-pipeline\" failed to Get Pipeline: \"foo/pipeline-not-exist\"",
		}, {
			name:        "invalid-pipeline-run-name-shd-stop-reconciling",
			pipelineRun: "test/pipeline-fail/t",
			log:         "invalid resource key: test/pipeline-fail/t",
		}, {
			name:        "invalid-pipeline-run-missing-tasks-shd-stop-reconciling",
			pipelineRun: "foo/pipelinerun-missing-tasks",
			log:         "PipelineRun foo/pipeline-missing-tasks's Pipeline foo/pipelinerun-missing-tasks can't be Run; it contains Tasks that don't exist: task.pipeline.knative.dev \"sometask\" not found",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c, logs, _ := test.GetPipelineRunController(d)
			err := c.Reconciler.Reconcile(context.Background(), tc.pipelineRun)
			// When a PipelineRun is invalid and can't run, we don't want to return an error because
			// an error will tell the Reconciler to keep trying to reconcile; instead we want to stop
			// and forget about the Run.
			if err != nil {
				t.Errorf("Did not expect to see error when reconciling but saw %s", err)
			}
			if tc.log != "" && logs.FilterMessage(tc.log).Len() == 0 {
				m := test.GetLogMessages(logs)
				t.Errorf("Log lines diff %s", cmp.Diff(tc.log, m))
			}
		})
	}
}
