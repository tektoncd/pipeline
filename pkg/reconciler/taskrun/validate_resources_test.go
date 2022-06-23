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

package taskrun

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
				Image:   "myimage",
				Command: []string{"mycmd"},
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
	if err := ValidateResolvedTaskResources(ctx, []v1beta1.Param{}, []v1beta1.Param{}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating valid resolved TaskRun but saw %v", err)
	}
}

func TestValidateResolvedTaskResources_ValidParams(t *testing.T) {
	ctx := context.Background()
	task := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Image:   "myimage",
				Command: []string{"mycmd"},
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
				{
					Name: "zoo",
					Type: v1beta1.ParamTypeString,
				}, {
					Name: "myobj",
					Type: v1beta1.ParamTypeObject,
					Properties: map[string]v1beta1.PropertySpec{
						"key1": {},
						"key2": {},
					},
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
	}, {
		Name: "myobj",
		Value: *v1beta1.NewObject(map[string]string{
			"key1":      "val1",
			"key2":      "val2",
			"extra_key": "val3",
		}),
	}}
	m := []v1beta1.Param{{
		Name:  "zoo",
		Value: *v1beta1.NewArrayOrString("a", "b", "c"),
	}}
	if err := ValidateResolvedTaskResources(ctx, p, m, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}

	t.Run("alpha-extra-params", func(t *testing.T) {
		ctx := config.ToContext(ctx, &config.Config{FeatureFlags: &config.FeatureFlags{EnableAPIFields: "alpha"}})
		extra := v1beta1.Param{
			Name:  "extra",
			Value: *v1beta1.NewArrayOrString("i am an extra param"),
		}
		extraarray := v1beta1.Param{
			Name:  "extraarray",
			Value: *v1beta1.NewArrayOrString("i", "am", "an", "extra", "array", "param"),
		}
		if err := ValidateResolvedTaskResources(ctx, append(p, extra), append(m, extraarray), rtr); err != nil {
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
				Image:   "myimage",
				Command: []string{"mycmd"},
			}},
			Params: []v1beta1.ParamSpec{
				{
					Name: "foo",
					Type: v1beta1.ParamTypeString,
				}, {
					Name: "bar",
					Type: v1beta1.ParamTypeArray,
				},
				{
					Name: "myobj",
					Type: v1beta1.ParamTypeObject,
					Properties: map[string]v1beta1.PropertySpec{
						"key1": {},
						"key2": {},
					},
				},
			},
		},
	}
	tcs := []struct {
		name   string
		rtr    *resources.ResolvedTaskResources
		params []v1beta1.Param
		matrix []v1beta1.Param
	}{{
		name: "missing-params",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name:  "foobar",
			Value: *v1beta1.NewArrayOrString("somethingfun"),
		}},
		matrix: []v1beta1.Param{{
			Name:  "barfoo",
			Value: *v1beta1.NewArrayOrString("bar", "foo"),
		}},
	}, {
		name: "invalid-type-in-params",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name:  "foo",
			Value: *v1beta1.NewArrayOrString("bar", "foo"),
		}},
	}, {
		name: "invalid-type-in-matrix",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		matrix: []v1beta1.Param{{
			Name:  "bar",
			Value: *v1beta1.NewArrayOrString("bar", "foo"),
		}},
	}, {
		name: "missing object param keys",
		rtr: &resources.ResolvedTaskResources{
			TaskSpec: &task.Spec,
		},
		params: []v1beta1.Param{{
			Name: "myobj",
			Value: *v1beta1.NewObject(map[string]string{
				"key1":    "val1",
				"misskey": "val2",
			}),
		}},
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateResolvedTaskResources(ctx, tc.params, tc.matrix, tc.rtr); err == nil {
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
			if err := ValidateResolvedTaskResources(ctx, []v1beta1.Param{}, []v1beta1.Param{}, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun but saw none")
			}
		})
	}
}

func TestValidateOverrides(t *testing.T) {
	tcs := []struct {
		name    string
		ts      *v1beta1.TaskSpec
		trs     *v1beta1.TaskRunSpec
		wantErr bool
	}{{
		name: "valid stepOverrides",
		ts: &v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1beta1.TaskRunSpec{
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "step1",
			}},
		},
	}, {
		name: "valid sidecarOverrides",
		ts: &v1beta1.TaskSpec{
			Sidecars: []v1beta1.Sidecar{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1beta1.TaskRunSpec{
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "step1",
			}},
		},
	}, {
		name: "invalid stepOverrides",
		ts:   &v1beta1.TaskSpec{},
		trs: &v1beta1.TaskRunSpec{
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}, {
		name: "invalid sidecarOverrides",
		ts:   &v1beta1.TaskSpec{},
		trs: &v1beta1.TaskRunSpec{
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOverrides(tc.ts, tc.trs)
			if (err != nil) != tc.wantErr {
				t.Errorf("expected err: %t, but got err %s", tc.wantErr, err)
			}
		})
	}
}
