/*
Copyright 2023 The Tekton Authors

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
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Validate ensures that a supplied Ref field is populated
// correctly. No errors are returned for a nil Ref.
func (ref *Ref) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return
	}

	switch {
	case ref.Resolver != "" || ref.Params != nil:
		if ref.Resolver != "" {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver", config.BetaAPIFields).ViaField("resolver"))
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "resolver"))
			}
		}
		if ref.Params != nil {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver params", config.BetaAPIFields).ViaField("params"))
			if ref.Name != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "params"))
			}
			if ref.Resolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			errs = errs.Also(ValidateParameters(ctx, ref.Params))
		}
	case ref.Name != "":
		// ref name must be a valid k8s name
		if errSlice := validation.IsQualifiedName(ref.Name); len(errSlice) != 0 {
			errs = errs.Also(apis.ErrInvalidValue(strings.Join(errSlice, ","), "name"))
		}
	default:
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	return errs
}

func validateSidecars(sidecars []Sidecar) (errs *apis.FieldError) {
	for _, sc := range sidecars {
		errs = errs.Also(validateSidecarName(sc))

		if sc.Image == "" {
			errs = errs.Also(apis.ErrMissingField("image"))
		}

		if sc.Script != "" {
			if len(sc.Command) > 0 {
				errs = errs.Also(&apis.FieldError{
					Message: "script cannot be used with command",
					Paths:   []string{"script"},
				})
			}
		}
	}
	return errs
}

func validateStep(ctx context.Context, s Step, names sets.String) (errs *apis.FieldError) {
	if s.Ref != nil {
		if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableStepActions && isCreateOrUpdateAndDiverged(ctx, s) {
			return apis.ErrGeneric("feature flag %s should be set to true to reference StepActions in Steps.", config.EnableStepActions)
		}
		errs = errs.Also(s.Ref.Validate(ctx))
		if s.Image != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "image cannot be used with Ref",
				Paths:   []string{"image"},
			})
		}
		if len(s.Command) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "command cannot be used with Ref",
				Paths:   []string{"command"},
			})
		}
		if len(s.Args) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "args cannot be used with Ref",
				Paths:   []string{"args"},
			})
		}
		if s.Script != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "script cannot be used with Ref",
				Paths:   []string{"script"},
			})
		}
		if s.Env != nil {
			errs = errs.Also(&apis.FieldError{
				Message: "env cannot be used with Ref",
				Paths:   []string{"env"},
			})
		}
		if len(s.VolumeMounts) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "volumeMounts cannot be used with Ref",
				Paths:   []string{"volumeMounts"},
			})
		}
		if len(s.Results) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "results cannot be used with Ref",
				Paths:   []string{"results"},
			})
		}
	} else {
		if len(s.Params) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "params cannot be used without Ref",
				Paths:   []string{"params"},
			})
		}
		if len(s.Results) > 0 {
			if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableStepActions && isCreateOrUpdateAndDiverged(ctx, s) {
				return apis.ErrGeneric("feature flag %s should be set to true in order to use Results in Steps.", config.EnableStepActions)
			}
		}
		if s.Image == "" {
			errs = errs.Also(apis.ErrMissingField("Image"))
		}

		if s.Script != "" {
			if len(s.Command) > 0 {
				errs = errs.Also(&apis.FieldError{
					Message: "script cannot be used with command",
					Paths:   []string{"script"},
				})
			}
		}
	}

	if s.Name != "" {
		if names.Has(s.Name) {
			errs = errs.Also(apis.ErrInvalidValue(s.Name, "name"))
		}
		if e := validation.IsDNS1123Label(s.Name); len(e) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid value %q", s.Name),
				Paths:   []string{"name"},
				Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
			})
		}
		names.Insert(s.Name)
	}

	if s.Timeout != nil {
		if s.Timeout.Duration < time.Duration(0) {
			return apis.ErrInvalidValue(s.Timeout.Duration, "negative timeout")
		}
	}

	for j, vm := range s.VolumeMounts {
		if strings.HasPrefix(vm.MountPath, "/tekton/") &&
			!strings.HasPrefix(vm.MountPath, "/tekton/home") {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("volumeMount cannot be mounted under /tekton/ (volumeMount %q mounted at %q)", vm.Name, vm.MountPath), "mountPath").ViaFieldIndex("volumeMounts", j))
		}
		if strings.HasPrefix(vm.Name, "tekton-internal-") {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf(`volumeMount name %q cannot start with "tekton-internal-"`, vm.Name), "name").ViaFieldIndex("volumeMounts", j))
		}
	}

	if s.OnError != "" {
		if !isParamRefs(string(s.OnError)) && s.OnError != Continue && s.OnError != StopAndFail {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid value: \"%v\"", s.OnError),
				Paths:   []string{"onError"},
				Details: "Task step onError must be either \"continue\" or \"stopAndFail\"",
			})
		}
	}

	if s.Script != "" {
		cleaned := strings.TrimSpace(s.Script)
		if strings.HasPrefix(cleaned, "#!win") {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "windows script support", config.AlphaAPIFields).ViaField("script"))
		}
	}

	// StdoutConfig is an alpha feature and will fail validation if it's used in a task spec
	// when the enable-api-fields feature gate is not "alpha".
	if s.StdoutConfig != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "step stdout stream support", config.AlphaAPIFields).ViaField("stdoutconfig"))
	}
	// StderrConfig is an alpha feature and will fail validation if it's used in a task spec
	// when the enable-api-fields feature gate is not "alpha".
	if s.StderrConfig != nil {
		errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "step stderr stream support", config.AlphaAPIFields).ViaField("stderrconfig"))
	}
	return errs
}