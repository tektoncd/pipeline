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
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/apis/version"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/strings/slices"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var _ apis.Validatable = (*TaskRun)(nil)
var _ resourcesemantics.VerbLimited = (*TaskRun)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (tr *TaskRun) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate taskrun
func (tr *TaskRun) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(tr.GetObjectMeta()).ViaField("metadata")
	return errs.Also(tr.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
}

// Validate taskrun spec
func (ts *TaskRunSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	// Must have exactly one of taskRef and taskSpec.
	if ts.TaskRef == nil && ts.TaskSpec == nil {
		errs = errs.Also(apis.ErrMissingOneOf("taskRef", "taskSpec"))
	}
	if ts.TaskRef != nil && ts.TaskSpec != nil {
		errs = errs.Also(apis.ErrMultipleOneOf("taskRef", "taskSpec"))
	}
	// Validate TaskRef if it's present.
	if ts.TaskRef != nil {
		errs = errs.Also(ts.TaskRef.Validate(ctx).ViaField("taskRef"))
	}
	// Validate TaskSpec if it's present.
	if ts.TaskSpec != nil {
		errs = errs.Also(ts.TaskSpec.Validate(ctx).ViaField("taskSpec"))
	}

	errs = errs.Also(ValidateParameters(ctx, ts.Params).ViaField("params"))

	// Validate propagated parameters
	errs = errs.Also(ts.validateInlineParameters(ctx))
	errs = errs.Also(ValidateWorkspaceBindings(ctx, ts.Workspaces).ViaField("workspaces"))
	if ts.Debug != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "debug", config.AlphaAPIFields).ViaField("debug"))
		errs = errs.Also(validateDebug(ts.Debug).ViaField("debug"))
	}
	if ts.StepSpecs != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "stepSpecs", config.AlphaAPIFields).ViaField("stepSpecs"))
		errs = errs.Also(validateStepSpecs(ts.StepSpecs).ViaField("stepSpecs"))
	}
	if ts.SidecarSpecs != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "sidecarSpecs", config.AlphaAPIFields).ViaField("sidecarSpecs"))
		errs = errs.Also(validateSidecarSpecs(ts.SidecarSpecs).ViaField("sidecarSpecs"))
	}
	if ts.ComputeResources != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "computeResources", config.AlphaAPIFields).ViaField("computeResources"))
		errs = errs.Also(validateTaskRunComputeResources(ts.ComputeResources, ts.StepSpecs))
	}

	if ts.Status != "" {
		if ts.Status != TaskRunSpecStatusCancelled {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be %s", ts.Status, TaskRunSpecStatusCancelled), "status"))
		}
	}
	if ts.Status == "" {
		if ts.StatusMessage != "" {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("statusMessage should not be set if status is not set, but it is currently set to %s", ts.StatusMessage), "statusMessage"))
		}
	}

	if ts.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ts.Timeout.Duration < 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", ts.Timeout.Duration.String()), "timeout"))
		}
	}

	if ts.PodTemplate != nil {
		errs = errs.Also(validatePodTemplateEnv(ctx, *ts.PodTemplate))
	}
	return errs
}

// validateInlineParameters validates that any parameters called in the
// Task spec are declared in the TaskRun.
// This is crucial for propagated parameters because the parameters could
// be defined under taskRun and then called directly in the task steps.
// In this case, parameters cannot be validated by the underlying taskSpec
// since they may not have the parameters declared because of propagation.
func (ts *TaskRunSpec) validateInlineParameters(ctx context.Context) (errs *apis.FieldError) {
	if ts.TaskSpec == nil {
		return errs
	}
	paramSpecForValidation := make(map[string]ParamSpec)
	for _, p := range ts.Params {
		paramSpecForValidation = createParamSpecFromParam(p, paramSpecForValidation)
	}

	for _, p := range ts.TaskSpec.Params {
		var err *apis.FieldError
		paramSpecForValidation, err = combineParamSpec(p, paramSpecForValidation)
		if err != nil {
			errs = errs.Also(err)
		}
	}
	var paramSpec []ParamSpec
	for _, v := range paramSpecForValidation {
		paramSpec = append(paramSpec, v)
	}
	if ts.TaskSpec != nil && ts.TaskSpec.Steps != nil {
		errs = errs.Also(ValidateParameterTypes(ctx, paramSpec))
		errs = errs.Also(ValidateParameterVariables(ctx, ts.TaskSpec.Steps, paramSpec))
		errs = errs.Also(ValidateUsageOfDeclaredParameters(ctx, ts.TaskSpec.Steps, paramSpec))
	}
	return errs
}

func validatePodTemplateEnv(ctx context.Context, podTemplate pod.Template) (errs *apis.FieldError) {
	forbiddenEnvsConfigured := config.FromContextOrDefaults(ctx).Defaults.DefaultForbiddenEnv
	if len(forbiddenEnvsConfigured) == 0 {
		return errs
	}
	for _, pEnv := range podTemplate.Env {
		if slices.Contains(forbiddenEnvsConfigured, pEnv.Name) {
			errs = errs.Also(apis.ErrInvalidValue("PodTemplate cannot update a forbidden env: "+pEnv.Name, "PodTemplate.Env"))
		}
	}
	return errs
}

func createParamSpecFromParam(p Param, paramSpecForValidation map[string]ParamSpec) map[string]ParamSpec {
	value := p.Value
	pSpec := ParamSpec{
		Name:    p.Name,
		Default: &value,
		Type:    p.Value.Type,
	}
	if p.Value.ObjectVal != nil {
		pSpec.Properties = make(map[string]PropertySpec)
		prop := make(map[string]PropertySpec)
		for k := range p.Value.ObjectVal {
			prop[k] = PropertySpec{Type: ParamTypeString}
		}
		pSpec.Properties = prop
	}
	paramSpecForValidation[p.Name] = pSpec
	return paramSpecForValidation
}

func combineParamSpec(p ParamSpec, paramSpecForValidation map[string]ParamSpec) (map[string]ParamSpec, *apis.FieldError) {
	if pSpec, ok := paramSpecForValidation[p.Name]; ok {
		// Merge defaults with provided values in the taskrun.
		if p.Default != nil && p.Default.ObjectVal != nil {
			for k, v := range p.Default.ObjectVal {
				if pSpec.Default.ObjectVal == nil {
					pSpec.Default.ObjectVal = map[string]string{k: v}
				} else {
					pSpec.Default.ObjectVal[k] = v
				}
			}
			// If Default values of object type are provided then Properties must also be fully declared.
			if p.Properties == nil {
				return paramSpecForValidation, apis.ErrMissingField(fmt.Sprintf("%s.properties", p.Name))
			}
		}

		// Properties must be defined if paramSpec is of object Type
		if pSpec.Type == ParamTypeObject {
			if p.Properties == nil {
				return paramSpecForValidation, apis.ErrMissingField(fmt.Sprintf("%s.properties", p.Name))
			}
			// Expect Properties to be complete
			pSpec.Properties = p.Properties
		}
		paramSpecForValidation[p.Name] = pSpec
	} else {
		// No values provided by task run but found a paramSpec declaration.
		// Expect it to be fully speced out.
		paramSpecForValidation[p.Name] = p
	}
	return paramSpecForValidation, nil
}

// validateDebug
func validateDebug(db *TaskRunDebug) (errs *apis.FieldError) {
	breakpointOnFailure := "onFailure"
	validBreakpoints := sets.NewString()
	validBreakpoints.Insert(breakpointOnFailure)

	for _, b := range db.Breakpoint {
		if !validBreakpoints.Has(b) {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s is not a valid breakpoint. Available valid breakpoints include %s", b, validBreakpoints.List()), "breakpoint"))
		}
	}
	return errs
}

// ValidateWorkspaceBindings makes sure the volumes provided for the Task's declared workspaces make sense.
func ValidateWorkspaceBindings(ctx context.Context, wb []WorkspaceBinding) (errs *apis.FieldError) {
	var names []string
	for idx, w := range wb {
		names = append(names, w.Name)
		errs = errs.Also(w.Validate(ctx).ViaIndex(idx))
	}
	errs = errs.Also(validateNoDuplicateNames(names, true))
	return errs
}

// ValidateParameters makes sure the params for the Task are valid.
func ValidateParameters(ctx context.Context, params Params) (errs *apis.FieldError) {
	var names []string
	for _, p := range params {
		if p.Value.Type == ParamTypeObject {
			// Object type parameter is a beta feature and will fail validation if it's used in a taskrun spec
			// when the enable-api-fields feature gate is not "alpha" or "beta".
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "object type parameter", config.BetaAPIFields))
		}
		names = append(names, p.Name)
	}
	return errs.Also(validateNoDuplicateNames(names, false))
}

func validateStepSpecs(specs []TaskRunStepSpec) (errs *apis.FieldError) {
	var names []string
	for i, o := range specs {
		if o.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name").ViaIndex(i))
		} else {
			names = append(names, o.Name)
		}
	}
	errs = errs.Also(validateNoDuplicateNames(names, true))
	return errs
}

// validateTaskRunComputeResources ensures that compute resources are not configured at both the step level and the task level
func validateTaskRunComputeResources(computeResources *corev1.ResourceRequirements, specs []TaskRunStepSpec) (errs *apis.FieldError) {
	for _, spec := range specs {
		if spec.ComputeResources.Size() != 0 && computeResources != nil {
			return apis.ErrMultipleOneOf(
				"stepSpecs.resources",
				"computeResources",
			)
		}
	}
	return nil
}

func validateSidecarSpecs(specs []TaskRunSidecarSpec) (errs *apis.FieldError) {
	var names []string
	for i, o := range specs {
		if o.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name").ViaIndex(i))
		} else {
			names = append(names, o.Name)
		}
	}
	errs = errs.Also(validateNoDuplicateNames(names, true))
	return errs
}

// validateNoDuplicateNames returns an error for each name that is repeated in names.
// Case insensitive.
// If byIndex is true, the error will be reported by index instead of by key.
func validateNoDuplicateNames(names []string, byIndex bool) (errs *apis.FieldError) {
	seen := sets.NewString()
	for i, n := range names {
		if seen.Has(strings.ToLower(n)) {
			if byIndex {
				errs = errs.Also(apis.ErrMultipleOneOf("name").ViaIndex(i))
			} else {
				errs = errs.Also(apis.ErrMultipleOneOf("name").ViaKey(n))
			}
		}
		seen.Insert(strings.ToLower(n))
	}
	return errs
}
