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
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/internal/artifactref"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

// Validate ensures that a supplied Ref field is populated
// correctly. No errors are returned for a nil Ref.
func (ref *Ref) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ref == nil {
		return errs
	}
	return validateRef(ctx, ref.Name, ref.Resolver, ref.Params)
}

func validateRef(ctx context.Context, refName string, refResolver ResolverName, refParams Params) (errs *apis.FieldError) {
	switch {
	case refResolver != "" || refParams != nil:
		if refParams != nil {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver params", config.BetaAPIFields).ViaField("params"))
			if refName != "" {
				errs = errs.Also(apis.ErrMultipleOneOf("name", "params"))
			}
			if refResolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			errs = errs.Also(ValidateParameters(ctx, refParams))
		}
		if refResolver != "" {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "resolver", config.BetaAPIFields).ViaField("resolver"))
			if refName != "" {
				// make sure that the name is url-like.
				err := RefNameLikeUrl(refName)
				if err == nil && !config.FromContextOrDefaults(ctx).FeatureFlags.EnableConciseResolverSyntax {
					// If name is url-like then concise resolver syntax must be enabled
					errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use concise resolver syntax", config.EnableConciseResolverSyntax), ""))
				}
				if err != nil {
					errs = errs.Also(apis.ErrInvalidValue(err, "name"))
				}
			}
		}
	case refName != "":
		// ref name can be a Url-like format.
		if err := RefNameLikeUrl(refName); err == nil {
			// If name is url-like then concise resolver syntax must be enabled
			if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableConciseResolverSyntax {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use concise resolver syntax", config.EnableConciseResolverSyntax), ""))
			}
			// In stage1 of concise remote resolvers syntax, this is a required field.
			// TODO: remove this check when implementing stage 2 where this is optional.
			if refResolver == "" {
				errs = errs.Also(apis.ErrMissingField("resolver"))
			}
			// Or, it must be a valid k8s name
		} else {
			// ref name must be a valid k8s name
			if errSlice := validation.IsQualifiedName(refName); len(errSlice) != 0 {
				errs = errs.Also(apis.ErrInvalidValue(strings.Join(errSlice, ","), "name"))
			}
		}
	default:
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	return errs
}

// RefNameLikeUrl checks if the name is url parsable and returns an error if it isn't.
func RefNameLikeUrl(name string) error {
	schemeRegex := regexp.MustCompile(`[\w-]+:\/\/*`)
	if !schemeRegex.MatchString(name) {
		return errors.New("invalid URI for request")
	}
	return nil
}

// Validate implements apis.Validatable
func (s *Step) Validate(ctx context.Context) (errs *apis.FieldError) {
	if err := validateArtifactsReferencesInStep(ctx, s); err != nil {
		return err
	}

	if s.Ref != nil {
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
		if s.WorkingDir != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "working dir cannot be used with Ref",
				Paths:   []string{"workingDir"},
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
		if e := validation.IsDNS1123Label(s.Name); len(e) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid value %q", s.Name),
				Paths:   []string{"name"},
				Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
			})
		}
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
		if !isParamRefs(string(s.OnError)) && s.OnError != Continue && s.OnError != StopAndFail && s.OnError != ContinueAndFail {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid value: \"%v\"", s.OnError),
				Paths:   []string{"onError"},
				Details: "Task step onError must be \"continue\", \"stopAndFail\" or \"continueAndFail\"",
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

	// Validate usage of step result reference.
	// Referencing previous step's results are only allowed in `env`, `command` and `args`.
	errs = errs.Also(validateStepResultReference(s))

	// Validate usage of step artifacts output reference
	// Referencing previous step's results are only allowed in `env`, `command` and `args`, `script`.
	errs = errs.Also(validateStepArtifactsReference(s))
	return errs
}

// isParamRefs attempts to check if a specified string looks like it contains any parameter reference
// This is useful to make sure the specified value looks like a Parameter Reference before performing any strict validation
func isParamRefs(s string) bool {
	return strings.HasPrefix(s, "$("+ParamsPrefix)
}

func validateArtifactsReferencesInStep(ctx context.Context, s *Step) *apis.FieldError {
	cfg := config.FromContextOrDefaults(ctx)
	if cfg == nil || cfg.FeatureFlags == nil {
		cfg = &config.Config{
			FeatureFlags: &config.FeatureFlags{},
		}
	}

	if !cfg.FeatureFlags.EnableArtifacts {
		var t []string
		if s.Script != "" {
			t = append(t, s.Script)
		}
		if len(s.Command) > 0 {
			t = append(t, s.Command...)
		}
		if len(s.Args) > 0 {
			t = append(t, s.Args...)
		}
		if s.Env != nil {
			for _, e := range s.Env {
				if e.Value != "" {
					t = append(t, e.Value)
				}
			}
		}
		if slices.ContainsFunc(t, stepArtifactReferenceExists) || slices.ContainsFunc(t, taskArtifactReferenceExists) {
			return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts), "")
		}
	}
	return nil
}

func stepArtifactReferenceExists(src string) bool {
	return len(artifactref.StepArtifactRegex.FindAllStringSubmatch(src, -1)) > 0 || strings.Contains(src, "$("+artifactref.StepArtifactPathPattern+")")
}

func taskArtifactReferenceExists(src string) bool {
	return len(artifactref.TaskArtifactRegex.FindAllStringSubmatch(src, -1)) > 0 || strings.Contains(src, "$("+artifactref.TaskArtifactPathPattern+")")
}

func validateStepResultReference(s *Step) (errs *apis.FieldError) {
	errs = errs.Also(errorIfStepResultReferencedInField(s.Name, "name"))
	errs = errs.Also(errorIfStepResultReferencedInField(s.Image, "image"))
	errs = errs.Also(errorIfStepResultReferencedInField(s.Script, "script"))
	errs = errs.Also(errorIfStepResultReferencedInField(string(s.ImagePullPolicy), "imagePullPolicy"))
	errs = errs.Also(errorIfStepResultReferencedInField(s.WorkingDir, "workingDir"))
	for _, e := range s.EnvFrom {
		errs = errs.Also(errorIfStepResultReferencedInField(e.Prefix, "envFrom.prefix"))
		if e.ConfigMapRef != nil {
			errs = errs.Also(errorIfStepResultReferencedInField(e.ConfigMapRef.LocalObjectReference.Name, "envFrom.configMapRef"))
		}
		if e.SecretRef != nil {
			errs = errs.Also(errorIfStepResultReferencedInField(e.SecretRef.LocalObjectReference.Name, "envFrom.secretRef"))
		}
	}
	for _, v := range s.VolumeMounts {
		errs = errs.Also(errorIfStepResultReferencedInField(v.Name, "volumeMounts.name"))
		errs = errs.Also(errorIfStepResultReferencedInField(v.MountPath, "volumeMounts.mountPath"))
		errs = errs.Also(errorIfStepResultReferencedInField(v.SubPath, "volumeMounts.subPath"))
	}
	for _, v := range s.VolumeDevices {
		errs = errs.Also(errorIfStepResultReferencedInField(v.Name, "volumeDevices.name"))
		errs = errs.Also(errorIfStepResultReferencedInField(v.DevicePath, "volumeDevices.devicePath"))
	}
	return errs
}

func errorIfStepResultReferencedInField(value, fieldName string) (errs *apis.FieldError) {
	matches := resultref.StepResultRegex.FindAllStringSubmatch(value, -1)
	if len(matches) > 0 {
		errs = errs.Also(&apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{fieldName},
		})
	}
	return errs
}

func validateStepArtifactsReference(s *Step) (errs *apis.FieldError) {
	errs = errs.Also(errorIfStepArtifactReferencedInField(s.Name, "name"))
	errs = errs.Also(errorIfStepArtifactReferencedInField(s.Image, "image"))
	errs = errs.Also(errorIfStepArtifactReferencedInField(string(s.ImagePullPolicy), "imagePullPolicy"))
	errs = errs.Also(errorIfStepArtifactReferencedInField(s.WorkingDir, "workingDir"))
	for _, e := range s.EnvFrom {
		errs = errs.Also(errorIfStepArtifactReferencedInField(e.Prefix, "envFrom.prefix"))
		if e.ConfigMapRef != nil {
			errs = errs.Also(errorIfStepArtifactReferencedInField(e.ConfigMapRef.LocalObjectReference.Name, "envFrom.configMapRef"))
		}
		if e.SecretRef != nil {
			errs = errs.Also(errorIfStepArtifactReferencedInField(e.SecretRef.LocalObjectReference.Name, "envFrom.secretRef"))
		}
	}
	for _, v := range s.VolumeMounts {
		errs = errs.Also(errorIfStepArtifactReferencedInField(v.Name, "volumeMounts.name"))
		errs = errs.Also(errorIfStepArtifactReferencedInField(v.MountPath, "volumeMounts.mountPath"))
		errs = errs.Also(errorIfStepArtifactReferencedInField(v.SubPath, "volumeMounts.subPath"))
	}
	for _, v := range s.VolumeDevices {
		errs = errs.Also(errorIfStepArtifactReferencedInField(v.Name, "volumeDevices.name"))
		errs = errs.Also(errorIfStepArtifactReferencedInField(v.DevicePath, "volumeDevices.devicePath"))
	}
	return errs
}

func errorIfStepArtifactReferencedInField(value, fieldName string) (errs *apis.FieldError) {
	if stepArtifactReferenceExists(value) {
		errs = errs.Also(&apis.FieldError{
			Message: "stepArtifact substitutions are only allowed in env, command, args and script. Found usage in",
			Paths:   []string{fieldName},
		})
	}
	return errs
}

func (sc *Sidecar) Validate(ctx context.Context) (errs *apis.FieldError) {
	if sc.Name == pipeline.ReservedResultsSidecarName {
		errs = errs.Also(&apis.FieldError{
			Message: fmt.Sprintf("Invalid: cannot use reserved sidecar name %v ", sc.Name),
			Paths:   []string{"name"},
		})
	}

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
	return errs
}
