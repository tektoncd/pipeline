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
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*PipelineRun)(nil)

// Validate pipelinerun
func (pr *PipelineRun) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(pr.GetObjectMeta()).ViaField("metadata")

	if apis.IsInDelete(ctx) {
		return nil
	}

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
		errs = errs.Also(ps.PipelineSpec.Validate(ctx).ViaField("pipelineSpec"))
	}

	if ps.Timeout != nil {
		// timeout should be a valid duration of at least 0.
		if ps.Timeout.Duration < 0 {
			errs = errs.Also(apis.ErrInvalidValue(fmt.Sprintf("%s should be >= 0", ps.Timeout.Duration.String()), "timeout"))
		}
	}

	// This is an alpha feature and will fail validation if it's used in a pipelinerun spec
	// when the enable-api-fields feature gate is anything but "alpha".
	if ps.Timeouts != nil {
		if ps.Timeout != nil {
			// can't have both at the same time
			errs = errs.Also(apis.ErrDisallowedFields("timeout", "timeouts"))
		}

		errs = errs.Also(ValidateEnabledAPIFields(ctx, "timeouts", config.AlphaAPIFields))

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

	errs = errs.Also(validateSpecStatus(ctx, ps.Status))

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

	return errs
}

func validateSpecStatus(ctx context.Context, status PipelineRunSpecStatus) *apis.FieldError {
	switch status {
	case "":
		return nil
	case PipelineRunSpecStatusPending,
		PipelineRunSpecStatusCancelledDeprecated:
		return nil
	case PipelineRunSpecStatusCancelled,
		PipelineRunSpecStatusCancelledRunFinally,
		PipelineRunSpecStatusStoppedRunFinally:
		return ValidateEnabledAPIFields(ctx, "graceful termination", "alpha")
	}

	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableAPIFields == config.AlphaAPIFields {
		return apis.ErrInvalidValue(fmt.Sprintf("%s should be %s, %s, %s or %s", status,
			PipelineRunSpecStatusCancelled,
			PipelineRunSpecStatusCancelledRunFinally,
			PipelineRunSpecStatusStoppedRunFinally,
			PipelineRunSpecStatusPending), "status")
	}
	return apis.ErrInvalidValue(fmt.Sprintf("%s should be %s or %s", status,
		PipelineRunSpecStatusCancelledDeprecated,
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
		if ps.Timeouts.Tasks.Duration > timeout {
			tasksTimeoutErr = true
		}
		if ps.Timeouts.Tasks.Duration == apisconfig.NoTimeoutDuration && timeout != apisconfig.NoTimeoutDuration {
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
		if ps.Timeouts.Finally.Duration > timeout {
			finallyTimeoutErr = true
		}
		if ps.Timeouts.Finally.Duration == apisconfig.NoTimeoutDuration && timeout != apisconfig.NoTimeoutDuration {
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
