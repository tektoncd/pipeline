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

package v1beta1

import (
	"context"
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*TaskRun)(nil)

// Validate taskrun
func (tr *TaskRun) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(tr.GetObjectMeta()).ViaField("metadata")
	if apis.IsInDelete(ctx) {
		return nil
	}
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

	errs = errs.Also(validateParameters(ctx, ts.Params).ViaField("params"))
	errs = errs.Also(validateWorkspaceBindings(ctx, ts.Workspaces).ViaField("workspaces"))
	errs = errs.Also(ts.Resources.Validate(ctx).ViaField("resources"))
	if ts.Debug != nil {
		errs = errs.Also(ValidateEnabledAPIFields(ctx, "debug", config.AlphaAPIFields).ViaField("debug"))
		errs = errs.Also(validateDebug(ts.Debug).ViaField("debug"))
	}
	if ts.StepOverrides != nil {
		errs = errs.Also(ValidateEnabledAPIFields(ctx, "stepOverrides", config.AlphaAPIFields).ViaField("stepOverrides"))
		errs = errs.Also(validateStepOverrides(ts.StepOverrides).ViaField("stepOverrides"))
	}
	if ts.SidecarOverrides != nil {
		errs = errs.Also(ValidateEnabledAPIFields(ctx, "sidecarOverrides", config.AlphaAPIFields).ViaField("sidecarOverrides"))
		errs = errs.Also(validateSidecarOverrides(ts.SidecarOverrides).ViaField("sidecarOverrides"))
	}

	if ts.Status != "" {
		if ts.Status != TaskRunSpecStatusCancelled {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be %s", ts.Status, TaskRunSpecStatusCancelled), "status"))
		}
	}
	if ts.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ts.Timeout.Duration < 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", ts.Timeout.Duration.String()), "timeout"))
		}
	}

	return errs
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

// validateWorkspaceBindings makes sure the volumes provided for the Task's declared workspaces make sense.
func validateWorkspaceBindings(ctx context.Context, wb []WorkspaceBinding) (errs *apis.FieldError) {
	var names []string
	for idx, w := range wb {
		names = append(names, w.Name)
		errs = errs.Also(w.Validate(ctx).ViaIndex(idx))
	}
	errs = errs.Also(validateNoDuplicateNames(names, true))
	return errs
}

func validateParameters(ctx context.Context, params []Param) (errs *apis.FieldError) {
	var names []string
	for _, p := range params {
		if p.Value.Type == ParamTypeObject {
			// Object type parameter is an alpha feature and will fail validation if it's used in a taskrun spec
			// when the enable-api-fields feature gate is not "alpha".
			errs = errs.Also(ValidateEnabledAPIFields(ctx, "object type parameter", config.AlphaAPIFields))
		}
		names = append(names, p.Name)
	}
	return errs.Also(validateNoDuplicateNames(names, false))
}

func validateStepOverrides(overrides []TaskRunStepOverride) (errs *apis.FieldError) {
	var names []string
	for i, o := range overrides {
		if o.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name").ViaIndex(i))
		} else {
			names = append(names, o.Name)
		}
	}
	errs = errs.Also(validateNoDuplicateNames(names, true))
	return errs
}

func validateSidecarOverrides(overrides []TaskRunSidecarOverride) (errs *apis.FieldError) {
	var names []string
	for i, o := range overrides {
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
