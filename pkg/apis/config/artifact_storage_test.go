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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewArtifactStorageFromMap(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]string
		want    *ArtifactStorage
		wantErr bool
	}{
		{
			name: "all fields set",
			data: map[string]string{
				"enabled":                "true",
				"backend":                "oci",
				"inline-threshold":       "512",
				"oci-repository":         "ghcr.io/org/project/artifacts",
				"insecure":               "true",
				"oci.credentialsSecret":  "my-secret",
				"oci.attachReferrers":    "false",
				"oci.tagPattern":         "{{namespace}}-{{taskrun}}",
				"oci.groupByPipelineRun": "true",
			},
			want: &ArtifactStorage{
				Enabled:               true,
				Backend:               "oci",
				InlineThreshold:       512,
				OCIRepository:         "ghcr.io/org/project/artifacts",
				Insecure:              true,
				OCICredentialsSecret:  "my-secret",
				OCIAttachReferrers:    false,
				OCITagPattern:         "{{namespace}}-{{taskrun}}",
				OCIGroupByPipelineRun: true,
			},
		},
		{
			name: "defaults when empty",
			data: map[string]string{},
			want: &ArtifactStorage{
				Enabled:            false,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "only repository and enabled",
			data: map[string]string{
				"enabled":        "true",
				"oci-repository": "registry:5000/artifacts",
			},
			want: &ArtifactStorage{
				Enabled:            true,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "registry:5000/artifacts",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "invalid insecure value",
			data: map[string]string{
				"insecure": "not-a-bool",
			},
			wantErr: true,
		},
		{
			name: "invalid enabled value",
			data: map[string]string{
				"enabled": "not-a-bool",
			},
			wantErr: true,
		},
		{
			name: "invalid inline-threshold not a number",
			data: map[string]string{
				"inline-threshold": "abc",
			},
			wantErr: true,
		},
		{
			name: "invalid inline-threshold negative",
			data: map[string]string{
				"inline-threshold": "-1",
			},
			wantErr: true,
		},
		{
			name: "invalid inline-threshold exceeds ceiling",
			data: map[string]string{
				"inline-threshold": "4096",
			},
			wantErr: true,
		},
		{
			name: "inline-threshold at ceiling is valid",
			data: map[string]string{
				"inline-threshold": "1024",
			},
			want: &ArtifactStorage{
				Backend:            DefaultBackend,
				InlineThreshold:    1024,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "inline-threshold zero is valid",
			data: map[string]string{
				"inline-threshold": "0",
			},
			want: &ArtifactStorage{
				Backend:            DefaultBackend,
				InlineThreshold:    0,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "invalid oci.attachReferrers",
			data: map[string]string{
				"oci.attachReferrers": "not-a-bool",
			},
			wantErr: true,
		},
		{
			name: "invalid oci.groupByPipelineRun",
			data: map[string]string{
				"oci.groupByPipelineRun": "not-a-bool",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewArtifactStorageFromMap(tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewArtifactStorageFromMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestReferrersEnabled(t *testing.T) {
	tests := []struct {
		name string
		as   *ArtifactStorage
		want bool
	}{
		{
			name: "nil storage",
			as:   nil,
			want: false,
		},
		{
			name: "all enabled",
			as: &ArtifactStorage{
				Enabled:            true,
				OCIAttachReferrers: true,
				OCIRepository:      "registry:5000/artifacts",
			},
			want: true,
		},
		{
			name: "storage disabled",
			as: &ArtifactStorage{
				Enabled:            false,
				OCIAttachReferrers: true,
				OCIRepository:      "registry:5000/artifacts",
			},
			want: false,
		},
		{
			name: "attach referrers disabled",
			as: &ArtifactStorage{
				Enabled:            true,
				OCIAttachReferrers: false,
				OCIRepository:      "registry:5000/artifacts",
			},
			want: false,
		},
		{
			name: "empty repository",
			as: &ArtifactStorage{
				Enabled:            true,
				OCIAttachReferrers: true,
				OCIRepository:      "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.as.ReferrersEnabled(); got != tt.want {
				t.Errorf("ReferrersEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewArtifactStorageFromConfigMap(t *testing.T) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-artifact-storage",
			Namespace: "tekton-pipelines",
		},
		Data: map[string]string{
			"enabled":                "true",
			"backend":                "oci",
			"inline-threshold":       "1024",
			"oci-repository":         "registry:5000/tekton-artifacts",
			"insecure":               "true",
			"oci.credentialsSecret":  "my-registry-creds",
			"oci.attachReferrers":    "true",
			"oci.tagPattern":         "{{namespace}}.{{taskrun}}.{{artifact}}",
			"oci.groupByPipelineRun": "false",
		},
	}

	got, err := NewArtifactStorageFromConfigMap(cm)
	if err != nil {
		t.Fatalf("NewArtifactStorageFromConfigMap() error = %v", err)
	}
	want := &ArtifactStorage{
		Enabled:              true,
		Backend:              "oci",
		InlineThreshold:      1024,
		OCIRepository:        "registry:5000/tekton-artifacts",
		Insecure:             true,
		OCICredentialsSecret: "my-registry-creds",
		OCIAttachReferrers:   true,
		OCITagPattern:        "{{namespace}}.{{taskrun}}.{{artifact}}",
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestDeepCopy(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		var as *ArtifactStorage
		got := as.DeepCopy()
		if got != nil {
			t.Errorf("DeepCopy(nil) = %v, want nil", got)
		}
	})

	t.Run("independent copy", func(t *testing.T) {
		original := &ArtifactStorage{
			Enabled:               true,
			Backend:               "oci",
			InlineThreshold:       512,
			OCIRepository:         "registry:5000/artifacts",
			Insecure:              false,
			OCICredentialsSecret:  "my-secret",
			OCIAttachReferrers:    true,
			OCITagPattern:         DefaultTagPattern,
			OCIGroupByPipelineRun: false,
		}
		copied := original.DeepCopy()

		if d := cmp.Diff(original, copied); d != "" {
			t.Fatalf("DeepCopy should be equal to original (-want +got):\n%s", d)
		}

		copied.Enabled = false
		copied.OCIRepository = "mutated"
		copied.InlineThreshold = 0
		copied.Backend = "s3"

		if original.Enabled != true {
			t.Error("mutating copy changed original Enabled")
		}
		if original.OCIRepository != "registry:5000/artifacts" {
			t.Error("mutating copy changed original OCIRepository")
		}
		if original.InlineThreshold != 512 {
			t.Error("mutating copy changed original InlineThreshold")
		}
		if original.Backend != "oci" {
			t.Error("mutating copy changed original Backend")
		}
	})
}

func TestApplyOverridesFromMap(t *testing.T) {
	baseStorage := func() *ArtifactStorage {
		return &ArtifactStorage{
			Enabled:               false,
			Backend:               DefaultBackend,
			InlineThreshold:       DefaultInlineThreshold,
			OCIRepository:         "",
			Insecure:              false,
			OCICredentialsSecret:  "",
			OCIAttachReferrers:    true,
			OCITagPattern:         DefaultTagPattern,
			OCIGroupByPipelineRun: false,
		}
	}

	tests := []struct {
		name    string
		base    *ArtifactStorage
		cfgMap  map[string]string
		want    *ArtifactStorage
		wantErr bool
	}{
		{
			name:   "nil base is no-op",
			base:   nil,
			cfgMap: map[string]string{"enabled": "true"},
		},
		{
			name:   "empty map leaves base unchanged",
			base:   baseStorage(),
			cfgMap: map[string]string{},
			want:   baseStorage(),
		},
		{
			name: "partial override only sets specified fields",
			base: baseStorage(),
			cfgMap: map[string]string{
				"enabled":        "true",
				"oci-repository": "quay.io/org/artifacts",
			},
			want: &ArtifactStorage{
				Enabled:            true,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "quay.io/org/artifacts",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "full override sets all fields",
			base: baseStorage(),
			cfgMap: map[string]string{
				"enabled":                "true",
				"backend":                "s3",
				"inline-threshold":       "256",
				"oci-repository":         "ghcr.io/org/art",
				"insecure":               "true",
				"oci.credentialsSecret":  "cred-secret",
				"oci.attachReferrers":    "false",
				"oci.tagPattern":         "custom-{{taskrun}}",
				"oci.groupByPipelineRun": "true",
			},
			want: &ArtifactStorage{
				Enabled:               true,
				Backend:               "s3",
				InlineThreshold:       256,
				OCIRepository:         "ghcr.io/org/art",
				Insecure:              true,
				OCICredentialsSecret:  "cred-secret",
				OCIAttachReferrers:    false,
				OCITagPattern:         "custom-{{taskrun}}",
				OCIGroupByPipelineRun: true,
			},
		},
		{
			name:    "invalid enabled value",
			base:    baseStorage(),
			cfgMap:  map[string]string{"enabled": "not-a-bool"},
			wantErr: true,
		},
		{
			name:    "invalid inline-threshold not a number",
			base:    baseStorage(),
			cfgMap:  map[string]string{"inline-threshold": "abc"},
			wantErr: true,
		},
		{
			name:    "negative inline-threshold",
			base:    baseStorage(),
			cfgMap:  map[string]string{"inline-threshold": "-1"},
			wantErr: true,
		},
		{
			name:    "inline-threshold exceeds max",
			base:    baseStorage(),
			cfgMap:  map[string]string{"inline-threshold": "9999"},
			wantErr: true,
		},
		{
			name:    "invalid insecure value",
			base:    baseStorage(),
			cfgMap:  map[string]string{"insecure": "nope"},
			wantErr: true,
		},
		{
			name:    "invalid oci.attachReferrers value",
			base:    baseStorage(),
			cfgMap:  map[string]string{"oci.attachReferrers": "nope"},
			wantErr: true,
		},
		{
			name:    "invalid oci.groupByPipelineRun value",
			base:    baseStorage(),
			cfgMap:  map[string]string{"oci.groupByPipelineRun": "nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplyOverridesFromMap(tt.base, tt.cfgMap)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ApplyOverridesFromMap() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr || tt.want == nil {
				return
			}
			if d := cmp.Diff(tt.want, tt.base); d != "" {
				t.Errorf("mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestResolveArtifactStorage(t *testing.T) {
	clusterBase := &ArtifactStorage{
		Enabled:            false,
		Backend:            DefaultBackend,
		InlineThreshold:    DefaultInlineThreshold,
		OCIRepository:      "cluster-repo",
		Insecure:           false,
		OCIAttachReferrers: true,
		OCITagPattern:      DefaultTagPattern,
	}

	tests := []struct {
		name       string
		base       *ArtifactStorage
		nsOverride map[string]string
		annots     map[string]string
		want       *ArtifactStorage
		wantErr    bool
	}{
		{
			name: "nil base returns nil",
			base: nil,
		},
		{
			name: "empty overrides and annotations return copy of base",
			base: clusterBase,
			want: clusterBase,
		},
		{
			name: "namespace overrides beat cluster base",
			base: clusterBase,
			nsOverride: map[string]string{
				"enabled":        "true",
				"oci-repository": "ns-repo",
			},
			want: &ArtifactStorage{
				Enabled:            true,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "ns-repo",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "annotation overrides beat namespace overrides",
			base: clusterBase,
			nsOverride: map[string]string{
				"enabled":        "true",
				"oci-repository": "ns-repo",
			},
			annots: map[string]string{
				ArtifactStorageOCIRepositoryAnnotation: "annot-repo",
			},
			want: &ArtifactStorage{
				Enabled:            true,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "annot-repo",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "annotation enabled overrides namespace enabled",
			base: clusterBase,
			nsOverride: map[string]string{
				"enabled": "false",
			},
			annots: map[string]string{
				ArtifactStorageEnabledAnnotation: "true",
			},
			want: &ArtifactStorage{
				Enabled:            true,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "cluster-repo",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
		{
			name: "all annotation fields override",
			base: clusterBase,
			annots: map[string]string{
				ArtifactStorageEnabledAnnotation:               "true",
				ArtifactStorageOCIRepositoryAnnotation:         "annot-repo",
				ArtifactStorageBackendAnnotation:               "s3",
				ArtifactStorageInlineThresholdAnnotation:       "128",
				ArtifactStorageInsecureAnnotation:              "true",
				ArtifactStorageOCICredentialsSecretAnnotation:  "annot-secret",
				ArtifactStorageOCIAttachReferrersAnnotation:    "false",
				ArtifactStorageOCITagPatternAnnotation:         "custom-{{artifact}}",
				ArtifactStorageOCIGroupByPipelineRunAnnotation: "true",
			},
			want: &ArtifactStorage{
				Enabled:               true,
				Backend:               "s3",
				InlineThreshold:       128,
				OCIRepository:         "annot-repo",
				Insecure:              true,
				OCICredentialsSecret:  "annot-secret",
				OCIAttachReferrers:    false,
				OCITagPattern:         "custom-{{artifact}}",
				OCIGroupByPipelineRun: true,
			},
		},
		{
			name:       "invalid namespace overrides return error",
			base:       clusterBase,
			nsOverride: map[string]string{"enabled": "not-a-bool"},
			wantErr:    true,
		},
		{
			name: "invalid annotation overrides return error",
			base: clusterBase,
			annots: map[string]string{
				ArtifactStorageInlineThresholdAnnotation: "not-a-number",
			},
			wantErr: true,
		},
		{
			name: "does not mutate original base",
			base: clusterBase,
			annots: map[string]string{
				ArtifactStorageOCIRepositoryAnnotation: "mutated-repo",
				ArtifactStorageEnabledAnnotation:       "true",
			},
			want: &ArtifactStorage{
				Enabled:            true,
				Backend:            DefaultBackend,
				InlineThreshold:    DefaultInlineThreshold,
				OCIRepository:      "mutated-repo",
				Insecure:           false,
				OCIAttachReferrers: true,
				OCITagPattern:      DefaultTagPattern,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var baseBefore *ArtifactStorage
			if tt.base != nil {
				cp := *tt.base
				baseBefore = &cp
			}

			got, err := ResolveArtifactStorage(tt.base, tt.nsOverride, tt.annots)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveArtifactStorage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("resolved mismatch (-want +got):\n%s", d)
			}
			if baseBefore != nil {
				if d := cmp.Diff(baseBefore, tt.base); d != "" {
					t.Errorf("original base was mutated (-before +after):\n%s", d)
				}
			}
		})
	}
}
