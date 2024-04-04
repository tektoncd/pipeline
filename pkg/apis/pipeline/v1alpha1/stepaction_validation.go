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

package v1alpha1

import (
	"context"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/validate"
	"github.com/tektoncd/pipeline/pkg/substitution"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/webhook/resourcesemantics"
)

var (
	_ apis.Validatable              = (*StepAction)(nil)
	_ resourcesemantics.VerbLimited = (*StepAction)(nil)
)

// SupportedVerbs returns the operations that validation should be called for
func (s *StepAction) SupportedVerbs() []admissionregistrationv1.OperationType {
	return []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
}

// Validate implements apis.Validatable
func (s *StepAction) Validate(ctx context.Context) (errs *apis.FieldError) {
	errs = validate.ObjectMetadata(s.GetObjectMeta()).ViaField("metadata")
	errs = errs.Also(s.Spec.Validate(apis.WithinSpec(ctx)).ViaField("spec"))
	return errs
}

// Validate implements apis.Validatable
func (ss *StepActionSpec) Validate(ctx context.Context) (errs *apis.FieldError) {
	if ss.Image == "" {
		errs = errs.Also(apis.ErrMissingField("Image"))
	}

	if ss.Script != "" {
		if len(ss.Command) > 0 {
			errs = errs.Also(&apis.FieldError{
				Message: "script cannot be used with command",
				Paths:   []string{"script"},
			})
		}

		cleaned := strings.TrimSpace(ss.Script)
		if strings.HasPrefix(cleaned, "#!win") {
			errs = errs.Also(config.ValidateEnabledAPIFields(ctx, "windows script support", config.AlphaAPIFields).ViaField("script"))
		}
		errs = errs.Also(validateNoParamSubstitutionsInScript(ss.Script))
	}
	errs = errs.Also(validateUsageOfDeclaredParameters(ctx, *ss))
	errs = errs.Also(v1.ValidateParameterTypes(ctx, ss.Params).ViaField("params"))
	errs = errs.Also(validateParameterVariables(ctx, *ss, ss.Params))
	errs = errs.Also(v1.ValidateStepResultsVariables(ctx, ss.Results, ss.Script))
	errs = errs.Also(v1.ValidateStepResults(ctx, ss.Results).ViaField("results"))
	errs = errs.Also(validateVolumeMounts(ss.VolumeMounts, ss.Params).ViaField("volumeMounts"))
	return errs
}

// validateNoParamSubstitutionsInScript validates that param substitutions are not invoked in the script
func validateNoParamSubstitutionsInScript(script string) *apis.FieldError {
	_, present, errString := substitution.ExtractVariablesFromString(script, "params")
	if errString != "" || present {
		return &apis.FieldError{
			Message: "param substitution in scripts is not allowed.",
			Paths:   []string{"script"},
		}
	}
	return nil
}

// validateUsageOfDeclaredParameters validates that all parameters referenced in the Task are declared by the Task.
func validateUsageOfDeclaredParameters(ctx context.Context, sas StepActionSpec) *apis.FieldError {
	params := sas.Params
	var errs *apis.FieldError
	_, _, objectParams := params.SortByType()
	allParameterNames := sets.NewString(params.GetNames()...)
	errs = errs.Also(validateStepActionVariables(ctx, sas, "params", allParameterNames))
	errs = errs.Also(validateObjectUsage(ctx, sas, objectParams))
	errs = errs.Also(v1.ValidateObjectParamsHaveProperties(ctx, params))
	return errs
}

func validateVolumeMounts(volumeMounts []corev1.VolumeMount, params v1.ParamSpecs) (errs *apis.FieldError) {
	if len(volumeMounts) == 0 {
		return
	}
	paramNames := sets.String{}
	for _, p := range params {
		paramNames.Insert(p.Name)
	}
	for idx, v := range volumeMounts {
		matches, _ := substitution.ExtractVariableExpressions(v.Name, "params")
		if len(matches) != 1 {
			errs = errs.Also(apis.ErrInvalidValue(v.Name, "name", "expect the Name to be a single param reference").ViaIndex(idx))
			return errs
		} else if matches[0] != v.Name {
			errs = errs.Also(apis.ErrInvalidValue(v.Name, "name", "expect the Name to be a single param reference").ViaIndex(idx))
			return errs
		}
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(v.Name, "params", paramNames).ViaIndex(idx))
	}
	return errs
}

// validateParameterVariables validates all variables within a slice of ParamSpecs against a StepAction
func validateParameterVariables(ctx context.Context, sas StepActionSpec, params v1.ParamSpecs) *apis.FieldError {
	var errs *apis.FieldError
	errs = errs.Also(params.ValidateNoDuplicateNames())
	stringParams, arrayParams, objectParams := params.SortByType()
	stringParameterNames := sets.NewString(stringParams.GetNames()...)
	arrayParameterNames := sets.NewString(arrayParams.GetNames()...)
	errs = errs.Also(v1.ValidateNameFormat(stringParameterNames.Insert(arrayParameterNames.List()...), objectParams))
	return errs.Also(validateStepActionArrayUsage(sas, "params", arrayParameterNames))
}

// validateObjectUsage validates the usage of individual attributes of an object param and the usage of the entire object
func validateObjectUsage(ctx context.Context, sas StepActionSpec, params v1.ParamSpecs) (errs *apis.FieldError) {
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
		errs = errs.Also(validateStepActionVariables(ctx, sas, "params\\."+p.Name, objectKeys))
	}

	return errs.Also(validateStepActionObjectUsageAsWhole(sas, "params", objectParameterNames))
}

// validateStepActionObjectUsageAsWhole returns an error if the StepAction contains references to the entire input object params in fields where these references are prohibited
func validateStepActionObjectUsageAsWhole(sas StepActionSpec, prefix string, vars sets.String) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToEntireProhibitedVariables(sas.Image, prefix, vars).ViaField("image")
	errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(sas.Script, prefix, vars).ViaField("script"))
	for i, cmd := range sas.Command {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range sas.Args {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range sas.Env {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, vm := range sas.VolumeMounts {
		errs = errs.Also(substitution.ValidateNoReferencesToEntireProhibitedVariables(vm.Name, prefix, vars).ViaFieldIndex("volumeMounts", i))
	}
	return errs
}

// validateStepActionArrayUsage returns an error if the Step contains references to the input array params in fields where these references are prohibited
func validateStepActionArrayUsage(sas StepActionSpec, prefix string, arrayParamNames sets.String) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToProhibitedVariables(sas.Image, prefix, arrayParamNames).ViaField("image")
	errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(sas.Script, prefix, arrayParamNames).ViaField("script"))
	for i, cmd := range sas.Command {
		errs = errs.Also(substitution.ValidateVariableReferenceIsIsolated(cmd, prefix, arrayParamNames).ViaFieldIndex("command", i))
	}
	for i, arg := range sas.Args {
		errs = errs.Also(substitution.ValidateVariableReferenceIsIsolated(arg, prefix, arrayParamNames).ViaFieldIndex("args", i))
	}
	for _, env := range sas.Env {
		errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(env.Value, prefix, arrayParamNames).ViaFieldKey("env", env.Name))
	}
	for i, vm := range sas.VolumeMounts {
		errs = errs.Also(substitution.ValidateNoReferencesToProhibitedVariables(vm.Name, prefix, arrayParamNames).ViaFieldIndex("volumeMounts", i))
	}
	return errs
}

// validateStepActionVariables returns an error if the StepAction contains references to any unknown variables
func validateStepActionVariables(ctx context.Context, sas StepActionSpec, prefix string, vars sets.String) *apis.FieldError {
	errs := substitution.ValidateNoReferencesToUnknownVariables(sas.Image, prefix, vars).ViaField("image")
	errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(sas.Script, prefix, vars).ViaField("script"))
	for i, cmd := range sas.Command {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(cmd, prefix, vars).ViaFieldIndex("command", i))
	}
	for i, arg := range sas.Args {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(arg, prefix, vars).ViaFieldIndex("args", i))
	}
	for _, env := range sas.Env {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(env.Value, prefix, vars).ViaFieldKey("env", env.Name))
	}
	for i, vm := range sas.VolumeMounts {
		errs = errs.Also(substitution.ValidateNoReferencesToUnknownVariables(vm.Name, prefix, vars).ViaFieldIndex("volumeMounts", i))
	}
	return errs
}
