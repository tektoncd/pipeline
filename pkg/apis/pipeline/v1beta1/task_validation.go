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
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/internal/artifactref"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
	"github.com/tektoncd/pipeline/pkg/substitution"

	"golang.org/x/exp/slices"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

const (
	// stringAndArrayVariableNameFormat is the regex to validate if string/array variable name format follows the following rules.
	// - Must only contain alphanumeric characters, hyphens (-), underscores (_), and dots (.)
	// - Must begin with a letter or an underscore (_)
	stringAndArrayVariableNameFormat = "^[_a-zA-Z][_a-zA-Z0-9.-]*$"

	// objectVariableNameFormat is the regext used to validate object name and key names format
	// The difference with the array or string name format is that object variable names shouldn't contain dots.
	objectVariableNameFormat = "^[_a-zA-Z][_a-zA-Z0-9-]*$"
)

var (
	_ apis.Validatable              = (*Task)(nil)
	_ resourcesemantics.VerbLimited = (*Task)(nil)
)

// SupportedVerbs returns the operations that validation should be called for
func (t *Task) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

var (
	stringAndArrayVariableNameFormatRegex = regexp.MustCompile(stringAndArrayVariableNameFormat)
	objectVariableNameFormatRegex         = regexp.MustCompile(objectVariableNameFormat)
)

// Validate implements apis.Validatable
func (t *Task) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(t.GetObjectMeta()).ViaField("metadata")
	errs = errs.Also(t.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
	// When a Task is created directly, instead of declared inline in a TaskRun or PipelineRun,
	// we do not support propagated parameters. Validate that all params it uses are declared.
	return errs.Also(ValidateUsageOfDeclaredParameters(ctx, t.Spec.Steps, t.Spec.Params).ViaField("spec"))
}

// Validate implements apis.Validatable
func (ts *TaskSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if len(ts.Steps) == 0 {
		errs = errs.Also(apis.ErrMissingField("steps"))
	}

	errs = errs.Also(ValidateVolumes(ts.Volumes).ViaField("volumes"))
	errs = errs.Also(validateDeclaredWorkspaces(ts.Workspaces, ts.Steps, ts.StepTemplate).ViaField("workspaces"))
	errs = errs.Also(validateWorkspaceUsages(ctx, ts))
	mergedSteps, err := MergeStepsWithStepTemplate(ts.StepTemplate, ts.Steps)
	if err != nil {
		errs = errs.Also(&apis.FieldError{
			Message: fmt.Sprintf("error merging step template and steps: %s", err),
			Paths:   []string{"stepTemplate"},
			Details: err.Error(),
		})
	}

	errs = errs.Also(validateSteps(ctx, mergedSteps).ViaField("steps"))
	errs = errs.Also(validateSidecarNames(ts.Sidecars))
	errs = errs.Also(ValidateParameterTypes(ctx, ts.Params).ViaField("params"))
	errs = errs.Also(ValidateParameterVariables(ctx, ts.Steps, ts.Params))
	errs = errs.Also(validateTaskContextVariables(ctx, ts.Steps))
	errs = errs.Also(validateTaskResultsVariables(ctx, ts.Steps, ts.Results))
	errs = errs.Also(validateResults(ctx, ts.Results).ViaField("results"))
	if ts.Resources != nil {
		errs = errs.Also(apis.ErrDisallowedFields("resources"))
	}
	return errs
}

// ValidateUsageOfDeclaredParameters validates that all parameters referenced in the Task are declared by the Task.
func ValidateUsageOfDeclaredParameters(ctx context.Context, steps []Step, params ParamSpecs) *apis.FieldError {
	var errs *apis.FieldError
	_, _, objectParams := params.sortByType()
	allParameterNames := sets.NewString(params.getNames()...)
	errs = errs.Also(validateVariables(ctx, steps, "params", allParameterNames))
	errs = errs.Also(validateObjectUsage(ctx, steps, objectParams))
	errs = errs.Also(validateObjectParamsHaveProperties(ctx, params))
	return errs
}

// validateObjectParamsHaveProperties returns an error if any declared object params are missing properties
func validateObjectParamsHaveProperties(ctx context.Context, params ParamSpecs) *apis.FieldError {
	var errs *apis.FieldError
	for _, p := range params {
		if p.Type == ParamTypeObject && p.Properties == nil {
			errs = errs.Also(apis.ErrMissingField(p.Name + ".properties"))
		}
	}
	return errs
}

func validateSidecarNames(sidecars []Sidecar) (errs *apis.FieldError) {
	for _, sc := range sidecars {
		if sc.Name == pipeline.ReservedResultsSidecarName {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("Invalid: cannot use reserved sidecar name %v ", sc.Name),
				Paths:   []string{"sidecars"},
			})
		}
	}
	return errs
}

func validateResults(ctx context.Context, results []TaskResult) (errs *apis.FieldError) {
	for index, result := range results {
		errs = errs.Also(result.Validate(ctx).ViaIndex(index))
	}
	return errs
}

// a mount path which conflicts with any other declared workspaces, with the explicitly
// declared volume mounts, or with the stepTemplate. The names must also be unique.
func validateDeclaredWorkspaces(workspaces []WorkspaceDeclaration, steps []Step, stepTemplate *StepTemplate) (errs *apis.FieldError) {
	mountPaths := sets.NewString()
	for _, step := range steps {
		for _, vm := range step.VolumeMounts {
			mountPaths.Insert(filepath.Clean(vm.MountPath))
		}
	}
	if stepTemplate != nil {
		for _, vm := range stepTemplate.VolumeMounts {
			mountPaths.Insert(filepath.Clean(vm.MountPath))
		}
	}

	wsNames := sets.NewString()
	for idx, w := range workspaces {
		// Workspace names must be unique
		if wsNames.Has(w.Name) {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("workspace name %q must be unique", w.Name), "name").ViaIndex(idx))
		} else {
			wsNames.Insert(w.Name)
		}
		// Workspaces must not try to use mount paths that are already used
		mountPath := filepath.Clean(w.GetMountPath())
		if _, ok := mountPaths[mountPath]; ok {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("workspace mount path %q must be unique", mountPath), "mountpath").ViaIndex(idx))
		}
		mountPaths[mountPath] = struct{}{}
	}
	return errs
}

// validateWorkspaceUsages checks that all WorkspaceUsage objects in Steps
// refer to workspaces that are defined in the Task.
//
// This is a beta feature and will fail validation if it's used by a step
// or sidecar when the enable-api-fields feature gate is anything but "beta".
//
// Note that this feature reached beta after the v1 API version has been released and
// consequently it is *not* implicitly enabled on the v1beta1 API to avoid suffering
// from the issues described in TEP-0138 https://github.com/tektoncd/community/pull/1034
func validateWorkspaceUsages(ctx context.Context, ts *TaskSpec) (errs *apis.FieldError) {
	workspaces := ts.Workspaces
	steps := ts.Steps
	sidecars := ts.Sidecars

	wsNames := sets.NewString()
	for _, w := range workspaces {
		wsNames.Insert(w.Name)
	}

	for stepIdx, step := range steps {
		if len(step.Workspaces) != 0 {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "step workspaces", config.BetaAPIFields).ViaIndex(stepIdx).ViaField("steps"))
		}
		for workspaceIdx, w := range step.Workspaces {
			if !wsNames.Has(w.Name) {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("undefined workspace %q", w.Name), "name").ViaIndex(workspaceIdx).ViaField("workspaces").ViaIndex(stepIdx).ViaField("steps"))
			}
		}
	}

	for sidecarIdx, sidecar := range sidecars {
		if len(sidecar.Workspaces) != 0 {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "sidecar workspaces", config.BetaAPIFields).ViaIndex(sidecarIdx).ViaField("sidecars"))
		}
		for workspaceIdx, w := range sidecar.Workspaces {
			if !wsNames.Has(w.Name) {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("undefined workspace %q", w.Name), "name").ViaIndex(workspaceIdx).ViaField("workspaces").ViaIndex(sidecarIdx).ViaField("sidecars"))
			}
		}
	}

	return errs
}

// ValidateVolumes validates a slice of volumes to make sure there are no dupilcate names
func ValidateVolumes(volumes []corev1.Volume) (errs *apis.FieldError) {
	// Task must not have duplicate volume names.
	vols := sets.NewString()
	for idx, v := range volumes {
		if vols.Has(v.Name) {
			errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("multiple volumes with same name %q", v.Name), "name").ViaIndex(idx))
		} else {
			vols.Insert(v.Name)
		}
	}
	return errs
}

func validateSteps(ctx context.Context, steps []Step) (errs *apis.FieldError) {
	// Task must not have duplicate step names.
	names := sets.NewString()
	for idx, s := range steps {
		errs = errs.Also(validateStep(ctx, s, names).ViaIndex(idx))
		if s.Results != nil {
			errs = errs.Also(v1.ValidateStepResultsVariables(ctx, s.Results, s.Script).ViaIndex(idx))
			errs = errs.Also(v1.ValidateStepResults(ctx, s.Results).ViaIndex(idx).ViaField("results"))
		}
		if len(s.When) > 0 {
			errs = errs.Also(s.When.validate(ctx).ViaIndex(idx))
		}
	}
	return errs
}

// isCreateOrUpdateAndDiverged checks if the webhook event was create or update
// if create, it returns true.
// if update, it checks if the step results have diverged and returns if diverged.
// if neither, it returns false.
func isCreateOrUpdateAndDiverged(ctx context.Context, s Step) bool {
	if apis.IsInCreate(ctx) {
		return true
	}
	if apis.IsInUpdate(ctx) {
		baseline := apis.GetBaseline(ctx)
		var baselineStep Step
		switch o := baseline.(type) {
		case *TaskRun:
			if o.Spec.TaskSpec != nil {
				for _, step := range o.Spec.TaskSpec.Steps {
					if s.Name == step.Name {
						baselineStep = step
						break
					}
				}
			}
		default:
			// the baseline is not a taskrun.
			// return true so that the validation can happen
			return true
		}
		// If an update event, check if the results have diverged from the baseline
		// this way, the feature flag check wont happen.
		// This will avoid issues like https://github.com/tektoncd/pipeline/issues/5203
		// when the feature is turned off mid-run.
		diverged := !reflect.DeepEqual(s.Results, baselineStep.Results)
		return diverged
	}
	return false
}

func errorIfStepResultReferenceinField(value, fieldName string) (errs *apis.FieldError) {
	matches := resultref.StepResultRegex.FindAllStringSubmatch(value, -1)
	if len(matches) > 0 {
		errs = errs.Also(&apis.FieldError{
			Message: "stepResult substitutions are only allowed in env, command and args. Found usage in",
			Paths:   []string{fieldName},
		})
	}
	return errs
}

func stepArtifactReferenceExists(src string) bool {
	return len(artifactref.StepArtifactRegex.FindAllStringSubmatch(src, -1)) > 0 || strings.Contains(src, "$("+artifactref.StepArtifactPathPattern+")")
}

func taskArtifactReferenceExists(src string) bool {
	return len(artifactref.TaskArtifactRegex.FindAllStringSubmatch(src, -1)) > 0 || strings.Contains(src, "$("+artifactref.TaskArtifactPathPattern+")")
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

func validateStepArtifactsReference(s Step) (errs *apis.FieldError) {
	errs = errs.Also(errorIfStepArtifactReferencedInField(s.Name, "name"))
	errs = errs.Also(errorIfStepArtifactReferencedInField(s.Image, "image"))
	errs = errs.Also(errorIfStepArtifactReferencedInField(string(s.ImagePullPolicy), "imagePullPoliicy"))
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

func validateStepResultReference(s Step) (errs *apis.FieldError) {
	errs = errs.Also(errorIfStepResultReferenceinField(s.Name, "name"))
	errs = errs.Also(errorIfStepResultReferenceinField(s.Image, "image"))
	errs = errs.Also(errorIfStepResultReferenceinField(s.Script, "script"))
	errs = errs.Also(errorIfStepResultReferenceinField(string(s.ImagePullPolicy), "imagePullPoliicy"))
	errs = errs.Also(errorIfStepResultReferenceinField(s.WorkingDir, "workingDir"))
	for _, e := range s.EnvFrom {
		errs = errs.Also(errorIfStepResultReferenceinField(e.Prefix, "envFrom.prefix"))
		if e.ConfigMapRef != nil {
			errs = errs.Also(errorIfStepResultReferenceinField(e.ConfigMapRef.LocalObjectReference.Name, "envFrom.configMapRef"))
		}
		if e.SecretRef != nil {
			errs = errs.Also(errorIfStepResultReferenceinField(e.SecretRef.LocalObjectReference.Name, "envFrom.secretRef"))
		}
	}
	for _, v := range s.VolumeMounts {
		errs = errs.Also(errorIfStepResultReferenceinField(v.Name, "volumeMounts.name"))
		errs = errs.Also(errorIfStepResultReferenceinField(v.MountPath, "volumeMounts.mountPath"))
		errs = errs.Also(errorIfStepResultReferenceinField(v.SubPath, "volumeMounts.subPath"))
	}
	for _, v := range s.VolumeDevices {
		errs = errs.Also(errorIfStepResultReferenceinField(v.Name, "volumeDevices.name"))
		errs = errs.Also(errorIfStepResultReferenceinField(v.DevicePath, "volumeDevices.devicePath"))
	}
	return errs
}

func validateStep(ctx context.Context, s Step, names sets.String) (errs *apis.FieldError) {
	if err := validateArtifactsReferencesInStep(ctx, s); err != nil {
		return err
	}

	if s.Ref != nil {
		if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableStepActions && isCreateOrUpdateAndDiverged(ctx, s) {
			return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to reference StepActions in Steps.", config.EnableStepActions), "")
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
		if len(s.Results) > 0 {
			if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableStepActions && isCreateOrUpdateAndDiverged(ctx, s) {
				return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true in order to use Results in Steps.", config.EnableStepActions), "")
			}
		}
		if len(s.When) > 0 {
			if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableStepActions && isCreateOrUpdateAndDiverged(ctx, s) {
				return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true in order to use When in Steps.", config.EnableStepActions), "")
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

	// Validate usage of step result reference.
	// Referencing previous step's results are only allowed in `env`, `command` and `args`.
	errs = errs.Also(validateStepResultReference(s))

	// Validate usage of step artifacts output reference
	// Referencing previous step's results are only allowed in `env`, `command` and `args`, `script`.
	errs = errs.Also(validateStepArtifactsReference(s))

	return errs
}

func validateArtifactsReferencesInStep(ctx context.Context, s Step) *apis.FieldError {
	if !config.FromContextOrDefaults(ctx).FeatureFlags.EnableArtifacts {
		var t []string
		t = append(t, s.Script)
		t = append(t, s.Command...)
		t = append(t, s.Args...)
		for _, e := range s.Env {
			t = append(t, e.Value)
		}
		if slices.ContainsFunc(t, stepArtifactReferenceExists) || slices.ContainsFunc(t, taskArtifactReferenceExists) {
			return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature.", config.EnableArtifacts), "")
		}
	}
	return nil
}

// ValidateParameterTypes validates all the types within a slice of ParamSpecs
func ValidateParameterTypes(ctx context.Context, params []ParamSpec) (errs *apis.FieldError) {
	for _, p := range params {
		errs = errs.Also(p.ValidateType(ctx))
	}
	return errs
}

// ValidateType checks that the type of a ParamSpec is allowed and its default value matches that type
func (p ParamSpec) ValidateType(ctx context.Context) *apis.FieldError {
	// Ensure param has a valid type.
	validType := false
	for _, allowedType := range AllParamTypes {
		if p.Type == allowedType {
			validType = true
		}
	}
	if !validType {
		return apis.ErrInvalidValue(p.Type, p.Name+".type")
	}

	// If a default value is provided, ensure its type matches param's declared type.
	if (p.Default != nil) && (p.Default.Type != p.Type) {
		return &apis.FieldError{
			Message: fmt.Sprintf(
				"\"%v\" type does not match default value's type: \"%v\"", p.Type, p.Default.Type),
			Paths: []string{
				p.Name + ".type",
				p.Name + ".default.type",
			},
		}
	}

	// Check object type and its PropertySpec type
	return p.ValidateObjectType(ctx)
}

// ValidateObjectType checks that object type parameter does not miss the
// definition of `properties` section and the type of a PropertySpec is allowed.
// (Currently, only string is allowed)
func (p ParamSpec) ValidateObjectType(ctx context.Context) *apis.FieldError {
	invalidKeys := []string{}
	for key, propertySpec := range p.Properties {
		if propertySpec.Type != ParamTypeString {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) != 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("The value type specified for these keys %v is invalid", invalidKeys),
			Paths:   []string{p.Name + ".properties"},
		}
	}

	return nil
}

// ValidateParameterVariables validates all variables within a slice of ParamSpecs against a slice of Steps
func ValidateParameterVariables(ctx context.Context, steps []Step, params ParamSpecs) *apis.FieldError {
	var errs *apis.FieldError
	errs = errs.Also(params.validateNoDuplicateNames())
	errs = errs.Also(params.validateParamEnums(ctx).ViaField("params"))
	stringParams, arrayParams, objectParams := params.sortByType()
	stringParameterNames := sets.NewString(stringParams.getNames()...)
	arrayParameterNames := sets.NewString(arrayParams.getNames()...)
	errs = errs.Also(validateNameFormat(stringParameterNames.Insert(arrayParameterNames.List()...), objectParams))
	return errs.Also(validateArrayUsage(steps, "params", arrayParameterNames))
}

// validateTaskContextVariables returns an error if any Steps reference context variables that don't exist.
func validateTaskContextVariables(ctx context.Context, steps []Step) *apis.FieldError {
	taskRunContextNames := sets.NewString().Insert(
		"name",
		"namespace",
		"uid",
	)
	taskContextNames := sets.NewString().Insert(
		"name",
		"retry-count",
	)
	errs := validateVariables(ctx, steps, "context\\.taskRun", taskRunContextNames)
	return errs.Also(validateVariables(ctx, steps, "context\\.task", taskContextNames))
}

// validateTaskResultsVariables validates if the results referenced in step script are defined in task results
func validateTaskResultsVariables(ctx context.Context, steps []Step, results []TaskResult) (errs *apis.FieldError) {
	resultsNames := sets.NewString()
	for _, r := range results {
		resultsNames.Insert(r.Name)
	}
	for idx, step := range steps {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(step.Script, "results", resultsNames).ViaField("script").ViaFieldIndex("steps", idx))
	}
	return errs
}

// validateObjectUsage validates the usage of individual attributes of an object param and the usage of the entire object
func validateObjectUsage(ctx context.Context, steps []Step, params []ParamSpec) (errs *apis.FieldError) {
	objectParameterNames := sets.NewString()
	for _, p := range params {
		// collect all names of object type params
		objectParameterNames.Insert(p.Name)

		// collect all keys for this object param
		objectKeys := sets.NewString()
		for key := range p.Properties {
			objectKeys.Insert(key)
		}

		// check if the object's key names are referenced correctly i.e. param.objectParam.key1
		errs = errs.Also(validateVariables(ctx, steps, "params\\."+p.Name, objectKeys))
	}

	return errs.Also(validateObjectUsageAsWhole(steps, "params", objectParameterNames))
}

// validateObjectUsageAsWhole returns an error if the Steps contain references to the entire input object params in fields where these references are prohibited
func validateObjectUsageAsWhole(steps []Step, prefix string, vars sets.String) (errs *apis.FieldError) {
	for idx, step := range steps {
		errs = errs.Also(validateStepObjectUsageAsWhole(step, prefix, vars)).ViaFieldIndex("steps", idx)
	}
	return errs
}

// validateStepObjectUsageAsWhole returns an error if the Step contains references to the entire input object params in fields where these references are prohibited
func validateStepObjectUsageAsWhole(step Step, prefix string, vars sets.String) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToEntireProhibitedVariables(step.Name, prefix, vars).ViaField("name")
	errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(step.Image, prefix, vars).ViaField("image"))
	errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(step.WorkingDir, prefix, vars).ViaField("workingDir"))
	errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(step.Script, prefix, vars).ViaField("script"))
	for i, cmd := range step.Command {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range step.Args {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range step.Env {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, v := range step.VolumeMounts {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(v.Name, prefix, vars).ViaField("name").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(v.MountPath, prefix, vars).ViaField("mountPath").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(v.SubPath, prefix, vars).ViaField("subPath").ViaFieldIndex("volumeMount", i))
	}
	return errs
}

// validateArrayUsage returns an error if the Steps contain references to the input array params in fields where these references are prohibited
func validateArrayUsage(steps []Step, prefix string, arrayParamNames sets.String) (errs *apis.FieldError) {
	for idx, step := range steps {
		errs = errs.Also(validateStepArrayUsage(step, prefix, arrayParamNames)).ViaFieldIndex("steps", idx)
	}
	return errs
}

// validateStepArrayUsage returns an error if the Step contains references to the input array params in fields where these references are prohibited
func validateStepArrayUsage(step Step, prefix string, arrayParamNames sets.String) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToProhibitedVariables(step.Name, prefix, arrayParamNames).ViaField("name")
	errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(step.Image, prefix, arrayParamNames).ViaField("image"))
	errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(step.WorkingDir, prefix, arrayParamNames).ViaField("workingDir"))
	errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(step.Script, prefix, arrayParamNames).ViaField("script"))
	for i, cmd := range step.Command {
		errs = errs.Also(substitution.ValidateVariableReferenceIsIsolated(cmd, prefix, arrayParamNames).ViaFieldIndex("command", i))
	}
	for i, arg := range step.Args {
		errs = errs.Also(substitution.ValidateVariableReferenceIsIsolated(arg, prefix, arrayParamNames).ViaFieldIndex("args", i))
	}
	for _, env := range step.Env {
		errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(env.Value, prefix, arrayParamNames).ViaFieldKey("env", env.Name))
	}
	for i, v := range step.VolumeMounts {
		errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(v.Name, prefix, arrayParamNames).ViaField("name").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(v.MountPath, prefix, arrayParamNames).ViaField("mountPath").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(v.SubPath, prefix, arrayParamNames).ViaField("subPath").ViaFieldIndex("volumeMount", i))
	}
	return errs
}

// validateVariables returns an error if the Steps contain references to any unknown variables
func validateVariables(ctx context.Context, steps []Step, prefix string, vars sets.String) (errs *apis.FieldError) {
	// We've checked param name format. Now, we want to check if param names are referenced correctly in each step
	for idx, step := range steps {
		errs = errs.Also(validateStepVariables(ctx, step, prefix, vars).ViaFieldIndex("steps", idx))
	}
	return errs
}

// validateNameFormat validates that the name format of all param types follows the rules
func validateNameFormat(stringAndArrayParams sets.String, objectParams []ParamSpec) (errs *apis.FieldError) {
	// checking string or array name format
	// ----
	invalidStringAndArrayNames := []string{}
	// Converting to sorted list here rather than just looping map keys
	// because we want the order of items in vars to be deterministic for purpose of unit testing
	for _, name := range stringAndArrayParams.List() {
		if !stringAndArrayVariableNameFormatRegex.MatchString(name) {
			invalidStringAndArrayNames = append(invalidStringAndArrayNames, name)
		}
	}

	if len(invalidStringAndArrayNames) != 0 {
		errs = errs.Also(&apis.FieldError{
			Message: fmt.Sprintf("The format of following array and string variable names is invalid: %s", invalidStringAndArrayNames),
			Paths:   []string{"params"},
			Details: "String/Array Names: \nMust only contain alphanumeric characters, hyphens (-), underscores (_), and dots (.)\nMust begin with a letter or an underscore (_)",
		})
	}

	// checking object name and key name format
	// -----
	invalidObjectNames := map[string][]string{}
	for _, obj := range objectParams {
		// check object param name
		if !objectVariableNameFormatRegex.MatchString(obj.Name) {
			invalidObjectNames[obj.Name] = []string{}
		}

		// check key names
		for k := range obj.Properties {
			if !objectVariableNameFormatRegex.MatchString(k) {
				invalidObjectNames[obj.Name] = append(invalidObjectNames[obj.Name], k)
			}
		}
	}

	if len(invalidObjectNames) != 0 {
		errs = errs.Also(&apis.FieldError{
			Message: fmt.Sprintf("Object param name and key name format is invalid: %s", invalidObjectNames),
			Paths:   []string{"params"},
			Details: "Object Names: \nMust only contain alphanumeric characters, hyphens (-), underscores (_) \nMust begin with a letter or an underscore (_)",
		})
	}

	return errs
}

// validateStepVariables returns an error if the Step contains references to any unknown variables
func validateStepVariables(ctx context.Context, step Step, prefix string, vars sets.String) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToUnknownVariables(step.Name, prefix, vars).ViaField("name")
	errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(step.Image, prefix, vars).ViaField("image"))
	errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(step.WorkingDir, prefix, vars).ViaField("workingDir"))
	errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(step.Script, prefix, vars).ViaField("script"))
	for i, cmd := range step.Command {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range step.Args {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range step.Env {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, v := range step.VolumeMounts {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(v.Name, prefix, vars).ViaField("name").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(v.MountPath, prefix, vars).ViaField("MountPath").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(v.SubPath, prefix, vars).ViaField("SubPath").ViaFieldIndex("volumeMount", i))
	}
	errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(string(step.OnError), prefix, vars).ViaField("onError"))
	return errs
}

// isParamRefs attempts to check if a specified string looks like it contains any parameter reference
// This is useful to make sure the specified value looks like a Parameter Reference before performing any strict validation
func isParamRefs(s string) bool {
	return strings.HasPrefix(s, "$("+ParamsPrefix)
}

// GetIndexingReferencesToArrayParams returns all strings referencing indices of TaskRun array parameters
// from parameters, workspaces, and when expressions defined in the Task.
// For example, if a Task has a parameter with a value "$(params.array-param-name[1])",
// this would be one of the strings returned.
func (ts *TaskSpec) GetIndexingReferencesToArrayParams() sets.String {
	// collect all the possible places to use param references
	paramsRefs := []string{}
	paramsRefs = append(paramsRefs, extractParamRefsFromSteps(ts.Steps)...)
	paramsRefs = append(paramsRefs, extractParamRefsFromStepTemplate(ts.StepTemplate)...)
	paramsRefs = append(paramsRefs, extractParamRefsFromVolumes(ts.Volumes)...)
	for _, v := range ts.Workspaces {
		paramsRefs = append(paramsRefs, v.MountPath)
	}
	paramsRefs = append(paramsRefs, extractParamRefsFromSidecars(ts.Sidecars)...)
	// extract all array indexing references, for example []{"$(params.array-params[1])"}
	arrayIndexParamRefs := []string{}
	for _, p := range paramsRefs {
		arrayIndexParamRefs = append(arrayIndexParamRefs, extractArrayIndexingParamRefs(p)...)
	}
	return sets.NewString(arrayIndexParamRefs...)
}
