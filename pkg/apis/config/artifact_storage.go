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

package config

import (
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	// ArtifactStorageConfigName is the name of the ConfigMap for artifact storage configuration.
	ArtifactStorageConfigName = "config-artifact-storage"

	enabledKey              = "enabled"
	backendKey              = "backend"
	inlineThresholdKey      = "inline-threshold"
	ociRepositoryKey        = "oci-repository"
	insecureKey             = "insecure"
	ociCredentialsSecretKey = "oci.credentialsSecret"
	ociAttachReferrersKey   = "oci.attachReferrers"
	ociTagPatternKey        = "oci.tagPattern"
	ociGroupByPipelineRun   = "oci.groupByPipelineRun"

	DefaultBackend         = "oci"
	DefaultInlineThreshold = 512
	DefaultTagPattern      = "{{namespace}}.{{taskrun}}.{{artifact}}"

	// Annotation keys for Pipeline/PipelineRun-level overrides (levels 1 & 2).
	ArtifactStorageEnabledAnnotation               = "tekton.dev/artifact-storage-enabled"
	ArtifactStorageOCIRepositoryAnnotation         = "tekton.dev/artifact-storage-oci-repository"
	ArtifactStorageBackendAnnotation               = "tekton.dev/artifact-storage-backend"
	ArtifactStorageInlineThresholdAnnotation       = "tekton.dev/artifact-storage-inline-threshold"
	ArtifactStorageInsecureAnnotation              = "tekton.dev/artifact-storage-insecure"
	ArtifactStorageOCICredentialsSecretAnnotation  = "tekton.dev/artifact-storage-oci-credentials-secret" //nolint:gosec // annotation key, not a credential
	ArtifactStorageOCIAttachReferrersAnnotation    = "tekton.dev/artifact-storage-oci-attach-referrers"
	ArtifactStorageOCITagPatternAnnotation         = "tekton.dev/artifact-storage-oci-tag-pattern"
	ArtifactStorageOCIGroupByPipelineRunAnnotation = "tekton.dev/artifact-storage-oci-group-by-pipelinerun"

	// MaxInlineThreshold is the upper bound for the inline-threshold
	// config value. CRI runtimes share the 4KB termination message
	// budget with container metadata, leaving ~2KB for the actual
	// message. At 1024 raw bytes the base64 encoding is 1368 bytes
	// which, with JSON framing, fits safely within that limit.
	MaxInlineThreshold = 1024

	// PodInlineBudget is the maximum total raw bytes of inline artifact
	// content across all steps in a Pod.
	PodInlineBudget = 2048

	// InlineBudgetFile is the well-known path where the entrypoint
	// persists cumulative inline consumption across steps. This must
	// be on a volume shared and writable by all step containers;
	// /tekton/artifacts is an emptyDir mounted in every step.
	InlineBudgetFile = "/tekton/artifacts/.inline-budget"
)

// ArtifactStorage holds configuration for OCI-based artifact storage.
type ArtifactStorage struct {
	// Enabled controls whether content artifact storage is active.
	Enabled bool
	// Backend selects the storage backend type (e.g., "oci").
	Backend string
	// InlineThreshold is the size in bytes below which artifacts are stored
	// inline in TaskRun status rather than uploaded to the backend.
	InlineThreshold int
	// OCIRepository is the base OCI repository for artifact storage
	// e.g., "ghcr.io/org/project/artifacts" or "registry:5000/artifacts"
	OCIRepository string
	// Insecure allows plain HTTP and skips TLS certificate verification
	// for the OCI registry.
	Insecure bool
	// OCICredentialsSecret is the name of a kubernetes.io/dockerconfigjson Secret
	// for OCI registry authentication. If empty, ServiceAccount imagePullSecrets are used.
	OCICredentialsSecret string
	// OCIAttachReferrers controls whether output artifacts are attached as
	// OCI referrers to subject artifacts after a PipelineRun completes.
	OCIAttachReferrers bool
	// OCITagPattern is the tag pattern for artifact manifests.
	// Variables: {{namespace}}, {{pipelinerun}}, {{taskrun}}, {{artifact}}
	OCITagPattern string
	// OCIGroupByPipelineRun creates a root OCI Index per PipelineRun and
	// attaches all artifacts as referrers to it.
	OCIGroupByPipelineRun bool
}

// ReferrersEnabled reports whether OCI referrer attachment should run.
func (as *ArtifactStorage) ReferrersEnabled() bool {
	return as != nil && as.Enabled && as.OCIAttachReferrers && as.OCIRepository != ""
}

// NewArtifactStorageFromMap creates an ArtifactStorage from a map of string values.
func NewArtifactStorageFromMap(cfgMap map[string]string) (*ArtifactStorage, error) {
	as := &ArtifactStorage{
		Backend:            DefaultBackend,
		InlineThreshold:    DefaultInlineThreshold,
		OCIAttachReferrers: true,
		OCITagPattern:      DefaultTagPattern,
	}

	if v, ok := cfgMap[enabledKey]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing artifact storage config %q: %w for key %s", v, err, enabledKey)
		}
		as.Enabled = b
	}

	if v, ok := cfgMap[backendKey]; ok && v != "" {
		as.Backend = v
	}

	if v, ok := cfgMap[inlineThresholdKey]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing artifact storage config %q: %w for key %s", v, err, inlineThresholdKey)
		}
		if n < 0 {
			return nil, fmt.Errorf("invalid artifact storage config: %s must be non-negative, got %d", inlineThresholdKey, n)
		}
		if n > MaxInlineThreshold {
			return nil, fmt.Errorf("invalid artifact storage config: %s must not exceed %d, got %d", inlineThresholdKey, MaxInlineThreshold, n)
		}
		as.InlineThreshold = n
	}

	if v, ok := cfgMap[ociRepositoryKey]; ok {
		as.OCIRepository = v
	}

	if v, ok := cfgMap[insecureKey]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing artifact storage config %q: %w for key %s", v, err, insecureKey)
		}
		as.Insecure = b
	}

	if v, ok := cfgMap[ociCredentialsSecretKey]; ok {
		as.OCICredentialsSecret = v
	}

	if v, ok := cfgMap[ociAttachReferrersKey]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing artifact storage config %q: %w for key %s", v, err, ociAttachReferrersKey)
		}
		as.OCIAttachReferrers = b
	}

	if v, ok := cfgMap[ociTagPatternKey]; ok && v != "" {
		as.OCITagPattern = v
	}

	if v, ok := cfgMap[ociGroupByPipelineRun]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("failed parsing artifact storage config %q: %w for key %s", v, err, ociGroupByPipelineRun)
		}
		as.OCIGroupByPipelineRun = b
	}

	return as, nil
}

// NewArtifactStorageFromConfigMap creates an ArtifactStorage from a ConfigMap.
func NewArtifactStorageFromConfigMap(config *corev1.ConfigMap) (*ArtifactStorage, error) {
	return NewArtifactStorageFromMap(config.Data)
}

// GetArtifactStorageConfigName returns the name of the ConfigMap for artifact storage.
func GetArtifactStorageConfigName() string {
	if e := os.Getenv("CONFIG_ARTIFACT_STORAGE_NAME"); e != "" {
		return e
	}
	return ArtifactStorageConfigName
}

// DeepCopy returns an independent copy of the ArtifactStorage.
func (as *ArtifactStorage) DeepCopy() *ArtifactStorage {
	if as == nil {
		return nil
	}
	out := *as
	return &out
}

// ApplyOverridesFromMap applies only the fields present in cfgMap onto base,
// leaving other fields untouched.
func ApplyOverridesFromMap(base *ArtifactStorage, cfgMap map[string]string) error {
	if base == nil || len(cfgMap) == 0 {
		return nil
	}

	if v, ok := cfgMap[enabledKey]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing artifact storage override %q: %w for key %s", v, err, enabledKey)
		}
		base.Enabled = b
	}

	if v, ok := cfgMap[backendKey]; ok && v != "" {
		base.Backend = v
	}

	if v, ok := cfgMap[inlineThresholdKey]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("failed parsing artifact storage override %q: %w for key %s", v, err, inlineThresholdKey)
		}
		if n < 0 {
			return fmt.Errorf("invalid artifact storage override: %s must be non-negative, got %d", inlineThresholdKey, n)
		}
		if n > MaxInlineThreshold {
			return fmt.Errorf("invalid artifact storage override: %s must not exceed %d, got %d", inlineThresholdKey, MaxInlineThreshold, n)
		}
		base.InlineThreshold = n
	}

	if v, ok := cfgMap[ociRepositoryKey]; ok {
		base.OCIRepository = v
	}

	if v, ok := cfgMap[insecureKey]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing artifact storage override %q: %w for key %s", v, err, insecureKey)
		}
		base.Insecure = b
	}

	if v, ok := cfgMap[ociCredentialsSecretKey]; ok {
		base.OCICredentialsSecret = v
	}

	if v, ok := cfgMap[ociAttachReferrersKey]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing artifact storage override %q: %w for key %s", v, err, ociAttachReferrersKey)
		}
		base.OCIAttachReferrers = b
	}

	if v, ok := cfgMap[ociTagPatternKey]; ok && v != "" {
		base.OCITagPattern = v
	}

	if v, ok := cfgMap[ociGroupByPipelineRun]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing artifact storage override %q: %w for key %s", v, err, ociGroupByPipelineRun)
		}
		base.OCIGroupByPipelineRun = b
	}

	return nil
}

// applyAnnotationOverrides reads artifact storage annotations and applies
// them as overrides onto base. This implements levels 1 and 2 (Pipeline/PipelineRun annotations) of the
// configuration hierarchy.
func applyAnnotationOverrides(base *ArtifactStorage, annotations map[string]string) error {
	if base == nil || len(annotations) == 0 {
		return nil
	}

	if v, ok := annotations[ArtifactStorageEnabledAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing annotation %s value %q: %w", ArtifactStorageEnabledAnnotation, v, err)
		}
		base.Enabled = b
	}

	if v, ok := annotations[ArtifactStorageOCIRepositoryAnnotation]; ok {
		base.OCIRepository = v
	}

	if v, ok := annotations[ArtifactStorageBackendAnnotation]; ok && v != "" {
		base.Backend = v
	}

	if v, ok := annotations[ArtifactStorageInlineThresholdAnnotation]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("failed parsing annotation %s value %q: %w", ArtifactStorageInlineThresholdAnnotation, v, err)
		}
		if n < 0 || n > MaxInlineThreshold {
			return fmt.Errorf("invalid annotation %s: must be 0..%d, got %d", ArtifactStorageInlineThresholdAnnotation, MaxInlineThreshold, n)
		}
		base.InlineThreshold = n
	}

	if v, ok := annotations[ArtifactStorageInsecureAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing annotation %s value %q: %w", ArtifactStorageInsecureAnnotation, v, err)
		}
		base.Insecure = b
	}

	if v, ok := annotations[ArtifactStorageOCICredentialsSecretAnnotation]; ok {
		base.OCICredentialsSecret = v
	}

	if v, ok := annotations[ArtifactStorageOCIAttachReferrersAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing annotation %s value %q: %w", ArtifactStorageOCIAttachReferrersAnnotation, v, err)
		}
		base.OCIAttachReferrers = b
	}

	if v, ok := annotations[ArtifactStorageOCITagPatternAnnotation]; ok && v != "" {
		base.OCITagPattern = v
	}

	if v, ok := annotations[ArtifactStorageOCIGroupByPipelineRunAnnotation]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("failed parsing annotation %s value %q: %w", ArtifactStorageOCIGroupByPipelineRunAnnotation, v, err)
		}
		base.OCIGroupByPipelineRun = b
	}

	return nil
}

// ResolveArtifactStorage resolves the artifact storage configuration by
// merging all four hierarchy levels (highest to lowest priority):
//
//	1+2. annotations (PipelineRun/Pipeline, already merged by caller)
//	3.   namespace ConfigMap overrides (nil if not found)
//	4.   cluster ConfigMap (base, from context)
//
// The base is deep-copied before mutation so the caller's original is
// not modified.
func ResolveArtifactStorage(base *ArtifactStorage, namespaceOverrides map[string]string, annotations map[string]string) (*ArtifactStorage, error) {
	resolved := base.DeepCopy()
	if resolved == nil {
		return nil, nil //nolint:nilnil // nil base means artifact storage is not configured
	}

	if len(namespaceOverrides) > 0 {
		if err := ApplyOverridesFromMap(resolved, namespaceOverrides); err != nil {
			return nil, fmt.Errorf("invalid namespace artifact storage config: %w", err)
		}
	}

	if err := applyAnnotationOverrides(resolved, annotations); err != nil {
		return nil, fmt.Errorf("invalid artifact storage annotation: %w", err)
	}

	return resolved, nil
}
