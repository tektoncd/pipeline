/*
Copyright 2018 The Knative Authors

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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/templating"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/validation"
)

func (t *Task) Validate(ctx context.Context) *apis.FieldError {
	if err := validateObjectMetadata(t.GetObjectMeta()); err != nil {
		return err.ViaField("metadata")
	}
	return t.Spec.Validate(ctx)
}

func (ts *TaskSpec) Validate(ctx context.Context) *apis.FieldError {
	if equality.Semantic.DeepEqual(ts, &TaskSpec{}) {
		return apis.ErrMissingField(apis.CurrentField)
	}

	if len(ts.Steps) == 0 {
		return apis.ErrMissingField("steps")
	}
	if err := ValidateVolumes(ts.Volumes).ViaField("volumes"); err != nil {
		return err
	}
	if err := validateSteps(ts.Steps).ViaField("steps"); err != nil {
		return err
	}

	// A task doesn't have to have inputs or outputs, but if it does they must be valid.
	// A task can't duplicate input or output names.

	if ts.Inputs != nil {
		for _, resource := range ts.Inputs.Resources {
			if err := validateResourceType(resource, fmt.Sprintf("taskspec.Inputs.Resources.%s.Type", resource.Name)); err != nil {
				return err
			}
		}
		if err := checkForDuplicates(ts.Inputs.Resources, "taskspec.Inputs.Resources.Name"); err != nil {
			return err
		}
	}
	if ts.Outputs != nil {
		for _, resource := range ts.Outputs.Resources {
			if err := validateResourceType(resource, fmt.Sprintf("taskspec.Outputs.Resources.%s.Type", resource.Name)); err != nil {
				return err
			}
		}
		if err := checkForDuplicates(ts.Outputs.Resources, "taskspec.Outputs.Resources.Name"); err != nil {
			return err
		}
	}

	// Validate task step names
	for _, step := range ts.Steps {
		if errs := validation.IsDNS1123Label(step.Name); len(errs) > 0 {
			return &apis.FieldError{
				Message: fmt.Sprintf("invalid value %q", step.Name),
				Paths:   []string{"taskspec.steps.name"},
				Details: "Task step name must be a valid DNS Label, For more info refer to https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names",
			}
		}
	}

	if err := validateInputParameterVariables(ts.Steps, ts.Inputs); err != nil {
		return err
	}
	if err := validateResourceVariables(ts.Steps, ts.Inputs, ts.Outputs); err != nil {
		return err
	}
	return nil
}

func ValidateVolumes(volumes []corev1.Volume) *apis.FieldError {
	// Task must not have duplicate volume names.
	vols := map[string]struct{}{}
	for _, v := range volumes {
		if _, ok := vols[v.Name]; ok {
			return apis.ErrMultipleOneOf("name")
		}
		vols[v.Name] = struct{}{}
	}
	return nil
}

func validateSteps(steps []corev1.Container) *apis.FieldError {
	// Task must not have duplicate step names.
	names := map[string]struct{}{}
	for _, s := range steps {
		if s.Image == "" {
			return apis.ErrMissingField("Image")
		}

		if s.Name == "" {
			continue
		}
		if _, ok := names[s.Name]; ok {
			return apis.ErrInvalidValue(s.Name, "name")
		}
		names[s.Name] = struct{}{}
	}
	return nil
}

func validateInputParameterVariables(steps []corev1.Container, inputs *Inputs) *apis.FieldError {
	parameterNames := map[string]struct{}{}
	if inputs != nil {
		for _, p := range inputs.Params {
			parameterNames[p.Name] = struct{}{}
		}
	}
	return validateVariables(steps, "params", "inputs.", parameterNames)
}

func validateResourceVariables(steps []corev1.Container, inputs *Inputs, outputs *Outputs) *apis.FieldError {
	var err *apis.FieldError
	// Keep track of input and output resources separately.
	// This ensures we can validate against appropriate variables set.
	inputVars := map[string]struct{}{}
	outputVars := map[string]struct{}{}
	if inputs != nil {
		inputVars, err = getResourceVariables(inputs.Resources, "taskspec.inputs.resources.")
		if err != nil {
			return err
		}
	}
	if outputs != nil {
		outputVars, err = getResourceVariables(outputs.Resources, "taskspec.outputs.resources.")
		if err != nil {
			return err
		}
	}
	err = validateVariables(steps, "resources", "inputs.", inputVars)
	if err != nil {
		return err
	}
	err = validateVariables(steps, "resources", "outputs.", outputVars)
	if err != nil {
		return err
	}
	return nil
}

func getResourceVariables(resources []TaskResource, pathPrefix string) (map[string]struct{}, *apis.FieldError) {
	vars := map[string]struct{}{}
	for _, r := range resources {
		attrs, err := AttributesFromType(r.Type)
		if err != nil {
			return nil, &apis.FieldError{
				Message: fmt.Sprintf("invalid resource type %s", r.Type),
				Paths:   []string{pathPrefix + r.Name},
				Details: err.Error(),
			}
		}
		for _, a := range attrs {
			rv := r.Name + "." + a
			vars[rv] = struct{}{}
		}
	}
	return vars, nil
}

func validateVariables(steps []corev1.Container, prefix, contextPrefix string, vars map[string]struct{}) *apis.FieldError {
	for _, step := range steps {
		if err := validateTaskVariable("name", step.Name, prefix, contextPrefix, vars); err != nil {
			return err
		}
		if err := validateTaskVariable("image", step.Image, prefix, contextPrefix, vars); err != nil {
			return err
		}
		if err := validateTaskVariable("workingDir", step.WorkingDir, prefix, contextPrefix, vars); err != nil {
			return err
		}
		for i, cmd := range step.Command {
			if err := validateTaskVariable(fmt.Sprintf("command[%d]", i), cmd, prefix, contextPrefix, vars); err != nil {
				return err
			}
		}
		for i, arg := range step.Args {
			if err := validateTaskVariable(fmt.Sprintf("arg[%d]", i), arg, prefix, contextPrefix, vars); err != nil {
				return err
			}
		}
		for _, env := range step.Env {
			if err := validateTaskVariable(fmt.Sprintf("env[%s]", env.Name), env.Value, prefix, contextPrefix, vars); err != nil {
				return err
			}
		}
		for i, v := range step.VolumeMounts {
			if err := validateTaskVariable(fmt.Sprintf("volumeMount[%d].Name", i), v.Name, prefix, contextPrefix, vars); err != nil {
				return err
			}
			if err := validateTaskVariable(fmt.Sprintf("volumeMount[%d].MountPath", i), v.MountPath, prefix, contextPrefix, vars); err != nil {
				return err
			}
			if err := validateTaskVariable(fmt.Sprintf("volumeMount[%d].SubPath", i), v.SubPath, prefix, contextPrefix, vars); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateTaskVariable(name, value, prefix, contextPrefix string, vars map[string]struct{}) *apis.FieldError {
	return templating.ValidateVariable(name, value, prefix, contextPrefix, "step", "taskspec.steps", vars)
}

func checkForDuplicates(resources []TaskResource, path string) *apis.FieldError {
	encountered := map[string]struct{}{}
	for _, r := range resources {
		if _, ok := encountered[strings.ToLower(r.Name)]; ok {
			return apis.ErrMultipleOneOf(path)
		}
		encountered[strings.ToLower(r.Name)] = struct{}{}
	}
	return nil
}

func validateResourceType(r TaskResource, path string) *apis.FieldError {
	for _, allowed := range AllResourceTypes {
		if r.Type == allowed {
			return nil
		}
	}
	return apis.ErrInvalidValue(string(r.Type), path)
}
