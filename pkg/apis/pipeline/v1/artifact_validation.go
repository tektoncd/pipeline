/*
Copyright 2026 The Tekton Authors

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
	"regexp"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

// stepArtifactValuePattern matches $(steps.<step>.artifacts.<name>)
var stepArtifactValuePattern = regexp.MustCompile(`^\$\(steps\.([^.]+)\.artifacts\.([^.)]+)\)$`)

// resolveArtifactStorageForValidation resolves the 4-level artifact storage
// hierarchy and returns a context with the resolved config. When a KubeClient
// is available (webhook admission), it reads the namespace ConfigMap. The
// annotations from the object (Pipeline/PipelineRun/TaskRun) are merged
// as the highest-priority overrides.
func resolveArtifactStorageForValidation(ctx context.Context, namespace string, annotations map[string]string) context.Context {
	cfg := config.FromContextOrDefaults(ctx)
	if cfg == nil || cfg.ArtifactStorage == nil {
		return ctx
	}

	var nsOverrides map[string]string
	if kc, ok := ctx.Value(kubeclient.Key{}).(kubernetes.Interface); ok && kc != nil && namespace != "" {
		cm, err := kc.CoreV1().ConfigMaps(namespace).Get(ctx, config.ArtifactStorageConfigName, metav1.GetOptions{})
		if err == nil {
			nsOverrides = cm.Data
		}
	}

	resolved, err := config.ResolveArtifactStorage(cfg.ArtifactStorage, nsOverrides, annotations)
	if err != nil {
		return ctx
	}

	cfgCopy := *cfg
	cfgCopy.ArtifactStorage = resolved
	return config.ToContext(ctx, &cfgCopy)
}

// validateArtifactStorageConfig checks the artifact storage configuration from
// the context. The caller is expected to have resolved the 4-level hierarchy
// before calling this function.
func validateArtifactStorageConfig(ctx context.Context) *apis.FieldError {
	cfg := config.FromContextOrDefaults(ctx)
	if cfg == nil || cfg.ArtifactStorage == nil {
		return nil
	}
	as := cfg.ArtifactStorage
	if as.OCIRepository != "" && !as.Enabled {
		return apis.ErrGeneric(fmt.Sprintf("artifact storage config is invalid: oci-repository is set to %q but enabled is false; set enabled to true to activate artifact storage", as.OCIRepository), "")
	}
	return nil
}

// ValidateArtifactStorageAnnotations validates the tekton.dev/artifact-storage-*
// annotations on any object (PipelineRun, Pipeline, TaskRun). Invalid values
// are rejected at admission time rather than causing runtime errors.
func ValidateArtifactStorageAnnotations(annotations map[string]string) *apis.FieldError {
	if len(annotations) == 0 {
		return nil
	}

	var errs *apis.FieldError

	if v, ok := annotations[config.ArtifactStorageEnabledAnnotation]; ok {
		if _, err := strconv.ParseBool(v); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("invalid boolean %q", v),
				"metadata.annotations["+config.ArtifactStorageEnabledAnnotation+"]"))
		}
	}

	if v, ok := annotations[config.ArtifactStorageInlineThresholdAnnotation]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("invalid integer %q", v),
				"metadata.annotations["+config.ArtifactStorageInlineThresholdAnnotation+"]"))
		} else if n < 0 || n > config.MaxInlineThreshold {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("must be 0..%d, got %d", config.MaxInlineThreshold, n),
				"metadata.annotations["+config.ArtifactStorageInlineThresholdAnnotation+"]"))
		}
	}

	if v, ok := annotations[config.ArtifactStorageInsecureAnnotation]; ok {
		if _, err := strconv.ParseBool(v); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("invalid boolean %q", v),
				"metadata.annotations["+config.ArtifactStorageInsecureAnnotation+"]"))
		}
	}

	if v, ok := annotations[config.ArtifactStorageOCIAttachReferrersAnnotation]; ok {
		if _, err := strconv.ParseBool(v); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("invalid boolean %q", v),
				"metadata.annotations["+config.ArtifactStorageOCIAttachReferrersAnnotation+"]"))
		}
	}

	if v, ok := annotations[config.ArtifactStorageOCIGroupByPipelineRunAnnotation]; ok {
		if _, err := strconv.ParseBool(v); err != nil {
			errs = errs.Also(apis.ErrInvalidValue(
				fmt.Sprintf("invalid boolean %q", v),
				"metadata.annotations["+config.ArtifactStorageOCIGroupByPipelineRunAnnotation+"]"))
		}
	}

	return errs
}

// ValidateArtifactDeclarations validates artifact declarations on a TaskSpec.
func ValidateArtifactDeclarations(ctx context.Context, decls *ArtifactDeclarations) *apis.FieldError {
	if decls == nil {
		return nil
	}

	var errs *apis.FieldError

	cfg := config.FromContextOrDefaults(ctx)
	if cfg == nil || cfg.FeatureFlags == nil || !cfg.FeatureFlags.EnableArtifacts {
		return errs.Also(apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature", config.EnableArtifacts), ""))
	}
	if err := validateArtifactStorageConfig(ctx); err != nil {
		return errs.Also(err)
	}

	// Validate inputs
	inputNames := map[string]bool{}
	for i, input := range decls.Inputs {
		if input.Name == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("inputs[%d].name", i)))
		} else if inputNames[input.Name] {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("duplicate artifact input name %q", input.Name),
				Paths:   []string{fmt.Sprintf("inputs[%d].name", i)},
			})
		}
		inputNames[input.Name] = true
		if input.Type != "" && input.Type != ArtifactTypeContent && input.Type != ArtifactTypeReference {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid artifact type %q, must be %q or %q", input.Type, ArtifactTypeContent, ArtifactTypeReference),
				Paths:   []string{fmt.Sprintf("inputs[%d].type", i)},
			})
		}
		if input.Subject {
			errs = errs.Also(&apis.FieldError{
				Message: "subject is not allowed on artifact inputs, only on outputs",
				Paths:   []string{fmt.Sprintf("inputs[%d].subject", i)},
			})
		}
		if input.Value != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "value is not allowed on artifact inputs, only on outputs",
				Paths:   []string{fmt.Sprintf("inputs[%d].value", i)},
			})
		}
	}

	// Validate outputs
	outputNames := map[string]bool{}
	for i, output := range decls.Outputs {
		if output.Name == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("outputs[%d].name", i)))
		} else if outputNames[output.Name] {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("duplicate artifact output name %q", output.Name),
				Paths:   []string{fmt.Sprintf("outputs[%d].name", i)},
			})
		}
		outputNames[output.Name] = true
		if output.Type != "" && output.Type != ArtifactTypeContent && output.Type != ArtifactTypeReference {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid artifact type %q, must be %q or %q", output.Type, ArtifactTypeContent, ArtifactTypeReference),
				Paths:   []string{fmt.Sprintf("outputs[%d].type", i)},
			})
		}
		if output.Value != "" && !stepArtifactValuePattern.MatchString(output.Value) {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid value %q, must match $(steps.<step>.artifacts.<name>)", output.Value),
				Paths:   []string{fmt.Sprintf("outputs[%d].value", i)},
			})
		}
	}

	return errs
}

// ValidateStepArtifacts validates step-scoped artifact declarations.
func ValidateStepArtifacts(ctx context.Context, artifacts *StepArtifacts) *apis.FieldError {
	if artifacts == nil {
		return nil
	}

	cfg := config.FromContextOrDefaults(ctx)
	if cfg == nil || cfg.FeatureFlags == nil || !cfg.FeatureFlags.EnableArtifacts {
		return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature", config.EnableArtifacts), "")
	}
	if err := validateArtifactStorageConfig(ctx); err != nil {
		return err
	}

	var errs *apis.FieldError
	outputNames := map[string]bool{}
	for i, output := range artifacts.Outputs {
		if output.Name == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("outputs[%d].name", i)))
		} else if outputNames[output.Name] {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("duplicate artifact output name %q", output.Name),
				Paths:   []string{fmt.Sprintf("outputs[%d].name", i)},
			})
		}
		outputNames[output.Name] = true
		if output.Type != "" && output.Type != ArtifactTypeContent && output.Type != ArtifactTypeReference {
			errs = errs.Also(&apis.FieldError{
				Message: fmt.Sprintf("invalid artifact type %q, must be %q or %q", output.Type, ArtifactTypeContent, ArtifactTypeReference),
				Paths:   []string{fmt.Sprintf("outputs[%d].type", i)},
			})
		}
		if output.Subject {
			errs = errs.Also(&apis.FieldError{
				Message: "subject is not allowed on step-scoped artifacts, only on task-level outputs",
				Paths:   []string{fmt.Sprintf("outputs[%d].subject", i)},
			})
		}
		if output.Value != "" {
			errs = errs.Also(&apis.FieldError{
				Message: "value is not allowed on step-scoped artifacts, only on task-level outputs",
				Paths:   []string{fmt.Sprintf("outputs[%d].value", i)},
			})
		}
	}

	return errs
}

func validatePipelineArtifacts(ctx context.Context, ps *PipelineSpec) *apis.FieldError {
	if ps.Artifacts == nil {
		return nil
	}
	if cfg := config.FromContextOrDefaults(ctx); cfg == nil || !cfg.FeatureFlags.EnableArtifacts {
		return apis.ErrGeneric(fmt.Sprintf("feature flag %s should be set to true to use artifacts feature", config.EnableArtifacts), "")
	}
	var errs *apis.FieldError
	inputNames := map[string]bool{}
	for i, in := range ps.Artifacts.Inputs {
		if in.Name == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("inputs[%d].name", i)))
		}
		if inputNames[in.Name] {
			errs = errs.Also(apis.ErrInvalidValue(in.Name, fmt.Sprintf("inputs[%d].name", i)))
		}
		inputNames[in.Name] = true
	}
	outputNames := map[string]bool{}
	allTasks := append(append([]PipelineTask{}, ps.Tasks...), ps.Finally...)
	for i, out := range ps.Artifacts.Outputs {
		if out.Name == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("outputs[%d].name", i)))
		}
		if outputNames[out.Name] {
			errs = errs.Also(apis.ErrInvalidValue(out.Name, fmt.Sprintf("outputs[%d].name", i)))
		}
		outputNames[out.Name] = true
		parts := strings.Split(out.From, ".")
		if len(parts) != 4 || parts[0] != "tasks" || parts[2] != "outputs" {
			errs = errs.Also(apis.ErrInvalidValue(out.From, fmt.Sprintf("outputs[%d].from", i)))
			continue
		}
		for _, task := range allTasks {
			if task.Name == parts[1] && task.TaskSpec != nil {
				found := false
				for _, a := range task.TaskSpec.Artifacts.Outputs {
					if a.Name == parts[3] {
						found = true
						break
					}
				}
				if !found {
					errs = errs.Also(apis.ErrInvalidValue(out.From, fmt.Sprintf("outputs[%d].from", i)))
				}
			}
		}
	}
	return errs
}

// ValidatePipelineTaskArtifactBindings validates artifact bindings on a PipelineTask.
func ValidatePipelineTaskArtifactBindings(ctx context.Context, bindings *PipelineTaskArtifacts) *apis.FieldError {
	if bindings == nil {
		return nil
	}

	var errs *apis.FieldError
	errs = errs.Also(validateArtifactStorageConfig(ctx))
	for i, binding := range bindings.Inputs {
		if binding.Name == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("inputs[%d].name", i)))
		}
		if binding.From == "" {
			errs = errs.Also(apis.ErrMissingField(fmt.Sprintf("inputs[%d].from", i)))
		} else {
			// Validate format: tasks.<taskName>.outputs.<artifactName>
			parts := strings.Split(binding.From, ".")
			if len(parts) != 4 || parts[0] != PipelineTasks || parts[2] != "outputs" { //nolint:goconst
				errs = errs.Also(&apis.FieldError{
					Message: fmt.Sprintf("invalid from format %q, expected 'tasks.<taskName>.outputs.<artifactName>'", binding.From),
					Paths:   []string{fmt.Sprintf("inputs[%d].from", i)},
				})
			}
		}
	}

	return errs
}
