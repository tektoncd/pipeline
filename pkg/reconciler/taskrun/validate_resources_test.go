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

	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
)

func TestValidateResolvedTaskResources_ValidResources(t *testing.T) {
	ctx := context.Background()
	task := tb.Task("foo", tb.TaskSpec(
		tb.Step("myimage", tb.StepCommand("mycmd")),
		tb.TaskResources(
			tb.TaskResourcesInput("resource-to-build", resourcev1alpha1.PipelineResourceTypeGit),
			tb.TaskResourcesInput("optional-resource-to-build", resourcev1alpha1.PipelineResourceTypeGit, tb.ResourceOptional(true)),
			tb.TaskResourcesOutput("resource-to-provide", resourcev1alpha1.PipelineResourceTypeImage),
			tb.TaskResourcesOutput("optional-resource-to-provide", resourcev1alpha1.PipelineResourceTypeImage, tb.ResourceOptional(true)),
		),
	))
	rtr := &resources.ResolvedTaskResources{
		TaskSpec: &task.Spec,
		Inputs: map[string]*resourcev1alpha1.PipelineResource{
			"resource-to-build": tb.PipelineResource("example-resource",
				tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeGit,
					tb.PipelineResourceSpecParam("foo", "bar"),
				)),
			"optional-resource-to-build": tb.PipelineResource("example-resource",
				tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeGit,
					tb.PipelineResourceSpecParam("foo", "bar"),
				)),
		},
		Outputs: map[string]*resourcev1alpha1.PipelineResource{
			"resource-to-provide": tb.PipelineResource("example-image",
				tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeImage)),
			"optional-resource-to-provide": tb.PipelineResource("example-image",
				tb.PipelineResourceSpec(resourcev1alpha1.PipelineResourceTypeImage)),
		},
	}
	if err := taskrun.ValidateResolvedTaskResources(ctx, []v1beta1.Param{}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating valid resolved TaskRun but saw %v", err)
	}
}

func TestValidateResolvedTaskResources_ValidParams(t *testing.T) {
	ctx := context.Background()
	task := tb.Task("foo", tb.TaskSpec(
		tb.Step("myimage", tb.StepCommand("mycmd")),
		tb.TaskParam("foo", v1beta1.ParamTypeString),
		tb.TaskParam("bar", v1beta1.ParamTypeString),
	))
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
	task := tb.Task("foo", tb.TaskSpec(
		tb.Step("myimage", tb.StepCommand("mycmd")),
		tb.TaskParam("foo", v1beta1.ParamTypeString),
	))
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
	r := tb.PipelineResource("git-test-resource", tb.PipelineResourceSpec(
		resourcev1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("foo", "bar"),
	))
	testinput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(tb.TaskResourcesInput("testinput", resourcev1alpha1.PipelineResourceTypeGit)),
		// tb.TaskResources(tb.TaskResourcesInput()),
	))
	testimageinput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(tb.TaskResourcesInput("testimageinput", resourcev1alpha1.PipelineResourceTypeImage)),
	))
	testrequiredgitinput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(tb.TaskResourcesInput("requiredgitinput", resourcev1alpha1.PipelineResourceTypeGit,
			tb.ResourceOptional(false))),
	))
	testoutput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(tb.TaskResourcesOutput("testoutput", resourcev1alpha1.PipelineResourceTypeGit)),
	))
	testimageoutput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(tb.TaskResourcesOutput("testimageoutput", resourcev1alpha1.PipelineResourceTypeImage)),
	))
	testrequiredgitoutput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(tb.TaskResourcesOutput(
			"requiredgitoutput", resourcev1alpha1.PipelineResourceTypeGit,
			tb.ResourceOptional(false)),
		),
	))
	testrequiredinputandoutput := tb.Task("foo", tb.TaskSpec(
		tb.TaskResources(
			tb.TaskResourcesInput("requiredimageinput", resourcev1alpha1.PipelineResourceTypeImage,
				tb.ResourceOptional(false)),
			tb.TaskResourcesOutput("requiredimageoutput", resourcev1alpha1.PipelineResourceTypeImage,
				tb.ResourceOptional(false)),
		),
	))
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
