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
	"regexp"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/apis/version"
	"github.com/tektoncd/pipeline/pkg/substitution"
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

var _ apis.Validatable = (*Task)(nil)
var _ resourcesemantics.VerbLimited = (*Task)(nil)

// SupportedVerbs returns the operations that validation should be called for
func (t *Task) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

var stringAndArrayVariableNameFormatRegex = regexp.MustCompile(stringAndArrayVariableNameFormat)
var objectVariableNameFormatRegex = regexp.MustCompile(objectVariableNameFormat)

// Validate implements apis.Validatable
func (t *Task) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(t.GetObjectMeta()).ViaField("metadata")
	ctx = config.SkipValidationDueToPropagatedParametersAndWorkspaces(ctx, false)
	return errs.Also(t.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
}

// Validate implements apis.Validatable
func (ts *TaskSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if len(ts.Steps) == 0 {
		errs = errs.Also(apis.ErrMissingField("steps"))
	}

	if config.IsSubstituted(ctx) {
		// Validate the task's workspaces only.
		errs = errs.Also(validateDeclaredWorkspaces(ts.Workspaces, ts.Steps, ts.StepTemplate).ViaField("workspaces"))
		errs = errs.Also(validateWorkspaceUsages(ctx, ts))

		return errs
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
// This is an alpha feature and will fail validation if it's used by a step
// or sidecar when the enable-api-fields feature gate is anything but "alpha".
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
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "step workspaces", config.AlphaAPIFields).ViaIndex(stepIdx).ViaField("steps"))
		}
		for workspaceIdx, w := range step.Workspaces {
			if !wsNames.Has(w.Name) {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("undefined workspace %q", w.Name), "name").ViaIndex(workspaceIdx).ViaField("workspaces").ViaIndex(stepIdx).ViaField("steps"))
			}
		}
	}

	for sidecarIdx, sidecar := range sidecars {
		if len(sidecar.Workspaces) != 0 {
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "sidecar workspaces", config.AlphaAPIFields).ViaIndex(sidecarIdx).ViaField("sidecars"))
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
	}
	return errs
}

func validateStep(ctx context.Context, s Step, names sets.String) (errs *apis.FieldError) {
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
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "windows script support", config.AlphaAPIFields).ViaField("script"))
		}
	}

	// StdoutConfig is an alpha feature and will fail validation if it's used in a task spec
	// when the enable-api-fields feature gate is not "alpha".
	if s.StdoutConfig != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "step stdout stream support", config.AlphaAPIFields).ViaField("stdoutconfig"))
	}
	// StderrConfig is an alpha feature and will fail validation if it's used in a task spec
	// when the enable-api-fields feature gate is not "alpha".
	if s.StderrConfig != nil {
		errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "step stderr stream support", config.AlphaAPIFields).ViaField("stderrconfig"))
	}
	return errs
}

// ValidateParameterTypes validates all the types within a slice of ParamSpecs
func ValidateParameterTypes(ctx context.Context, params []ParamSpec) (errs *apis.FieldError) {
	for _, p := range params {
		if p.Type == ParamTypeObject {
			// Object type parameter is a beta feature and will fail validation if it's used in a task spec
			// when the enable-api-fields feature gate is not "alpha" or "beta".
			errs = errs.Also(version.ValidateEnabledAPIFields(ctx, "object type parameter", config.BetaAPIFields))
		}
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
		return apis.ErrInvalidValue(p.Type, fmt.Sprintf("%s.type", p.Name))
	}

	// If a default value is provided, ensure its type matches param's declared type.
	if (p.Default != nil) && (p.Default.Type != p.Type) {
		return &apis.FieldError{
			Message: fmt.Sprintf(
				"\"%v\" type does not match default value's type: \"%v\"", p.Type, p.Default.Type),
			Paths: []string{
				fmt.Sprintf("%s.type", p.Name),
				fmt.Sprintf("%s.default.type", p.Name),
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
	if p.Type == ParamTypeObject && p.Properties == nil {
		// If this we are not skipping validation checks due to propagated params
		// then properties field is required.
		if config.ValidateParameterVariablesAndWorkspaces(ctx) {
			return apis.ErrMissingField(fmt.Sprintf("%s.properties", p.Name))
		}
	}

	invalidKeys := []string{}
	for key, propertySpec := range p.Properties {
		if propertySpec.Type != ParamTypeString {
			invalidKeys = append(invalidKeys, key)
		}
	}

	if len(invalidKeys) != 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("The value type specified for these keys %v is invalid", invalidKeys),
			Paths:   []string{fmt.Sprintf("%s.properties", p.Name)},
		}
	}

	return nil
}

// ValidateParameterVariables validates all variables within a slice of ParamSpecs against a slice of Steps
func ValidateParameterVariables(ctx context.Context, steps []Step, params []ParamSpec) *apis.FieldError {
	allParameterNames := sets.NewString()
	stringParameterNames := sets.NewString()
	arrayParameterNames := sets.NewString()
	objectParamSpecs := []ParamSpec{}
	var errs *apis.FieldError
	for _, p := range params {
		// validate no duplicate names
		if allParameterNames.Has(p.Name) {
			errs = errs.Also(apis.ErrGeneric("parameter appears more than once", "").ViaFieldKey("params", p.Name))
		}
		allParameterNames.Insert(p.Name)

		switch p.Type {
		case ParamTypeArray:
			arrayParameterNames.Insert(p.Name)
		case ParamTypeObject:
			objectParamSpecs = append(objectParamSpecs, p)
		case ParamTypeString:
			fallthrough
		default:
			stringParameterNames.Insert(p.Name)
		}
	}
	errs = errs.Also(validateNameFormat(stringParameterNames.Insert(arrayParameterNames.List()...), objectParamSpecs))
	if config.ValidateParameterVariablesAndWorkspaces(ctx) {
		errs = errs.Also(validateVariables(ctx, steps, "params", allParameterNames))
		errs = errs.Also(validateObjectUsage(ctx, steps, objectParamSpecs))
	}
	return errs.Also(validateArrayUsage(steps, "params", arrayParameterNames))
}

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
		errs = errs.Also(validateTaskVariable(step.Script, "results", resultsNames).ViaField("script").ViaFieldIndex("steps", idx))
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
		errs = errs.Also(validateVariables(ctx, steps, fmt.Sprintf("params\\.%s", p.Name), objectKeys))
	}

	return errs.Also(validateObjectUsageAsWhole(steps, "params", objectParameterNames))
}

// validateObjectUsageAsWhole makes sure the object params are not used as whole when providing values for strings
// i.e. param.objectParam, param.objectParam[*]
func validateObjectUsageAsWhole(steps []Step, prefix string, vars sets.String) (errs *apis.FieldError) {
	for idx, step := range steps {
		errs = errs.Also(validateStepObjectUsageAsWhole(step, prefix, vars)).ViaFieldIndex("steps", idx)
	}
	return errs
}

func validateStepObjectUsageAsWhole(step Step, prefix string, vars sets.String) *apis.FieldError {
	errs := validateTaskNoObjectReferenced(step.Name, prefix, vars).ViaField("name")
	errs = errs.Also(validateTaskNoObjectReferenced(step.Image, prefix, vars).ViaField("image"))
	errs = errs.Also(validateTaskNoObjectReferenced(step.WorkingDir, prefix, vars).ViaField("workingDir"))
	errs = errs.Also(validateTaskNoObjectReferenced(step.Script, prefix, vars).ViaField("script"))
	for i, cmd := range step.Command {
		errs = errs.Also(validateTaskNoObjectReferenced(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range step.Args {
		errs = errs.Also(validateTaskNoObjectReferenced(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range step.Env {
		errs = errs.Also(validateTaskNoObjectReferenced(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, v := range step.VolumeMounts {
		errs = errs.Also(validateTaskNoObjectReferenced(v.Name, prefix, vars).ViaField("name").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(validateTaskNoObjectReferenced(v.MountPath, prefix, vars).ViaField("mountPath").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(validateTaskNoObjectReferenced(v.SubPath, prefix, vars).ViaField("subPath").ViaFieldIndex("volumeMount", i))
	}
	return errs
}

func validateArrayUsage(steps []Step, prefix string, vars sets.String) (errs *apis.FieldError) {
	for idx, step := range steps {
		errs = errs.Also(validateStepArrayUsage(step, prefix, vars)).ViaFieldIndex("steps", idx)
	}
	return errs
}

func validateStepArrayUsage(step Step, prefix string, vars sets.String) *apis.FieldError {
	errs := validateTaskNoArrayReferenced(step.Name, prefix, vars).ViaField("name")
	errs = errs.Also(validateTaskNoArrayReferenced(step.Image, prefix, vars).ViaField("image"))
	errs = errs.Also(validateTaskNoArrayReferenced(step.WorkingDir, prefix, vars).ViaField("workingDir"))
	errs = errs.Also(validateTaskNoArrayReferenced(step.Script, prefix, vars).ViaField("script"))
	for i, cmd := range step.Command {
		errs = errs.Also(validateTaskArraysIsolated(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range step.Args {
		errs = errs.Also(validateTaskArraysIsolated(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range step.Env {
		errs = errs.Also(validateTaskNoArrayReferenced(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, v := range step.VolumeMounts {
		errs = errs.Also(validateTaskNoArrayReferenced(v.Name, prefix, vars).ViaField("name").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(validateTaskNoArrayReferenced(v.MountPath, prefix, vars).ViaField("mountPath").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(validateTaskNoArrayReferenced(v.SubPath, prefix, vars).ViaField("subPath").ViaFieldIndex("volumeMount", i))
	}
	return errs
}

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

func validateStepVariables(ctx context.Context, step Step, prefix string, vars sets.String) *apis.FieldError {
	errs := validateTaskVariable(step.Name, prefix, vars).ViaField("name")
	errs = errs.Also(validateTaskVariable(step.Image, prefix, vars).ViaField("image"))
	errs = errs.Also(validateTaskVariable(step.WorkingDir, prefix, vars).ViaField("workingDir"))
	errs = errs.Also(validateTaskVariable(step.Script, prefix, vars).ViaField("script"))
	for i, cmd := range step.Command {
		errs = errs.Also(validateTaskVariable(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range step.Args {
		errs = errs.Also(validateTaskVariable(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range step.Env {
		errs = errs.Also(validateTaskVariable(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, v := range step.VolumeMounts {
		errs = errs.Also(validateTaskVariable(v.Name, prefix, vars).ViaField("name").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(validateTaskVariable(v.MountPath, prefix, vars).ViaField("MountPath").ViaFieldIndex("volumeMount", i))
		errs = errs.Also(validateTaskVariable(v.SubPath, prefix, vars).ViaField("SubPath").ViaFieldIndex("volumeMount", i))
	}
	errs = errs.Also(validateTaskVariable(string(step.OnError), prefix, vars).ViaField("onError"))
	return errs
}

func validateTaskVariable(value, prefix string, vars sets.String) *apis.FieldError {
	return substitution.ValidateVariableP(value, prefix, vars)
}

func validateTaskNoObjectReferenced(value, prefix string, objectNames sets.String) *apis.FieldError {
	return substitution.ValidateEntireVariableProhibitedP(value, prefix, objectNames)
}

func validateTaskNoArrayReferenced(value, prefix string, arrayNames sets.String) *apis.FieldError {
	return substitution.ValidateVariableProhibitedP(value, prefix, arrayNames)
}

func validateTaskArraysIsolated(value, prefix string, arrayNames sets.String) *apis.FieldError {
	return substitution.ValidateVariableIsolatedP(value, prefix, arrayNames)
}

// isParamRefs attempts to check if a specified string looks like it contains any parameter reference
// This is useful to make sure the specified value looks like a Parameter Reference before performing any strict validation
func isParamRefs(s string) bool {
	return strings.HasPrefix(s, "$("+ParamsPrefix)
}

// ValidateParamArrayIndex validates if the param reference to an array param is out of bound.
// error is returned when the array indexing reference is out of bound of the array param
// e.g. if a param reference of $(params.array-param[2]) and the array param is of length 2.
// - `trParams` are params from taskrun.
// - `taskSpec` contains params declarations.
func (ts *TaskSpec) ValidateParamArrayIndex(ctx context.Context, params Params) error {
	cfg := config.FromContextOrDefaults(ctx)
	if cfg.FeatureFlags.EnableAPIFields != config.AlphaAPIFields {
		return nil
	}

	// Collect all array params lengths
	arrayParamsLengths := ts.Params.extractParamArrayLengths()
	for k, v := range params.extractParamArrayLengths() {
		arrayParamsLengths[k] = v
	}

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

	return validateOutofBoundArrayParams(arrayIndexParamRefs, arrayParamsLengths)
}
