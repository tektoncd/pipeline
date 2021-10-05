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

package taskrun_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

func TestValidateResolvedTaskResources_ValidResources(t *testing.T) {
	ctx := context.Background()
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{
					Image:   "myimage",
					Command: []string{"mycmd"},
				},
			}},
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{
					{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:     "resource-to-build",
							Type:     resourcev1alpha1.PipelineResourceTypeGit,
							Optional: false,
						},
					},
					{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:     "optional-resource-to-build",
							Type:     resourcev1alpha1.PipelineResourceTypeGit,
							Optional: true,
						},
					},
				},
				Outputs: []v1beta1.TaskResource{
					{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:     "resource-to-provide",
							Type:     resourcev1alpha1.PipelineResourceTypeImage,
							Optional: false,
						},
					},
					{
						ResourceDeclaration: v1beta1.ResourceDeclaration{
							Name:     "optional-resource-to-provide",
							Type:     resourcev1alpha1.PipelineResourceTypeImage,
							Optional: true,
						},
					},
				},
			},
		},
	}
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
		Inputs: map[string]*resourcev1alpha1.PipelineResource{
			"resource-to-build": {
				ObjectMeta: metav1.ObjectMeta{Name: "example-resource"},
				Spec: v1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeGit,
					Params: []v1beta1.ResourceParam{{
						Name:  "foo",
						Value: "bar",
					}},
				},
			},
			"optional-resource-to-build": {
				ObjectMeta: metav1.ObjectMeta{Name: "example-resource"},
				Spec: v1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeGit,
					Params: []v1beta1.ResourceParam{{
						Name:  "foo",
						Value: "bar",
					}},
				},
			},
		},
		Outputs: map[string]*resourcev1alpha1.PipelineResource{
			"resource-to-provide": {
				ObjectMeta: metav1.ObjectMeta{Name: "example-image"},
				Spec: v1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeImage,
				},
			},
			"optional-resource-to-provide": {
				ObjectMeta: metav1.ObjectMeta{Name: "example-image"},
				Spec: v1alpha1.PipelineResourceSpec{
					Type: resourcev1alpha1.PipelineResourceTypeImage,
				},
			},
		},
	}
	if err := taskrun.ValidateResolvedTaskResources(ctx, []v1beta1.Param{}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating valid resolved TaskRun but saw %v", err)
	}
}

func TestValidateResolvedTaskResources_ValidParams(t *testing.T) {
	ctx := context.Background()
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{
					Image:   "myimage",
					Command: []string{"mycmd"},
				},
			}},
			Params: []v1beta1.ParamSpec{
				{
					Name: "foo",
					Type: v1beta1.ParamTypeString,
				},
				{
					Name: "bar",
					Type: v1beta1.ParamTypeString,
				},
			},
		},
	}
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
	}
	p := []v1beta1.Param{{
		Name:  "foo",
		Value: *v1beta1.NewArrayOrString("somethinggood"),
	}, {
		Name:  "bar",
		Value: *v1beta1.NewArrayOrString("somethinggood"),
	}}
	if err := taskrun.ValidateResolvedTaskResources(ctx, p, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}

	t.Run("alpha-extra-params", func(t *testing.T) {
		ctx := config.ToContext(ctx, &config.Config{FeatureFlags: &config.FeatureFlags{EnableAPIFields: "alpha"}})
		extra := v1beta1.Param{
			Name:  "extra",
			Value: *v1beta1.NewArrayOrString("i am an extra param"),
		}
		if err := taskrun.ValidateResolvedTaskResources(ctx, append(p, extra), rtr); err != nil {
			t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
		}
	})
}

func TestValidateResolvedTaskResources_InvalidParams(t *testing.T) {
	ctx := context.Background()
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Container: corev1.Container{
					Image:   "myimage",
					Command: []string{"mycmd"},
				},
			}},
			Params: []v1beta1.ParamSpec{
				{
					Name: "foo",
					Type: v1beta1.ParamTypeString,
				},
			},
		},
	}
	tcs := []struct {
		name   string
		rtr    *resources.ResolvedTaskResources
		params []v1beta1.Param
	}{{
		name: "missing-params",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name:  "foobar",
			Value: *v1beta1.NewArrayOrString("somethingfun"),
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := taskrun.ValidateResolvedTaskResources(ctx, tc.params, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun with wrong params but saw none")
			}
		})
	}
}

func TestValidateResolvedTaskResources_InvalidResources(t *testing.T) {
	ctx := context.Background()
	r := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{Name: "git-test-resource"},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
			Params: []resourcev1alpha1.ResourceParam{{
				Name:  "foo",
				Value: "bar",
			}},
		},
	}
	testinput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "testinput",
						Type: resourcev1alpha1.PipelineResourceTypeGit,
					},
				}},
			},
		},
	}
	testimageinput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "testimageinput",
						Type: resourcev1alpha1.PipelineResourceTypeImage,
					},
				}},
			},
		},
	}
	testrequiredgitinput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "requiredgitinput",
						Type:     resourcev1alpha1.PipelineResourceTypeGit,
						Optional: false,
					},
				}},
			},
		},
	}
	testoutput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Outputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "testoutput",
						Type: resourcev1alpha1.PipelineResourceTypeGit,
					},
				}},
			},
		},
	}
	testimageoutput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Outputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name: "testimageoutput",
						Type: resourcev1alpha1.PipelineResourceTypeImage,
					},
				}},
			},
		},
	}
	testrequiredgitoutput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Outputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "requiredgitoutput",
						Type:     resourcev1alpha1.PipelineResourceTypeGit,
						Optional: false,
					},
				}},
			},
		},
	}
	testrequiredinputandoutput := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "requiredimageinput",
						Type:     resourcev1alpha1.PipelineResourceTypeImage,
						Optional: false,
					},
				}},
				Outputs: []v1beta1.TaskResource{{
					ResourceDeclaration: v1beta1.ResourceDeclaration{
						Name:     "requiredimageoutput",
						Type:     resourcev1alpha1.PipelineResourceTypeImage,
						Optional: false,
					},
				}},
			},
		},
	}
	tcs := []struct {
		name string
		rtr  *resources.ResolvedTaskResources
	}{{
		name: "bad-inputkey",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testinput.Spec,
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{"wrong-resource-name": r},
		},
	}, {
		name: "bad-outputkey",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testoutput.Spec,
			Outputs:  map[string]*resourcev1alpha1.PipelineResource{"wrong-resource-name": r},
		},
	}, {
		name: "input-resource-mismatch",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testimageinput.Spec,
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{"testimageinput": r},
		},
	}, {
		name: "input-resource-missing",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testimageinput.Spec,
		},
	}, {
		name: "output-resource-mismatch",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testimageoutput.Spec,
			Outputs:  map[string]*resourcev1alpha1.PipelineResource{"testimageoutput": r},
		},
	}, {
		name: "output-resource-missing",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testimageoutput.Spec,
		},
	}, {
		name: "extra-input-resource",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testinput.Spec,
			Inputs: map[string]*resourcev1alpha1.PipelineResource{
				"testinput":      r,
				"someextrainput": r,
			},
		},
	}, {
		name: "extra-output-resource",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testoutput.Spec,
			Outputs: map[string]*resourcev1alpha1.PipelineResource{
				"testoutput":      r,
				"someextraoutput": r,
			},
		},
	}, {
		name: "extra-input-resource-none-required",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testoutput.Spec,
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{"someextrainput": r},
			Outputs:  map[string]*resourcev1alpha1.PipelineResource{"testoutput": r},
		},
	}, {
		name: "extra-output-resource-none-required",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testinput.Spec,
			Inputs:   map[string]*resourcev1alpha1.PipelineResource{"testinput": r},
			Outputs:  map[string]*resourcev1alpha1.PipelineResource{"someextraoutput": r},
		},
	}, {
		name: "required-input-resource-missing",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testrequiredgitinput.Spec,
		},
	}, {
		name: "required-output-resource-missing",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testrequiredgitoutput.Spec,
		},
	}, {
		name: "required-input-and-output-resource-missing",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &testrequiredinputandoutput.Spec,
		},
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := taskrun.ValidateResolvedTaskResources(ctx, []v1beta1.Param{}, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun but saw none")
			}
		})
	}
}
