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

package resources

import (
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateArtifactInputs(t *testing.T) {
	tests := []struct {
		name         string
		pipelineSpec *v1.PipelineSpec
		pr           *v1.PipelineRun
		wantErr      bool
	}{
		{
			name: "valid: PipelineRun input matches Pipeline declaration",
			pipelineSpec: &v1.PipelineSpec{
				Artifacts: &v1.PipelineArtifactDeclarations{
					Inputs: []v1.ArtifactDeclaration{
						{Name: "source"},
					},
				},
			},
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-run"},
				Spec: v1.PipelineRunSpec{
					Artifacts: &v1.PipelineRunArtifacts{
						Inputs: []v1.Artifact{
							{
								Name: "source",
								Values: []v1.ArtifactValue{
									{Uri: "registry:5000/artifacts/source@sha256:abc123"},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "error: PipelineRun provides undeclared input",
			pipelineSpec: &v1.PipelineSpec{
				Artifacts: &v1.PipelineArtifactDeclarations{
					Inputs: []v1.ArtifactDeclaration{
						{Name: "source"},
					},
				},
			},
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-run"},
				Spec: v1.PipelineRunSpec{
					Artifacts: &v1.PipelineRunArtifacts{
						Inputs: []v1.Artifact{
							{
								Name: "missing",
								Values: []v1.ArtifactValue{
									{Uri: "registry:5000/artifacts/missing@sha256:abc123"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name:         "valid: nil artifacts on both sides",
			pipelineSpec: &v1.PipelineSpec{},
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-run"},
				Spec:       v1.PipelineRunSpec{},
			},
			wantErr: false,
		},
		{
			name:         "valid: nil PipelineRun artifacts",
			pipelineSpec: &v1.PipelineSpec{},
			pr: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "test-run"},
				Spec: v1.PipelineRunSpec{
					Artifacts: nil,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateArtifactInputs(tt.pipelineSpec, tt.pr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArtifactInputs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
