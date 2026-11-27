/*
Copyright 2024 The Tekton Authors

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
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

// artifactsEnabledCtx returns a context with enable-artifacts set to true.
func artifactsEnabledCtx() context.Context {
	return contextWithArtifactsEnabled(true)
}

// contextWithArtifactsEnabled returns a context with the given enable-artifacts value.
func contextWithArtifactsEnabled(enabled bool) context.Context {
	cfg := &config.Config{
		FeatureFlags: &config.FeatureFlags{
			EnableArtifacts: enabled,
		},
		ArtifactStorage: &config.ArtifactStorage{},
	}
	return config.ToContext(context.Background(), cfg)
}

// contextWithArtifactStorage returns a context with enable-artifacts true and the given storage config.
func contextWithArtifactStorage(as *config.ArtifactStorage) context.Context {
	cfg := &config.Config{
		FeatureFlags: &config.FeatureFlags{
			EnableArtifacts: true,
		},
		ArtifactStorage: as,
	}
	return config.ToContext(context.Background(), cfg)
}

func TestValidateArtifactDeclarations_FeatureFlagDisabled(t *testing.T) {
	ctx := contextWithArtifactsEnabled(false)
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference, Subject: true},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error when enable-artifacts is false")
	}
	want := fmt.Sprintf("feature flag %s should be set to true to use artifacts feature: ", config.EnableArtifacts)
	if errs.Error() != want {
		t.Errorf("expected error message %q, got %q", want, errs.Error())
	}
}

func TestValidateArtifactDeclarations_Valid(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source"},
			{Name: "config", MediaType: "application/json"},
		},
		Outputs: []ArtifactDeclaration{
			{Name: "image", MediaType: "application/vnd.oci.image.manifest.v1+json", Subject: true},
			{Name: "sbom", MediaType: "application/vnd.cyclonedx+json"},
		},
	}

	if errs := ValidateArtifactDeclarations(ctx, decls); errs != nil {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidateArtifactDeclarations_DuplicateInputNames(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source"},
			{Name: "source"},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for duplicate input names")
	}
}

func TestValidateArtifactDeclarations_DuplicateOutputNames(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image"},
			{Name: "image"},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for duplicate output names")
	}
}

func TestValidateArtifactDeclarations_EmptyName(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: ""},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for empty artifact name")
	}
}

func TestValidateArtifactDeclarations_PathTraversalNames(t *testing.T) {
	ctx := artifactsEnabledCtx()
	tests := []struct {
		name         string
		artifactName string
		wantErr      bool
	}{
		{"path traversal with parent dir", "../../etc/shadow", true},
		{"dotdot only", "..", true},
		{"single dot", ".", true},
		{"slash in name", "foo/bar", true},
		{"backslash in name", "foo\\bar", true},
		{"dotdot prefix", "..foo", true},
		{"ssh key path", "../.ssh/id_rsa", true},
		{"starts with hyphen", "-image", true},
		{"starts with dot", ".hidden", true},
		{"valid simple name", "image", false},
		{"valid with hyphen", "my-image", false},
		{"valid with underscore", "my_image", false},
		{"valid with dot", "image.tar", false},
		{"valid alphanumeric", "sbom2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decls := &ArtifactDeclarations{
				Inputs: []ArtifactDeclaration{
					{Name: tt.artifactName},
				},
			}
			errs := ValidateArtifactDeclarations(ctx, decls)
			if (errs != nil) != tt.wantErr {
				t.Errorf("ValidateArtifactDeclarations() with name %q: errors = %v, wantErr %v", tt.artifactName, errs, tt.wantErr)
			}
		})
	}
}

func TestValidateArtifactDeclarations_MultipleSubjectsAllowed(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image1", Subject: true},
			{Name: "image2", Subject: true},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs != nil {
		t.Errorf("expected no error for multiple subjects, got: %v", errs)
	}
}

func TestValidateArtifactDeclarations_InvalidType(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: "bogus"},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for invalid artifact type")
	}
}

func TestValidateArtifactDeclarations_InvalidInputType(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source", Type: "bogus"},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for invalid input artifact type")
	}
}

func TestValidateArtifactDeclarations_ValidInputTypes(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source", Type: ArtifactTypeContent},
			{Name: "image-ref", Type: ArtifactTypeReference},
		},
	}

	if errs := ValidateArtifactDeclarations(ctx, decls); errs != nil {
		t.Errorf("expected no errors for valid input types, got: %v", errs)
	}
}

func TestValidateArtifactDeclarations_SubjectOnInput(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source", Subject: true},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for subject on input")
	}
}

func TestValidateArtifactDeclarations_ValueOnInput(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Inputs: []ArtifactDeclaration{
			{Name: "source", Value: "$(steps.clone.artifacts.source)"},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Error("expected error for value on input")
	}
}

func TestValidateArtifactDeclarations_ValidOutputValue(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Value: "$(steps.build.artifacts.image)", Subject: true},
		},
	}

	if errs := ValidateArtifactDeclarations(ctx, decls); errs != nil {
		t.Errorf("expected no errors for valid output value, got: %v", errs)
	}
}

func TestValidateArtifactDeclarations_InvalidOutputValueFormat(t *testing.T) {
	ctx := artifactsEnabledCtx()
	tests := []struct {
		name  string
		value string
	}{
		{"missing dollar-paren", "steps.build.artifacts.image"},
		{"wrong prefix", "$(tasks.build.artifacts.image)"},
		{"missing artifacts segment", "$(steps.build.image)"},
		{"extra segments", "$(steps.build.artifacts.image.uri)"},
		{"empty", "$(steps..artifacts.image)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decls := &ArtifactDeclarations{
				Outputs: []ArtifactDeclaration{
					{Name: "image", Value: tt.value},
				},
			}
			errs := ValidateArtifactDeclarations(ctx, decls)
			if errs == nil {
				t.Errorf("expected error for invalid value format %q", tt.value)
			}
		})
	}
}

func TestValidateArtifactDeclarations_TypeWithValueAllowed(t *testing.T) {
	ctx := artifactsEnabledCtx()
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeContent, Value: "$(steps.build.artifacts.image)"},
		},
	}

	if errs := ValidateArtifactDeclarations(ctx, decls); errs != nil {
		t.Errorf("expected no error when type is set alongside value (type is defaulted), got: %v", errs)
	}
}

func TestValidateArtifactDeclarations_Nil(t *testing.T) {
	ctx := artifactsEnabledCtx()
	if errs := ValidateArtifactDeclarations(ctx, nil); errs != nil {
		t.Errorf("expected no errors for nil, got: %v", errs)
	}
}

func TestValidateArtifactDeclarations_OCIRepositoryWithoutEnabled(t *testing.T) {
	ctx := contextWithArtifactStorage(&config.ArtifactStorage{
		Enabled:       false,
		OCIRepository: "registry:5000/artifacts",
	})
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference},
		},
	}

	errs := ValidateArtifactDeclarations(ctx, decls)
	if errs == nil {
		t.Fatal("expected error when oci-repository is set but enabled is false")
	}
	want := `artifact storage config is invalid: oci-repository is set to "registry:5000/artifacts" but enabled is false; set enabled to true to activate artifact storage: `
	if errs.Error() != want {
		t.Errorf("expected error message %q, got %q", want, errs.Error())
	}
}

func TestValidateArtifactDeclarations_OCIRepositoryWithEnabled(t *testing.T) {
	ctx := contextWithArtifactStorage(&config.ArtifactStorage{
		Enabled:       true,
		OCIRepository: "registry:5000/artifacts",
	})
	decls := &ArtifactDeclarations{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference},
		},
	}

	if errs := ValidateArtifactDeclarations(ctx, decls); errs != nil {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidatePipelineTaskArtifactBindings_Valid(t *testing.T) {
	bindings := &PipelineTaskArtifacts{
		Inputs: []PipelineTaskArtifactBinding{
			{Name: "source", From: "tasks.build.outputs.image"},
		},
	}

	if errs := ValidatePipelineTaskArtifactBindings(artifactsEnabledCtx(), bindings); errs != nil {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidatePipelineTaskArtifactBindings_OCIRepositoryWithoutEnabled(t *testing.T) {
	ctx := contextWithArtifactStorage(&config.ArtifactStorage{
		Enabled:       false,
		OCIRepository: "registry:5000/artifacts",
	})
	bindings := &PipelineTaskArtifacts{
		Inputs: []PipelineTaskArtifactBinding{
			{Name: "source", From: "tasks.build.outputs.image"},
		},
	}

	errs := ValidatePipelineTaskArtifactBindings(ctx, bindings)
	if errs == nil {
		t.Fatal("expected error when oci-repository is set but enabled is false")
	}
}

func TestValidatePipelineTaskArtifactBindings_PipelineInputFormat(t *testing.T) {
	bindings := &PipelineTaskArtifacts{
		Inputs: []PipelineTaskArtifactBinding{
			{Name: "source", From: "artifacts.inputs.source"},
		},
	}

	if errs := ValidatePipelineTaskArtifactBindings(artifactsEnabledCtx(), bindings); errs != nil {
		t.Errorf("expected no errors for artifacts.inputs.<name> format, got: %v", errs)
	}
}

func TestValidatePipelineTaskArtifactBindings_InvalidFromFormat(t *testing.T) {
	bindings := &PipelineTaskArtifacts{
		Inputs: []PipelineTaskArtifactBinding{
			{Name: "source", From: "invalid-format"},
		},
	}

	errs := ValidatePipelineTaskArtifactBindings(artifactsEnabledCtx(), bindings)
	if errs == nil {
		t.Error("expected error for invalid from format")
	}
}

func TestValidatePipelineTaskArtifactBindings_EmptyName(t *testing.T) {
	bindings := &PipelineTaskArtifacts{
		Inputs: []PipelineTaskArtifactBinding{
			{Name: "", From: "tasks.build.outputs.image"},
		},
	}

	errs := ValidatePipelineTaskArtifactBindings(artifactsEnabledCtx(), bindings)
	if errs == nil {
		t.Error("expected error for empty binding name")
	}
}

func TestValidatePipelineTaskArtifactBindings_EmptyFrom(t *testing.T) {
	bindings := &PipelineTaskArtifacts{
		Inputs: []PipelineTaskArtifactBinding{
			{Name: "source", From: ""},
		},
	}

	errs := ValidatePipelineTaskArtifactBindings(artifactsEnabledCtx(), bindings)
	if errs == nil {
		t.Error("expected error for empty from")
	}
}

func TestValidateStepArtifacts_Nil(t *testing.T) {
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), nil)
	if errs != nil {
		t.Errorf("expected no error for nil, got: %v", errs)
	}
}

func TestValidateStepArtifacts_Valid(t *testing.T) {
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference},
			{Name: "logs", Type: ArtifactTypeContent},
		},
	}
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), arts)
	if errs != nil {
		t.Errorf("expected no error, got: %v", errs)
	}
}

func TestValidateStepArtifacts_OCIRepositoryWithoutEnabled(t *testing.T) {
	ctx := contextWithArtifactStorage(&config.ArtifactStorage{
		Enabled:       false,
		OCIRepository: "registry:5000/artifacts",
	})
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "logs", Type: ArtifactTypeContent},
		},
	}

	errs := ValidateStepArtifacts(ctx, arts)
	if errs == nil {
		t.Fatal("expected error when oci-repository is set but enabled is false")
	}
}

func TestValidateStepArtifacts_FeatureFlagDisabled(t *testing.T) {
	ctx := contextWithArtifactsEnabled(false)
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference},
		},
	}
	errs := ValidateStepArtifacts(ctx, arts)
	if errs == nil {
		t.Error("expected error when feature flag disabled")
	}
}

func TestValidateStepArtifacts_DuplicateNames(t *testing.T) {
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference},
			{Name: "image", Type: ArtifactTypeContent},
		},
	}
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), arts)
	if errs == nil {
		t.Error("expected error for duplicate names")
	}
}

func TestValidateStepArtifacts_InvalidType(t *testing.T) {
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: "invalid"},
		},
	}
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), arts)
	if errs == nil {
		t.Error("expected error for invalid type")
	}
}

func TestValidateStepArtifacts_EmptyName(t *testing.T) {
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "", Type: ArtifactTypeContent},
		},
	}
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), arts)
	if errs == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidateStepArtifacts_SubjectRejected(t *testing.T) {
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference, Subject: true},
		},
	}
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), arts)
	if errs == nil {
		t.Error("expected error: subject not allowed on step-scoped artifacts")
	}
}

func TestValidateStepArtifacts_ValueRejected(t *testing.T) {
	arts := &StepArtifacts{
		Outputs: []ArtifactDeclaration{
			{Name: "image", Type: ArtifactTypeReference, Value: "$(steps.build.artifacts.image)"},
		},
	}
	errs := ValidateStepArtifacts(artifactsEnabledCtx(), arts)
	if errs == nil {
		t.Error("expected error: value not allowed on step-scoped artifacts")
	}
}

func TestValidateArtifactStorageAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		wantErr     bool
		errContains string
		errCount    int
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			wantErr:     false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			wantErr:     false,
		},
		{
			name: "valid inline-threshold",
			annotations: map[string]string{
				config.ArtifactStorageInlineThresholdAnnotation: "256",
			},
			wantErr: false,
		},
		{
			name: "invalid inline-threshold not a number",
			annotations: map[string]string{
				config.ArtifactStorageInlineThresholdAnnotation: "abc",
			},
			wantErr:     true,
			errContains: config.ArtifactStorageInlineThresholdAnnotation,
		},
		{
			name: "inline-threshold exceeds max",
			annotations: map[string]string{
				config.ArtifactStorageInlineThresholdAnnotation: "9999",
			},
			wantErr:     true,
			errContains: config.ArtifactStorageInlineThresholdAnnotation,
		},
		{
			name: "inline-threshold negative",
			annotations: map[string]string{
				config.ArtifactStorageInlineThresholdAnnotation: "-1",
			},
			wantErr:     true,
			errContains: config.ArtifactStorageInlineThresholdAnnotation,
		},
		{
			name: "valid oci-repository",
			annotations: map[string]string{
				config.ArtifactStorageOCIRepositoryAnnotation: "ghcr.io/test",
			},
			wantErr: false,
		},
		{
			name: "valid oci-repository empty",
			annotations: map[string]string{
				config.ArtifactStorageOCIRepositoryAnnotation: "",
			},
			wantErr:     true,
			errContains: config.ArtifactStorageOCIRepositoryAnnotation,
		},
		{
			name: "multiple valid annotations",
			annotations: map[string]string{
				config.ArtifactStorageInlineThresholdAnnotation: "128",
				config.ArtifactStorageOCIRepositoryAnnotation:   "ghcr.io/test",
			},
			wantErr: false,
		},
		{
			name: "unrelated annotations ignored",
			annotations: map[string]string{
				"some-other-annotation": "value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateArtifactStorageAnnotations(tt.annotations)
			if tt.wantErr {
				if errs == nil {
					t.Fatal("expected error but got nil")
				}
				if tt.errContains != "" && !strings.Contains(errs.Error(), tt.errContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errContains, errs.Error())
				}
				if tt.errCount > 0 {
					errStr := errs.Error()
					count := 0
					for _, line := range strings.Split(errStr, "\n") {
						if strings.TrimSpace(line) != "" {
							count++
						}
					}
					if count < tt.errCount {
						t.Errorf("expected at least %d errors, got %d: %s", tt.errCount, count, errStr)
					}
				}
			} else if errs != nil {
				t.Errorf("expected no error, got: %v", errs)
			}
		})
	}
}

func TestValidateArtifactStorageConfig(t *testing.T) {
	tests := []struct {
		name    string
		storage *config.ArtifactStorage
		wantErr bool
	}{
		{
			name:    "nil storage",
			storage: nil,
			wantErr: false,
		},
		{
			name: "oci-repository set and enabled true",
			storage: &config.ArtifactStorage{
				Enabled:       true,
				OCIRepository: "registry:5000/artifacts",
			},
			wantErr: false,
		},
		{
			name: "oci-repository set but enabled false",
			storage: &config.ArtifactStorage{
				Enabled:       false,
				OCIRepository: "registry:5000/artifacts",
			},
			wantErr: true,
		},
		{
			name: "oci-repository empty and enabled false",
			storage: &config.ArtifactStorage{
				Enabled:       false,
				OCIRepository: "",
			},
			wantErr: false,
		},
		{
			name: "oci-repository empty and enabled true",
			storage: &config.ArtifactStorage{
				Enabled:       true,
				OCIRepository: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.storage != nil {
				ctx = contextWithArtifactStorage(tt.storage)
			} else {
				cfg := &config.Config{
					FeatureFlags:    &config.FeatureFlags{},
					ArtifactStorage: nil,
				}
				ctx = config.ToContext(context.Background(), cfg)
			}
			errs := validateArtifactStorageConfig(ctx)
			if tt.wantErr && errs == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantErr && errs != nil {
				t.Errorf("expected no error, got: %v", errs)
			}
		})
	}
}

func TestResolveArtifactStorageForValidation(t *testing.T) {
	clusterConfig := &config.ArtifactStorage{
		Enabled:         false,
		Backend:         config.DefaultBackend,
		InlineThreshold: config.DefaultInlineThreshold,
		OCIRepository:   "cluster-repo",
		OCITagPattern:   config.DefaultTagPattern,
	}

	tests := []struct {
		name        string
		storage     *config.ArtifactStorage
		namespace   string
		annotations map[string]string
		nsCMData    map[string]string
		useKube     bool
		wantRepo    string
		wantEnabled bool
	}{
		{
			name:        "no kubeclient no annotations returns cluster config",
			storage:     clusterConfig,
			namespace:   "test-ns",
			annotations: nil,
			useKube:     false,
			wantRepo:    "cluster-repo",
			wantEnabled: false,
		},
		{
			name:    "no kubeclient annotation overrides oci-repository",
			storage: clusterConfig,
			annotations: map[string]string{
				config.ArtifactStorageOCIRepositoryAnnotation: "annotation-repo",
			},
			useKube:     false,
			wantRepo:    "annotation-repo",
			wantEnabled: false,
		},
		{
			name:    "no kubeclient unknown annotation is ignored",
			storage: clusterConfig,
			annotations: map[string]string{
				"tekton.dev/something-else": "true",
			},
			useKube:     false,
			wantRepo:    "cluster-repo",
			wantEnabled: false,
		},
		{
			name:        "nil ArtifactStorage returns context unchanged",
			storage:     nil,
			namespace:   "test-ns",
			annotations: nil,
			useKube:     false,
			wantRepo:    "",
			wantEnabled: false,
		},
		{
			name:      "kubeclient with namespace configmap overrides cluster",
			storage:   clusterConfig,
			namespace: "test-ns",
			nsCMData: map[string]string{
				"oci-repository": "ns-repo",
				"enabled":        "true",
			},
			useKube:     true,
			wantRepo:    "ns-repo",
			wantEnabled: true,
		},
		{
			name:        "kubeclient namespace configmap not found falls through",
			storage:     clusterConfig,
			namespace:   "test-ns",
			nsCMData:    nil,
			useKube:     true,
			wantRepo:    "cluster-repo",
			wantEnabled: false,
		},
		{
			name:      "kubeclient namespace configmap not found but annotation overrides",
			storage:   clusterConfig,
			namespace: "test-ns",
			nsCMData:  nil,
			annotations: map[string]string{
				config.ArtifactStorageOCIRepositoryAnnotation: "annotation-repo",
			},
			useKube:     true,
			wantRepo:    "annotation-repo",
			wantEnabled: false,
		},
		{
			name:      "annotation oci-repository wins over namespace configmap",
			storage:   clusterConfig,
			namespace: "test-ns",
			nsCMData: map[string]string{
				"oci-repository": "ns-repo",
				"enabled":        "true",
			},
			annotations: map[string]string{
				config.ArtifactStorageOCIRepositoryAnnotation: "annotation-repo",
			},
			useKube:     true,
			wantRepo:    "annotation-repo",
			wantEnabled: true,
		},
		{
			name:      "namespace overrides only some fields",
			storage:   clusterConfig,
			namespace: "test-ns",
			nsCMData: map[string]string{
				"enabled": "true",
			},
			useKube:     true,
			wantRepo:    "cluster-repo",
			wantEnabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableArtifacts: true,
				},
				ArtifactStorage: tt.storage,
			}
			ctx := config.ToContext(context.Background(), cfg)

			if tt.useKube {
				if tt.nsCMData != nil {
					cm := &corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      config.ArtifactStorageConfigName,
							Namespace: tt.namespace,
						},
						Data: tt.nsCMData,
					}
					fakeClient := fake.NewSimpleClientset(cm)
					ctx = context.WithValue(ctx, kubeclient.Key{}, fakeClient)
				} else {
					fakeClient := fake.NewSimpleClientset()
					ctx = context.WithValue(ctx, kubeclient.Key{}, fakeClient)
				}
			}

			resultCtx := resolveArtifactStorageForValidation(ctx, tt.namespace, tt.annotations)
			resolved := config.FromContextOrDefaults(resultCtx).ArtifactStorage

			if tt.storage == nil {
				if resolved != nil && resolved.OCIRepository != "" {
					t.Errorf("expected nil/default storage for nil input, got repo=%q", resolved.OCIRepository)
				}
				return
			}

			if resolved.OCIRepository != tt.wantRepo {
				t.Errorf("OCIRepository = %q, want %q", resolved.OCIRepository, tt.wantRepo)
			}
			if resolved.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", resolved.Enabled, tt.wantEnabled)
			}
		})
	}
}

func TestValidatePipelineArtifacts_CrossValidation(t *testing.T) {
	tests := []struct {
		name    string
		arts    *PipelineArtifactDeclarations
		tasks   []PipelineTask
		finally []PipelineTask
		wantErr bool
	}{
		{
			name: "valid: task references declared pipeline input",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source"}},
			},
			tasks: []PipelineTask{
				{
					Name: "build",
					Artifacts: &PipelineTaskArtifacts{
						Inputs: []PipelineTaskArtifactBinding{
							{Name: "src", From: "artifacts.inputs.source"},
						},
					},
				},
			},
		},
		{
			name: "error: task references undeclared pipeline input",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source"}},
			},
			tasks: []PipelineTask{
				{
					Name: "build",
					Artifacts: &PipelineTaskArtifacts{
						Inputs: []PipelineTaskArtifactBinding{
							{Name: "src", From: "artifacts.inputs.missing"},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: subject not allowed on pipeline inputs",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source", Subject: true}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: value not allowed on pipeline inputs",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source", Value: "$(steps.build.artifacts.img)"}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: invalid type on pipeline input",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source", Type: "invalid"}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "valid: content type on pipeline input",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source", Type: ArtifactTypeContent}},
			},
			tasks: []PipelineTask{{Name: "build"}},
		},
		{
			name: "valid: task uses task output, not pipeline input",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{
					{Name: "image", From: "tasks.build.outputs.image"},
				},
			},
			tasks: []PipelineTask{
				{Name: "build"},
				{
					Name: "sign",
					Artifacts: &PipelineTaskArtifacts{
						Inputs: []PipelineTaskArtifactBinding{
							{Name: "img", From: "tasks.build.outputs.image"},
						},
					},
				},
			},
		},
		{
			name:    "error: empty input name",
			arts:    &PipelineArtifactDeclarations{Inputs: []ArtifactDeclaration{{Name: ""}}},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: duplicate input names",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source"}, {Name: "source"}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: empty output name",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{{Name: "", From: "tasks.build.outputs.image"}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: duplicate output names",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{
					{Name: "image", From: "tasks.build.outputs.image"},
					{Name: "image", From: "tasks.build.outputs.logs"},
				},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: empty output from",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{{Name: "image", From: ""}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: invalid output from format",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{{Name: "image", From: "invalid.ref"}},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: output references nonexistent task",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{
					{Name: "image", From: "tasks.nonexistent.outputs.image"},
				},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			wantErr: true,
		},
		{
			name: "error: output references nonexistent finally task",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{
					{Name: "image", From: "finally.nonexistent.outputs.image"},
				},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			finally: []PipelineTask{{Name: "cleanup"}},
			wantErr: true,
		},
		{
			name: "valid: output references existing finally task",
			arts: &PipelineArtifactDeclarations{
				Outputs: []PipelineArtifactOutput{
					{Name: "image", From: "finally.cleanup.outputs.image"},
				},
			},
			tasks:   []PipelineTask{{Name: "build"}},
			finally: []PipelineTask{{Name: "cleanup"}},
		},
		{
			name: "valid: finally task references declared pipeline input",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source"}},
			},
			tasks: []PipelineTask{{Name: "build"}},
			finally: []PipelineTask{
				{
					Name: "cleanup",
					Artifacts: &PipelineTaskArtifacts{
						Inputs: []PipelineTaskArtifactBinding{
							{Name: "src", From: "artifacts.inputs.source"},
						},
					},
				},
			},
		},
		{
			name: "error: finally task references undeclared pipeline input",
			arts: &PipelineArtifactDeclarations{
				Inputs: []ArtifactDeclaration{{Name: "source"}},
			},
			tasks: []PipelineTask{{Name: "build"}},
			finally: []PipelineTask{
				{
					Name: "cleanup",
					Artifacts: &PipelineTaskArtifacts{
						Inputs: []PipelineTaskArtifactBinding{
							{Name: "src", From: "artifacts.inputs.missing"},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validatePipelineArtifacts(artifactsEnabledCtx(), tt.arts, tt.tasks, tt.finally)
			if (errs != nil) != tt.wantErr {
				t.Errorf("validatePipelineArtifacts() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}

func TestValidatePipelineRunArtifacts(t *testing.T) {
	tests := []struct {
		name    string
		spec    *PipelineRunSpec
		wantErr bool
	}{
		{
			name: "valid: nil artifacts",
			spec: &PipelineRunSpec{},
		},
		{
			name: "valid: input with uri and digest",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{
							Name: "source",
							Values: []ArtifactValue{
								{
									Uri:    "oci://registry:5000/artifacts/source@sha256:abc123",
									Digest: map[Algorithm]string{"sha256": "abc123"},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "error: empty input name",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{
							Name: "",
							Values: []ArtifactValue{
								{
									Uri:    "oci://registry:5000/artifacts/source@sha256:abc123",
									Digest: map[Algorithm]string{"sha256": "abc123"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: duplicate input names",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{
							Name: "source",
							Values: []ArtifactValue{
								{
									Uri:    "oci://registry:5000/artifacts/source@sha256:abc123",
									Digest: map[Algorithm]string{"sha256": "abc123"},
								},
							},
						},
						{
							Name: "source",
							Values: []ArtifactValue{
								{
									Uri:    "oci://registry:5000/artifacts/other@sha256:def456",
									Digest: map[Algorithm]string{"sha256": "def456"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: subject not allowed on inputs",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{
							Name:    "source",
							Subject: true,
							Values: []ArtifactValue{
								{
									Uri:    "oci://registry:5000/artifacts/source@sha256:abc123",
									Digest: map[Algorithm]string{"sha256": "abc123"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: empty values",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{Name: "source", Values: []ArtifactValue{}},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: missing uri",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{
							Name: "source",
							Values: []ArtifactValue{
								{Digest: map[Algorithm]string{"sha256": "abc123"}},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: missing digest",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Inputs: []Artifact{
						{
							Name: "source",
							Values: []ArtifactValue{
								{Uri: "oci://registry:5000/artifacts/source@sha256:abc123"},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error: outputs not allowed",
			spec: &PipelineRunSpec{
				Artifacts: &PipelineRunArtifacts{
					Outputs: []Artifact{
						{Name: "image"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.spec.validatePipelineRunArtifacts(artifactsEnabledCtx())
			if (errs != nil) != tt.wantErr {
				t.Errorf("validatePipelineRunArtifacts() errors = %v, wantErr %v", errs, tt.wantErr)
			}
		})
	}
}
