package pipelinerun_test

import (
	"context"
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/test"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_InvalidPipelineTask(t *testing.T) {
	ps := []*v1alpha1.Pipeline{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-bad-inputbindings",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
				InputSourceBindings: []v1alpha1.SourceBinding{{
					Key: "test-resource-name",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "non-exitent-resource1",
					},
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-bad-outputbindings",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-test-task"},
				OutputSourceBindings: []v1alpha1.SourceBinding{{
					Key: "test-resource-name",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "non-exitent-resource",
					},
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-bad-inputkey",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-task-wrong-input"},
				InputSourceBindings: []v1alpha1.SourceBinding{{
					Key: "non-existent",
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-bad-outputkey",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-task-wrong-output"},
				InputSourceBindings: []v1alpha1.SourceBinding{{
					Key: "non-existent",
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
				Resources: []v1alpha1.TaskResource{{}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-wrong-input",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "testinput",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-wrong-output",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Outputs: &v1alpha1.Outputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "testoutput",
				}},
			},
		},
	}}

	tcs := []struct {
		name     string
		pipeline *v1alpha1.Pipeline
		reason   string
	}{
		{
			name:     "bad-input-source-bindings",
			pipeline: ps[0],
			reason:   "input-source-binding-to-invalid-resource",
		}, {
			name:     "bad-output-source-bindings",
			pipeline: ps[1],
			reason:   "output-source-binding-to-invalid-resource",
		}, {
			name:     "bad-inputkey",
			pipeline: ps[2],
			reason:   "bad-input-mapping",
		}, {
			name:     "bad-ouputkey",
			pipeline: ps[3],
			reason:   "bad-output-mapping",
		}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			prs := []*v1alpha1.PipelineRun{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pipeline-run",
					Namespace: "foo",
				},
				Spec: v1alpha1.PipelineRunSpec{
					PipelineRef: v1alpha1.PipelineRef{
						Name: tc.pipeline.Name,
					},
				},
			}}
			d := test.Data{
				PipelineRuns: prs,
				Pipelines:    ps,
				Tasks:        ts,
			}

			c, _, _ := test.GetPipelineRunController(d)
			err := c.Reconciler.Reconcile(context.Background(), "foo/test-pipeline-run")

			if err != nil {
				t.Errorf("Did not expect to see error when reconciling invalid PipelineRun but saw %q", err)
			}
			condition := prs[0].Status.GetCondition(duckv1alpha1.ConditionSucceeded)
			if condition == nil || condition.Status != corev1.ConditionFalse {
				t.Errorf("Expected status to be failed on invalid PipelineRun %s but was: %v", tc.pipeline.Name, condition)
			}
		})
	}
}
