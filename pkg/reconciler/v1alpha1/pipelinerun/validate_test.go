package pipelinerun_test

import (
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/pipelinerun"
	"k8s.io/apimachinery/pkg/api/errors"
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
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-param-mismatch",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-task-multiple-params"},
				Params: []v1alpha1.Param{{
					Name:  "foobar",
					Value: "somethingfun",
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-bad-resourcetype",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineSpec{
			Tasks: []v1alpha1.PipelineTask{{
				Name:    "unit-test-1",
				TaskRef: v1alpha1.TaskRef{Name: "unit-task-bad-resourcetype"},
			}},
		},
	}}
	prs := []*v1alpha1.PipelineRun{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-1",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline-bad-inputbindings",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
				Inputs: []v1alpha1.TaskResourceBinding{{
					// TODO(#213): these are currently covering both the case where the name of the
					// resource is bad and the referred resource doesn't exist (the task has no inputs)
					Name: "test-resource-name",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "non-existent-resource1",
					},
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-2",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline-bad-outputbindings",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
				Outputs: []v1alpha1.TaskResourceBinding{{
					// TODO(#213): these are currently covering both the case where the name of the
					// resource is bad and the referred resource doesn't exist (the task has no outputs)
					Name: "test-resource-name",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "non-existent-resource1",
					},
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-3",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline-bad-inputkey",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
				Inputs: []v1alpha1.TaskResourceBinding{{
					Name: "non-existent",
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-4",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline-bad-outputkey",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
				Outputs: []v1alpha1.TaskResourceBinding{{
					Name: "non-existent",
				}},
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-5",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline-param-mismatch",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline-run-6",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "test-pipeline-bad-resourcetype",
			},
			PipelineTaskResources: []v1alpha1.PipelineTaskResource{{
				Name: "unit-test-1",
				Inputs: []v1alpha1.TaskResourceBinding{{
					Name: "testimageinput",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "git-test-resource",
					},
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
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-multiple-params",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Params: []v1alpha1.TaskParam{{
					Name:        "foo",
					Description: "foo",
				}, {
					Name:        "bar",
					Description: "bar",
				}},
			},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unit-task-bad-resourcetype",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskSpec{
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "testimageinput",
					Type: v1alpha1.PipelineResourceTypeImage,
				}},
			},
		},
	}}

	rr := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-test-resource",
			Namespace: "foo",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: v1alpha1.PipelineResourceTypeGit,
			Params: []v1alpha1.Param{{
				Name:  "foo",
				Value: "bar",
			}},
		},
	}}

	tcs := []struct {
		name        string
		pipeline    *v1alpha1.Pipeline
		pipelineRun *v1alpha1.PipelineRun
		reason      string
	}{
		{
			name:        "bad-input-source-bindings",
			pipeline:    ps[0],
			pipelineRun: prs[0],
			reason:      "input-source-binding-to-invalid-resource",
		}, {
			name:        "bad-output-source-bindings",
			pipeline:    ps[1],
			pipelineRun: prs[1],
			reason:      "output-source-binding-to-invalid-resource",
		}, {
			name:        "bad-inputkey",
			pipeline:    ps[2],
			pipelineRun: prs[2],
			reason:      "bad-input-mapping",
		}, {
			name:        "bad-outputkey",
			pipeline:    ps[3],
			pipelineRun: prs[3],
			reason:      "bad-output-mapping",
		}, {
			name:        "param-mismatch",
			pipeline:    ps[4],
			pipelineRun: prs[4],
			reason:      "input-param-mismatch",
		}, {
			name:        "resource-mismatch",
			pipeline:    ps[5],
			pipelineRun: prs[5],
			reason:      "input-resource-mismatch",
		}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			getPipeline := func(name string) (*v1alpha1.Pipeline, error) {
				for _, p := range ps {
					if p.Name == name {
						return p, nil
					}
				}
				return nil, errors.NewNotFound(v1alpha1.Resource("pipeline"), name)
			}
			getTask := func(name string) (*v1alpha1.Task, error) {
				for _, t := range ts {
					if t.Name == name {
						return t, nil
					}
				}
				return nil, errors.NewNotFound(v1alpha1.Resource("task"), name)
			}
			getResource := func(name string) (*v1alpha1.PipelineResource, error) {
				for _, r := range rr {
					if r.Name == name {
						return r, nil
					}
				}
				return nil, errors.NewNotFound(v1alpha1.Resource("pipelineResource"), name)
			}
			_, _, err := pipelinerun.ValidatePipelineRun(tc.pipelineRun, getPipeline, getTask, getResource)
			if err == nil {
				t.Errorf("Expected error from validating invalid PipelineRun but was none")
			}
		})
	}
}
