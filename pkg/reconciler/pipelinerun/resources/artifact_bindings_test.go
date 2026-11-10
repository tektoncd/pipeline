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

package resources

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestResolveArtifactBinding(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		artifacts map[string]*v1.Artifacts
		wantURI   string
		wantErr   bool
	}{
		{
			name: "valid binding",
			from: "tasks.build.outputs.image",
			artifacts: map[string]*v1.Artifacts{
				"build": {
					Outputs: []v1.Artifact{
						{
							Name: "image",
							Values: []v1.ArtifactValue{
								{Uri: "registry:5000/artifacts/image@sha256:abc123", Digest: map[v1.Algorithm]string{"sha256": "abc123"}},
							},
						},
					},
				},
			},
			wantURI: "registry:5000/artifacts/image@sha256:abc123",
		},
		{
			name: "task not found",
			from: "tasks.missing.outputs.image",
			artifacts: map[string]*v1.Artifacts{
				"build": {Outputs: []v1.Artifact{}},
			},
			wantErr: true,
		},
		{
			name: "artifact not found",
			from: "tasks.build.outputs.missing",
			artifacts: map[string]*v1.Artifacts{
				"build": {
					Outputs: []v1.Artifact{
						{Name: "image", Values: []v1.ArtifactValue{{Uri: "foo"}}},
					},
				},
			},
			wantErr: true,
		},
		{
			name:    "invalid from format",
			from:    "invalid-format",
			wantErr: true,
		},
		{
			name: "no values",
			from: "tasks.build.outputs.image",
			artifacts: map[string]*v1.Artifacts{
				"build": {
					Outputs: []v1.Artifact{
						{Name: "image", Values: []v1.ArtifactValue{}},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveArtifactBinding(tt.from, tt.artifacts)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveArtifactBinding() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.Uri != tt.wantURI {
				t.Errorf("ResolveArtifactBinding() Uri = %q, want %q", got.Uri, tt.wantURI)
			}
		})
	}
}
