package taskrun_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestValidateResolvedTaskResources_ValidResources(t *testing.T) {
	rtr := tb.ResolvedTaskResources(
		tb.ResolvedTaskResourcesTaskSpec(
			tb.Step("mystep", "myimage", tb.Command("mycmd")),
			tb.TaskInputs(tb.InputsResource("resource-to-build", v1alpha1.PipelineResourceTypeGit)),
			tb.TaskOutputs(tb.OutputsResource("resource-to-provide", v1alpha1.PipelineResourceTypeImage)),
		),
		tb.ResolvedTaskResourcesInputs("resource-to-build", tb.PipelineResource("example-resource", "foo",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit,
				tb.PipelineResourceSpecParam("foo", "bar"),
			))),
		tb.ResolvedTaskResourcesOutputs("resource-to-provide", tb.PipelineResource("example-image", "bar",
			tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeImage)),
		))
	if err := taskrun.ValidateResolvedTaskResources([]v1alpha1.Param{}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating valid resolved TaskRun but saw %v", err)
	}
}

func TestValidateResolvedTaskResources_ValidParams(t *testing.T) {
	rtr := tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
		tb.Step("mystep", "myimage", tb.Command("mycmd")),
		tb.TaskInputs(tb.InputsParam("foo"), tb.InputsParam("bar")),
	))
	p := []v1alpha1.Param{{
		Name:  "foo",
		Value: "somethinggood",
	}, {
		Name:  "bar",
		Value: "somethinggood",
	}}
	if err := taskrun.ValidateResolvedTaskResources(p, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating TaskRun with correct params but saw %v", err)
	}
}

func TestValidateResolvedTaskResources_InvalidParams(t *testing.T) {
	tcs := []struct {
		name   string
		rtr    *resources.ResolvedTaskResources
		params []v1alpha1.Param
	}{{
		name: "missing-params",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.Step("mystep", "myimage", tb.Command("mycmd")),
			tb.TaskInputs(tb.InputsParam("foo")),
		)),
		params: []v1alpha1.Param{{
			Name:  "foobar",
			Value: "somethingfun",
		}},
	}, {
		name: "missing-params",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.Step("mystep", "myimage", tb.Command("mycmd")),
			tb.TaskInputs(tb.InputsParam("foo")),
		)),
		params: []v1alpha1.Param{{
			Name:  "foo",
			Value: "i am a real param",
		}, {
			Name:  "extra",
			Value: "i am an extra param",
		}},
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := taskrun.ValidateResolvedTaskResources(tc.params, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun with wrong params but saw none")
			}
		})
	}
}

func TestValidateResolvedTaskResources_InvalidResources(t *testing.T) {
	r := tb.PipelineResource("git-test-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("foo", "bar"),
	))
	tcs := []struct {
		name string
		rtr  *resources.ResolvedTaskResources
	}{{
		name: "bad-inputkey",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskInputs(tb.InputsResource("testinput", v1alpha1.PipelineResourceTypeGit)),
		), tb.ResolvedTaskResourcesInputs("wrong-resource-name", r)),
	}, {
		name: "bad-outputkey",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskOutputs(tb.OutputsResource("testoutput", v1alpha1.PipelineResourceTypeGit)),
		), tb.ResolvedTaskResourcesOutputs("wrong-resource-name", r)),
	}, {
		name: "input-resource-mismatch",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskInputs(tb.InputsResource("testimageinput", v1alpha1.PipelineResourceTypeImage)),
		), tb.ResolvedTaskResourcesInputs("testimageinput", r)),
	}, {
		name: "input-resource-missing",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskInputs(tb.InputsResource("testimageinput", v1alpha1.PipelineResourceTypeImage)),
		)),
	}, {
		name: "output-resource-mismatch",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskOutputs(tb.OutputsResource("testimageoutput", v1alpha1.PipelineResourceTypeImage)),
		), tb.ResolvedTaskResourcesOutputs("testimageoutput", r)),
	}, {
		name: "output-resource-missing",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskOutputs(tb.OutputsResource("testimageoutput", v1alpha1.PipelineResourceTypeImage)),
		)),
	}, {
		name: "extra-input-resource",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskInputs(tb.InputsResource("testoutput", v1alpha1.PipelineResourceTypeGit))),
			tb.ResolvedTaskResourcesInputs("testoutput", r),
			tb.ResolvedTaskResourcesInputs("someextrainput", r),
		),
	}, {
		name: "extra-output-resource",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskOutputs(tb.OutputsResource("testoutput", v1alpha1.PipelineResourceTypeGit))),
			tb.ResolvedTaskResourcesOutputs("testoutput", r),
			tb.ResolvedTaskResourcesOutputs("someextraoutput", r),
		),
	}, {
		name: "extra-input-resource-none-required",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskOutputs(tb.OutputsResource("testoutput", v1alpha1.PipelineResourceTypeGit))),
			tb.ResolvedTaskResourcesOutputs("testoutput", r),
			tb.ResolvedTaskResourcesInputs("someextrainput", r),
		),
	}, {
		name: "extra-output-resource-none-required",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskInputs(tb.InputsResource("testinput", v1alpha1.PipelineResourceTypeGit))),
			tb.ResolvedTaskResourcesInputs("testinput", r),
			tb.ResolvedTaskResourcesOutputs("someextraoutput", r),
		),
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := taskrun.ValidateResolvedTaskResources([]v1alpha1.Param{}, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun but saw none")
			}
		})
	}
}
