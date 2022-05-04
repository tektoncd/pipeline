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
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/list"
	"github.com/tektoncd/pipeline/pkg/substitution"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
)

var _ apis.Validatable = (*Task)(nil)

const variableNameFormat = "^[_a-zA-Z][_a-zA-Z0-9.-]*$"

// Validate implements apis.Validatable
func (t *Task) Validate(ctx context.Context) *apis.FieldError {
	errs := validate.ObjectMetadata(t.GetObjectMeta()).ViaField("metadata")
	if apis.IsInDelete(ctx) {
		return nil
	}
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
	errs = errs.Also(ts.Resources.Validate(ctx).ViaField("resources"))
	errs = errs.Also(ValidateParameterTypes(ctx, ts.Params).ViaField("params"))
	errs = errs.Also(ValidateParameterVariables(ctx, ts.Steps, ts.Params))
	errs = errs.Also(ValidateResourcesVariables(ctx, ts.Steps, ts.Resources))
	errs = errs.Also(validateTaskContextVariables(ctx, ts.Steps))
	errs = errs.Also(validateResults(ctx, ts.Results).ViaField("results"))
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
			errs = errs.Also(ValidateEnabledAPIFields(ctx, "step workspaces", config.AlphaAPIFields).ViaIndex(stepIdx).ViaField("steps"))
		}
		for workspaceIdx, w := range step.Workspaces {
			if !wsNames.Has(w.Name) {
				errs = errs.Also(apis.ErrGeneric(fmt.Sprintf("undefined workspace %q", w.Name), "name").ViaIndex(workspaceIdx).ViaField("workspaces").ViaIndex(stepIdx).ViaField("steps"))
			}
		}
	}

	for sidecarIdx, sidecar := range sidecars {
		if len(sidecar.Workspaces) != 0 {
			errs = errs.Also(ValidateEnabledAPIFields(ctx, "sidecar workspaces", config.AlphaAPIFields).ViaIndex(sidecarIdx).ViaField("sidecars"))
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
				Message: fmt.Sprintf("script cannot be used with command"),
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
		if s.OnError != "continue" && s.OnError != "stopAndFail" {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid value: %v", s.OnError),
				Paths:   []string{"onError"},
				Details: "Task step onError must be either continue or stopAndFail",
			})
		}
	}

	if s.Script != "" {
		cleaned := strings.TrimSpace(s.Script)
		if strings.HasPrefix(cleaned, "#!win") {
			errs = errs.Also(ValidateEnabledAPIFields(ctx, "windows script support", config.AlphaAPIFields).ViaField("script"))
		}
	}
	return errs
}

// ValidateParameterTypes validates all the types within a slice of ParamSpecs
func ValidateParameterTypes(ctx context.Context, params []ParamSpec) (errs *apis.FieldError) {
	for _, p := range params {
		if p.Type == ParamTypeObject {
			// Object type parameter is an alpha feature and will fail validation if it's used in a task spec
			// when the enable-api-fields feature gate is not "alpha".
			errs = errs.Also(ValidateEnabledAPIFields(ctx, "object type parameter", config.AlphaAPIFields))
		}
		errs = errs.Also(p.ValidateType())
	}
	return errs
}

// ValidateType checks that the type of a ParamSpec is allowed and its default value matches that type
func (p ParamSpec) ValidateType() *apis.FieldError {
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
	return p.ValidateObjectType()
}

// ValidateObjectType checks that object type parameter does not miss the
// definition of `properties` section and the type of a PropertySpec is allowed.
// (Currently, only string is allowed)
func (p ParamSpec) ValidateObjectType() *apis.FieldError {
	if p.Type == ParamTypeObject && p.Properties == nil {
		return apis.ErrMissingField(fmt.Sprintf("%s.properties", p.Name))
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
	parameterNames := sets.NewString()
	arrayParameterNames := sets.NewString()
	objectParamSpecs := []ParamSpec{}
	var errs *apis.FieldError
	for _, p := range params {
		// validate no duplicate names
		if parameterNames.Has(p.Name) {
			errs = errs.Also(apis.ErrGeneric("parameter appears more than once", "").ViaFieldKey("params", p.Name))
		}
		parameterNames.Insert(p.Name)
		if p.Type == ParamTypeArray {
			arrayParameterNames.Insert(p.Name)
		}
		if p.Type == ParamTypeObject {
			objectParamSpecs = append(objectParamSpecs, p)
		}
	}

	errs = errs.Also(validateVariables(ctx, steps, "params", parameterNames))
	errs = errs.Also(validateArrayUsage(steps, "params", arrayParameterNames))
	return errs.Also(validateObjectUsage(ctx, steps, objectParamSpecs))
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

// ValidateResourcesVariables validates all variables within a TaskResources against a slice of Steps
func ValidateResourcesVariables(ctx context.Context, steps []Step, resources *TaskResources) *apis.FieldError {
	if resources == nil {
		return nil
	}
	resourceNames := sets.NewString()
	if resources.Inputs != nil {
		for _, r := range resources.Inputs {
			resourceNames.Insert(r.Name)
		}
	}
	if resources.Outputs != nil {
		for _, r := range resources.Outputs {
			resourceNames.Insert(r.Name)
		}
	}
	return validateVariables(ctx, steps, "resources.(?:inputs|outputs)", resourceNames)
}

// TODO (@chuangw6): Make sure an object param is not used as a whole when providing values for strings.
// https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md#variable-replacement-with-object-params
// "When providing values for strings, Task and Pipeline authors can access
// individual attributes of an object param; they cannot access the object
// as whole (we could add support for this later)."
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

		if p.Default != nil && p.Default.ObjectVal != nil {
			errs = errs.Also(validateObjectKeysInDefault(p.Default.ObjectVal, objectKeys, p.Name))
		}

		// check if the object's key names are referenced correctly i.e. param.objectParam.key1
		errs = errs.Also(validateVariables(ctx, steps, fmt.Sprintf("params\\.%s", p.Name), objectKeys))
	}

	return errs
}

// validate if object keys defined in properties are all provided in default
func validateObjectKeysInDefault(defaultObject map[string]string, neededObjectKeys sets.String, paramName string) (errs *apis.FieldError) {
	neededObjectKeysInSpec := neededObjectKeys.List()
	providedObjectKeysInDefault := []string{}
	for k := range defaultObject {
		providedObjectKeysInDefault = append(providedObjectKeysInDefault, k)
	}

	missingObjectKeys := list.DiffLeft(neededObjectKeysInSpec, providedObjectKeysInDefault)
	if len(missingObjectKeys) != 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("Required key(s) %s for the parameter %s are not provided in default.", missingObjectKeys, paramName),
			Paths:   []string{fmt.Sprintf("%s.properties", paramName), fmt.Sprintf("%s.default", paramName)},
		}
	}
	return nil
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
	// validate that the variable name format follows the rules
	// - Must only contain alphanumeric characters, hyphens (-), underscores (_), and dots (.)
	// - Must begin with a letter or an underscore (_)
	re := regexp.MustCompile(variableNameFormat)
	invalidNames := []string{}
	// Converting to sorted list here rather than just looping map keys
	// because we want the order of items in vars to be deterministic for purpose of unit testing
	for _, name := range vars.List() {
		if !re.MatchString(name) {
			invalidNames = append(invalidNames, name)
		}
	}

	if len(invalidNames) != 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("The format of following variable names is invalid. %s", invalidNames),
			Paths:   []string{"params"},
			Details: "Names: \nMust only contain alphanumeric characters, hyphens (-), underscores (_), and dots (.)\nMust begin with a letter or an underscore (_)",
		}
	}

	// We've checked param name format. Now, we want to check if param names are referenced correctly in each step
	for idx, step := range steps {
		errs = errs.Also(validateStepVariables(ctx, step, prefix, vars).ViaFieldIndex("steps", idx))
	}
	return errs
}

func validateStepVariables(ctx context.Context, step Step, prefix string, vars sets.String) *apis.FieldError {
	errs := validateTaskVariable(step.Name, prefix, vars).ViaField("name")
	errs = errs.Also(validateTaskVariable(step.Image, prefix, vars).ViaField("image"))
	errs = errs.Also(validateTaskVariable(step.WorkingDir, prefix, vars).ViaField("workingDir"))
	if !(config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha" && prefix == "params") {
		errs = errs.Also(validateTaskVariable(step.Script, prefix, vars).ViaField("script"))
	}
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
	return errs
}

func validateTaskVariable(value, prefix string, vars sets.String) *apis.FieldError {
	return substitution.ValidateVariableP(value, prefix, vars)
}

func validateTaskNoArrayReferenced(value, prefix string, arrayNames sets.String) *apis.FieldError {
	return substitution.ValidateVariableProhibitedP(value, prefix, arrayNames)
}

func validateTaskArraysIsolated(value, prefix string, arrayNames sets.String) *apis.FieldError {
	return substitution.ValidateVariableIsolatedP(value, prefix, arrayNames)
}
