/*
Copyright 2020 The Tekton Authors

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

package v1beta1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/strings/slices"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var (
	_ apis.Validatable = (*PipelineRun)(nil)
	_ resourcesemantics.VerbLimited
)

// SupportedVerbs returns the operations that validation should be called for
func (pr *PipelineRun) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate pipelinerun
func (pr *PipelineRun) Validate(ctx context.Context) *apis.FieldError {
	if apis.IsInDelete(ctx) {
		return nil
	}

	errs := validate.ObjectMetadata(pr.GetObjectMeta()).ViaField("metadata")

	if pr.IsPending() && pr.HasStarted() {
		errs = errs.Also(apis.ErrInvalidValue("PipelineRun cannot be Pending after it is started", "spec.status"))
	}

	return errs.Also(pr.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
}

// Validate pipelinerun spec
func (ps *PipelineRunSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	// Validate the spec changes
	errs = errs.Also(ps.ValidateUpdate(ctx))

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
		if slices.Contains(strings.Split(
			config.FromContextOrDefaults(ctx).FeatureFlags.DisableInlineSpec, ","), "pipelinerun") {
			errs = errs.Also(apis.ErrDisallowedFields("pipelineSpec"))
		}
		errs = errs.Also(ps.PipelineSpec.Validate(ctx).ViaField("pipelineSpec"))
	}

	// Validate PipelineRun parameters
	errs = errs.Also(ps.validatePipelineRunParameters(ctx))

	// Validate propagated parameters
	errs = errs.Also(ps.validateInlineParameters(ctx))
	// Validate propagated workspaces
	errs = errs.Also(ps.validatePropagatedWorkspaces(ctx))

	if ps.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ps.Timeout.Duration < 0 {
			errs = errs.Also(apis.ErrInvalidValue(ps.Timeout.Duration.String()+" should be >= 0", "timeout"))
		}
	}

	if ps.Timeouts != nil {
		if ps.Timeout != nil {
			// can't have both at the same time
			errs = errs.Also(apis.ErrDisallowedFields("timeout", "timeouts"))
		}

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
	if ps.PodTemplate != nil {
		errs = errs.Also(validatePodTemplateEnv(ctx, *ps.PodTemplate))
	}
	if ps.Resources != nil {
		errs = errs.Also(apis.ErrDisallowedFields("resources"))
	}

	return errs
}

// ValidateUpdate validates the update of a PipelineRunSpec
func (ps *PipelineRunSpec) ValidateUpdate(ctx context.Context) (errs *apis.FieldError) {
	if !apis.IsInUpdate(ctx) {
		return
	}
	oldObj, ok := apis.GetBaseline(ctx).(*PipelineRun)
	if !ok || oldObj == nil {
		return
	}
	old := &oldObj.Spec

	// If already in the done state, the spec cannot be modified. Otherwise, only the status field can be modified.
	tips := "Once the PipelineRun is complete, no updates are allowed"
	if !oldObj.IsDone() {
		old = old.DeepCopy()
		old.Status = ps.Status
		tips = "Once the PipelineRun has started, only status updates are allowed"
	}
	if !equality.Semantic.DeepEqual(old, ps) {
		errs = errs.Also(apis.ErrInvalidValue(tips, ""))
	}

	return
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
				expressions = filter(expressions, resultref.LooksLikeResultRef)
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

// validatePropagatedWorkspaces validates workspaces that are propagated.
func (ps *PipelineRunSpec) validatePropagatedWorkspaces(ctx context.Context) (errs *apis.FieldError) {
	if ps.PipelineSpec == nil {
		return errs
	}
	workspaceNames := sets.NewString()
	for _, w := range ps.Workspaces {
		workspaceNames.Insert(w.Name)
	}

	for _, w := range ps.PipelineSpec.Workspaces {
		workspaceNames.Insert(w.Name)
	}

	for i, pt := range ps.PipelineSpec.Tasks {
		for _, w := range pt.Workspaces {
			workspaceNames.Insert(w.Name)
		}
		errs = errs.Also(pt.validateWorkspaces(workspaceNames).ViaIndex(i))
	}
	for i, pt := range ps.PipelineSpec.Finally {
		for _, w := range pt.Workspaces {
			workspaceNames.Insert(w.Name)
		}
		errs = errs.Also(pt.validateWorkspaces(workspaceNames).ViaIndex(i))
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
	paramSpecForValidation := make(map[string]ParamSpec)
	for _, p := range ps.Params {
		paramSpecForValidation = createParamSpecFromParam(p, paramSpecForValidation)
	}
	for _, p := range ps.PipelineSpec.Params {
		var err *apis.FieldError
		paramSpecForValidation, err = combineParamSpec(p, paramSpecForValidation)
		if err != nil {
			errs = errs.Also(err)
		}
	}
	for _, pt := range ps.PipelineSpec.Tasks {
		paramSpecForValidation = appendPipelineTaskParams(paramSpecForValidation, pt.Params)
		if pt.TaskSpec != nil && pt.TaskSpec.Params != nil {
			for _, p := range pt.TaskSpec.Params {
				var err *apis.FieldError
				paramSpecForValidation, err = combineParamSpec(p, paramSpecForValidation)
				if err != nil {
					errs = errs.Also(err)
				}
			}
		}
	}
	var paramSpec []ParamSpec
	for _, v := range paramSpecForValidation {
		paramSpec = append(paramSpec, v)
	}
	if ps.PipelineSpec != nil && ps.PipelineSpec.Tasks != nil {
		for _, pt := range ps.PipelineSpec.Tasks {
			if pt.TaskSpec != nil && pt.TaskSpec.Steps != nil {
				errs = errs.Also(ValidateParameterTypes(ctx, paramSpec))
				errs = errs.Also(ValidateParameterVariables(ctx, pt.TaskSpec.Steps, paramSpec))
				errs = errs.Also(ValidateUsageOfDeclaredParameters(ctx, pt.TaskSpec.Steps, paramSpec))
			}
		}
		errs = errs.Also(ValidatePipelineParameterVariables(ctx, ps.PipelineSpec.Tasks, paramSpec))
		errs = errs.Also(validatePipelineTaskParameterUsage(ps.PipelineSpec.Tasks, paramSpec))
	}
	return errs
}

func appendPipelineTaskParams(paramSpecForValidation map[string]ParamSpec, params Params) map[string]ParamSpec {
	for _, p := range params {
		if pSpec, ok := paramSpecForValidation[p.Name]; ok {
			if p.Value.ObjectVal != nil {
				for k, v := range p.Value.ObjectVal {
					pSpec.Default.ObjectVal[k] = v
					pSpec.Properties[k] = PropertySpec{Type: ParamTypeString}
				}
			}
			paramSpecForValidation[p.Name] = pSpec
		} else {
			paramSpecForValidation = createParamSpecFromParam(p, paramSpecForValidation)
		}
	}
	return paramSpecForValidation
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
		fieldPath := "timeouts." + field
		return errs.Also(apis.ErrInvalidValue(d.Duration.String()+" should be >= 0", fieldPath))
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
	if trs.StepOverrides != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "stepOverrides", config.BetaAPIFields).ViaField("stepOverrides"))
		errs = errs.Also(validateStepOverrides(trs.StepOverrides).ViaField("stepOverrides"))
	}
	if trs.SidecarOverrides != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "sidecarOverrides", config.BetaAPIFields).ViaField("sidecarOverrides"))
		errs = errs.Also(validateSidecarOverrides(trs.SidecarOverrides).ViaField("sidecarOverrides"))
	}
	if trs.ComputeResources != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "computeResources", config.BetaAPIFields).ViaField("computeResources"))
		errs = errs.Also(validateTaskRunComputeResources(trs.ComputeResources, trs.StepOverrides))
	}
	if trs.TaskPodTemplate != nil {
		errs = errs.Also(validatePodTemplateEnv(ctx, *trs.TaskPodTemplate))
	}
	return errs
}
