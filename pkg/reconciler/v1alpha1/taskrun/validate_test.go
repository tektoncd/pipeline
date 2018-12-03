package taskrun_test

import (
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var validBuildSteps = []corev1.Container{{
	Name:    "mystep",
	Image:   "myimage",
	Command: []string{"mycmd"},
}}

func TestValidateTaskRunAndTask(t *testing.T) {
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &v1alpha1.TaskSpec{
			Steps: validBuildSteps,
			Inputs: &v1alpha1.Inputs{
				Resources: []v1alpha1.TaskResource{{
					Name: "resource-to-build",
					Type: v1alpha1.PipelineResourceTypeGit,
				}},
			},
		},
		Inputs: map[string]*v1alpha1.PipelineResource{
			"resource-to-build": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-resource",
					Namespace: "foo",
				},
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeGit,
					Params: []v1alpha1.Param{{
						Name:  "foo",
						Value: "bar",
					}},
				},
			}},
	}
	if err := taskrun.ValidateTaskRunAndTask([]v1alpha1.Param{}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating valid resolved TaskRun but saw %v", err)
	}
}

func Test_ValidParams(t *testing.T) {
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &v1alpha1.TaskSpec{
			Steps: validBuildSteps,
			Inputs: &v1alpha1.Inputs{
				Params: []v1alpha1.TaskParam{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
		},
	}
	p := []v1alpha1.Param{{
		Name:  "foo",
		Value: "somethinggood",
	}, {
		Name:  "bar",
		Value: "somethinggood",
	}}
	if err := taskrun.ValidateTaskRunAndTask(p, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}
}

func Test_InvalidParams(t *testing.T) {
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &v1alpha1.TaskSpec{
			Steps: validBuildSteps,
			Inputs: &v1alpha1.Inputs{
				Params: []v1alpha1.TaskParam{{
					Name: "foo",
				}, {
					Name: "bar",
				}},
			},
		},
	}
	p := []v1alpha1.Param{{
		Name:  "foobar",
		Value: "somethingfun",
	}}
	if err := taskrun.ValidateTaskRunAndTask(p, rtr); err == nil {
		t.Errorf("Expected to see error when validating invalid resolved TaskRun with wrong params but saw none")
	}
}

func Test_InvalidTaskRunTask(t *testing.T) {
	r := &v1alpha1.PipelineResource{
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
	}
	tcs := []struct {
		name string
		rtr  *resources.ResolvedTaskResources
	}{{
		name: "bad-inputkey",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "testinput",
					}},
				},
			},
			Inputs: map[string]*v1alpha1.PipelineResource{"wrong-resource-name": r},
		},
	}, {
		name: "bad-outputkey",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "testoutput",
					}},
				},
			},
			Outputs: map[string]*v1alpha1.PipelineResource{"wrong-resource-name": r},
		},
	}, {
		name: "input-resource-mismatch",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "testimageinput",
						Type: v1alpha1.PipelineResourceTypeImage,
					}},
				},
			},
			Inputs: map[string]*v1alpha1.PipelineResource{"testimageinput": r},
		},
	}, {
		name: "output-resource-mismatch",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &v1alpha1.TaskSpec{
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "testimageoutput",
						Type: v1alpha1.PipelineResourceTypeImage,
					}},
				},
			},
			Outputs: map[string]*v1alpha1.PipelineResource{"testimageoutput": r},
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := taskrun.ValidateTaskRunAndTask([]v1alpha1.Param{}, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun but saw none")
			}
		})
	}
}
