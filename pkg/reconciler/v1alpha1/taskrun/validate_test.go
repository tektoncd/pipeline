package taskrun_test

import (
	"testing"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	tb "github.com/knative/build-pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

var (
	validBuildSteps = []corev1.Container{{
		Name:    "mystep",
		Image:   "myimage",
		Command: []string{"mycmd"},
	}}
	validBuildStepFn = tb.Step("mystep", "myimage", tb.Command("mycmd"))
)

func TestValidateResolvedTaskResources_ValidResources(t *testing.T) {
	rtr := tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
		validBuildStepFn,
		tb.TaskInputs(tb.InputsResource("resource-to-build", v1alpha1.PipelineResourceTypeGit)),
	), tb.ResolvedTaskResourcesInputs("resource-to-build", tb.PipelineResource("example-resource", "foo",
		tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit,
			tb.PipelineResourceSpecParam("foo", "bar"),
		)),
	))
	if err := taskrun.ValidateResolvedTaskResources([]v1alpha1.Param{}, rtr); err != nil {
		t.Fatalf("Did not expect to see error when validating valid resolved TaskRun but saw %v", err)
	}
}

func TestValidateResolvedTaskResources_ValidParams(t *testing.T) {
	rtr := tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
		validBuildStepFn,
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
	rtr := tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
		validBuildStepFn,
		tb.TaskInputs(tb.InputsParam("foo"), tb.InputsParam("bar")),
	))
	p := []v1alpha1.Param{{
		Name:  "foobar",
		Value: "somethingfun",
	}}
	if err := taskrun.ValidateResolvedTaskResources(p, rtr); err == nil {
		t.Errorf("Expected to see error when validating invalid resolved TaskRun with wrong params but saw none")
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
		), tb.ResolvedTaskResourcesInputs("wrong-resource-name", r)),
	}, {
		name: "input-resource-mismatch",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskInputs(tb.InputsResource("testimageinput", v1alpha1.PipelineResourceTypeImage)),
		), tb.ResolvedTaskResourcesInputs("testimageinput", r)),
	}, {
		name: "output-resource-mismatch",
		rtr: tb.ResolvedTaskResources(tb.ResolvedTaskResourcesTaskSpec(
			tb.TaskOutputs(tb.OutputsResource("testimageoutput", v1alpha1.PipelineResourceTypeImage)),
		), tb.ResolvedTaskResourcesInputs("testimageoutput", r)),
	}}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if err := taskrun.ValidateResolvedTaskResources([]v1alpha1.Param{}, tc.rtr); err == nil {
				t.Errorf("Expected to see error when validating invalid resolved TaskRun but saw none")
			}
		})
	}
}
