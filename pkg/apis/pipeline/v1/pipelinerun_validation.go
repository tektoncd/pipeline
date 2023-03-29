/*
Copyright 2022 The Tekton Authors

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

package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/apis/version"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ apis.Validatable = (*PipelineRun)(nil)
var _ resourcesemantics.VerbLimited = (*PipelineRun)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (pr *PipelineRun) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate pipelinerun
func (pr *PipelineRun) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(pr.GetObjectMeta()).ViaField("metadata")

	if pr.IsPending() && pr.HasStarted() {
		errs = errs.Also(apis.ErrInvalidValue("PipelineRun cannot be Pending after it is started", "spec.status"))
	}

	return errs.Also(pr.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
}

// Validate pipelinerun spec
func (ps *PipelineRunSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	// Must have exactly one of pipelineRef and pipelineSpec.
	if ps.PipelineRef == nil && ps.PipelineSpec == nil {
		errs = errs.Also(apis.ErrMissingOneOf("pipelineRef", "pipelineSpec"))
	}
	if ps.PipelineRef != nil && ps.PipelineSpec != nil {
		errs = errs.Also(apis.ErrMultipleOneOf("pipelineRef", "pipelineSpec"))
	}

	// Validate PipelineRef if it's present
	if ps.PipelineRef != nil {
		errs = errs.Also(ps.PipelineRef.Validate(ctx).ViaField("pipelineRef"))
	}

	// Validate PipelineSpec if it's present
	if ps.PipelineSpec != nil {
		ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, true)
		errs = errs.Also(ps.PipelineSpec.Validate(ctx).ViaField("pipelineSpec"))
	}

	// Validate PipelineRun parameters
	errs = errs.Also(ps.validatePipelineRunParameters(ctx))

	// Validate propagated parameters
	errs = errs.Also(ps.validateInlineParameters(ctx))

	if ps.Timeouts != nil {
		// tasks timeout should be a valid duration of at least 0.
		errs = errs.Also(validateTimeoutDuration("tasks", ps.Timeouts.Tasks))

		// finally timeout should be a valid duration of at least 0.
		errs = errs.Also(validateTimeoutDuration("finally", ps.Timeouts.Finally))

		// pipeline timeout should be a valid duration of at least 0.
		errs = errs.Also(validateTimeoutDuration("pipeline", ps.Timeouts.Pipeline))

		if ps.Timeouts.Pipeline != nil {
			errs = errs.Also(ps.validatePipelineTimeout(ps.Timeouts.Pipeline.Duration, "should be <= pipeline duration"))
		} else {
			defaultTimeout := time.Duration(config.FromContextOrDefaults(ctx).Defaults.DefaultTimeoutMinutes)
			errs = errs.Also(ps.validatePipelineTimeout(defaultTimeout, "should be <= default timeout duration"))
		}
	}

	errs = errs.Also(validateSpecStatus(ps.Status))

	if ps.Workspaces != nil {
		wsNames := make(map[string]int)
		for idx, ws := range ps.Workspaces {
			errs = errs.Also(ws.Validate(ctx).ViaFieldIndex("workspaces", idx))
			if prevIdx, alreadyExists := wsNames[ws.Name]; alreadyExists {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("workspace %q provided by pipelinerun more than once, at index %d and %d", ws.Name, prevIdx, idx), "name").ViaFieldIndex("workspaces", idx))
			}
			wsNames[ws.Name] = idx
		}
	}
	for idx, trs := range ps.TaskRunSpecs {
		errs = errs.Also(validateTaskRunSpec(ctx, trs).ViaIndex(idx).ViaField("taskRunSpecs"))
	}

	return errs
}

func (ps *PipelineRunSpec) validatePipelineRunParameters(ctx context.Context) (errs *apis.FieldError) {
	if len(ps.Params) == 0 {
		return errs
	}

	// Validate parameter types and uniqueness
	errs = errs.Also(ValidateParameters(ctx, ps.Params).ViaField("params"))

	// Validate that task results aren't used in param values
	for _, param := range ps.Params {
		expressions, ok := GetVarSubstitutionExpressionsForParam(param)
		if ok {
			if LooksLikeContainsResultRefs(expressions) {
				expressions = filter(expressions, looksLikeResultRef)
				resultRefs := NewResultRefs(expressions)
				if len(resultRefs) > 0 {
					errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("cannot use result expressions in %v as PipelineRun parameter values", expressions),
						"value").ViaFieldKey("params", param.Name))
				}
			}
		}
	}

	return errs
}

// validateInlineParameters validates parameters that are defined inline.
// This is crucial for propagated parameters since the parameters could
// be defined under pipelineRun and then called directly in the task steps.
// In this case, parameters cannot be validated by the underlying pipelineSpec
// or taskSpec since they may not have the parameters declared because of propagation.
func (ps *PipelineRunSpec) validateInlineParameters(ctx context.Context) (errs *apis.FieldError) {
	if ps.PipelineSpec == nil {
		return errs
	}
	var paramSpec []ParamSpec
	for _, p := range ps.Params {
		pSpec := ParamSpec{
			Name:    p.Name,
			Default: &p.Value,
		}
		paramSpec = append(paramSpec, pSpec)
	}
	paramSpec = appendParamSpec(paramSpec, ps.PipelineSpec.Params)
	for _, pt := range ps.PipelineSpec.Tasks {
		paramSpec = appendParam(paramSpec, pt.Params)
		if pt.TaskSpec != nil && pt.TaskSpec.Params != nil {
			paramSpec = appendParamSpec(paramSpec, pt.TaskSpec.Params)
		}
	}
	if ps.PipelineSpec != nil && ps.PipelineSpec.Tasks != nil {
		for _, pt := range ps.PipelineSpec.Tasks {
			if pt.TaskSpec != nil && pt.TaskSpec.Steps != nil {
				errs = errs.Also(ValidateParameterVariables(
					config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, false), pt.TaskSpec.Steps, paramSpec))
			}
		}
	}
	return errs
}

func appendParamSpec(paramSpec []ParamSpec, params []ParamSpec) []ParamSpec {
	for _, p := range params {
		skip := false
		for _, ps := range paramSpec {
			if ps.Name == p.Name {
				skip = true
				break
			}
		}
		if !skip {
			paramSpec = append(paramSpec, p)
		}
	}
	return paramSpec
}

func appendParam(paramSpec []ParamSpec, params Params) []ParamSpec {
	for _, p := range params {
		skip := false
		for _, ps := range paramSpec {
			if ps.Name == p.Name {
				skip = true
				break
			}
		}
		if !skip {
			pSpec := ParamSpec{
				Name:    p.Name,
				Default: &p.Value,
			}
			paramSpec = append(paramSpec, pSpec)
		}
	}
	return paramSpec
}

func validateSpecStatus(status PipelineRunSpecStatus) *apis.FieldError {
	switch status {
	case "":
		return nil
	case PipelineRunSpecStatusPending:
		return nil
	case PipelineRunSpecStatusCancelled,
		PipelineRunSpecStatusCancelledRunFinally,
		PipelineRunSpecStatusStoppedRunFinally:
		return nil
	}

	return apis.ErrInvalidValue(fmt.Sprintf("%s should be %s, %s, %s or %s", status,
		PipelineRunSpecStatusCancelled,
		PipelineRunSpecStatusCancelledRunFinally,
		PipelineRunSpecStatusStoppedRunFinally,
		PipelineRunSpecStatusPending), "status")
}

func validateTimeoutDuration(field string, d *metav1.Duration) (errs *apis.FieldError) {
	if d != nil && d.Duration < 0 {
		fieldPath := fmt.Sprintf("timeouts.%s", field)
		return errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", d.Duration.String()), fieldPath))
	}
	return nil
}

func (ps *PipelineRunSpec) validatePipelineTimeout(timeout time.Duration, errorMsg string) (errs *apis.FieldError) {
	if ps.Timeouts.Tasks != nil {
		tasksTimeoutErr := false
		tasksTimeoutStr := ps.Timeouts.Tasks.Duration.String()
		if ps.Timeouts.Tasks.Duration > timeout && timeout != config.NoTimeoutDuration {
			tasksTimeoutErr = true
		}
		if ps.Timeouts.Tasks.Duration == config.NoTimeoutDuration && timeout != config.NoTimeoutDuration {
			tasksTimeoutErr = true
			tasksTimeoutStr += " (no timeout)"
		}
		if tasksTimeoutErr {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s %s", tasksTimeoutStr, errorMsg), "timeouts.tasks"))
		}
	}

	if ps.Timeouts.Finally != nil {
		finallyTimeoutErr := false
		finallyTimeoutStr := ps.Timeouts.Finally.Duration.String()
		if ps.Timeouts.Finally.Duration > timeout && timeout != config.NoTimeoutDuration {
			finallyTimeoutErr = true
		}
		if ps.Timeouts.Finally.Duration == config.NoTimeoutDuration && timeout != config.NoTimeoutDuration {
			finallyTimeoutErr = true
			finallyTimeoutStr += " (no timeout)"
		}
		if finallyTimeoutErr {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s %s", finallyTimeoutStr, errorMsg), "timeouts.finally"))
		}
	}

	if ps.Timeouts.Tasks != nil && ps.Timeouts.Finally != nil {
		if ps.Timeouts.Tasks.Duration+ps.Timeouts.Finally.Duration > timeout {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s + %s %s", ps.Timeouts.Tasks.Duration.String(), ps.Timeouts.Finally.Duration.String(), errorMsg), "timeouts.tasks"))
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s + %s %s", ps.Timeouts.Tasks.Duration.String(), ps.Timeouts.Finally.Duration.String(), errorMsg), "timeouts.finally"))
		}
	}
	return errs
}

func validateTaskRunSpec(ctx context.Context, trs PipelineTaskRunSpec) (errs *apis.FieldError) {
	if trs.StepSpecs != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "stepSpecs", config.AlphaAPIFields).ViaField("stepSpecs"))
		errs = errs.Also(validateStepSpecs(trs.StepSpecs).ViaField("stepSpecs"))
	}
	if trs.SidecarSpecs != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "sidecarSpecs", config.AlphaAPIFields).ViaField("sidecarSpecs"))
		errs = errs.Also(validateSidecarSpecs(trs.SidecarSpecs).ViaField("sidecarSpecs"))
	}
	if trs.ComputeResources != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "computeResources", config.AlphaAPIFields).ViaField("computeResources"))
		errs = errs.Also(validateTaskRunComputeResources(trs.ComputeResources, trs.StepSpecs))
	}
	if trs.PodTemplate != nil {
		errs = errs.Also(validatePodTemplateEnv(ctx, *trs.PodTemplate))
	}
	return errs
}
