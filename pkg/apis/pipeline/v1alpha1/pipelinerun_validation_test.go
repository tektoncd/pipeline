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

package v1alpha1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestPipelineRun_Invalidate(t *testing.T) {
	tests := []struct {
		name string
		pr   v1alpha1.PipelineRun
		want *apis.FieldError
	}{
		{
			name: "invalid pipelinerun",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prmetaname",
				},
			},
			want: apis.ErrMissingField("spec"),
		},
		{
			name: "invalid pipelinerun metadata",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinerun.name",
				},
			},
			want: &apis.FieldError{
				Message: "Invalid resource name: special character . must not be present",
				Paths:   []string{"metadata.name"},
			},
		}, {
			name: "no pipeline reference",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: v1alpha1.PipelineRunSpec{
					ServiceAccountName: "foo",
				},
			},
			want: apis.ErrMissingField("spec.pipelineref.name, spec.pipelinespec"),
		}, {
			name: "negative pipeline timeout",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: v1alpha1.PipelineRunSpec{
					PipelineRef: &v1alpha1.PipelineRef{
						Name: "prname",
					},
					Timeout: &metav1.Duration{Duration: -48 * time.Hour},
				},
			},
			want: apis.ErrInvalidValue("-48h0m0s should be >= 0", "spec.timeout"),
		},
	}

	for _, ps := range tests {
		t.Run(ps.name, func(t *testing.T) {
			err := ps.pr.Validate(context.Background())
			if d := cmp.Diff(err.Error(), ps.want.Error()); d != "" {
				t.Errorf("PipelineRun.Validate/%s (-want, +got) = %v", ps.name, d)
			}
		})
	}
}

func TestPipelineRun_Validate(t *testing.T) {
	tests := []struct {
		name string
		pr   v1alpha1.PipelineRun
	}{
		{
			name: "normal case",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: v1alpha1.PipelineRunSpec{
					PipelineRef: &v1alpha1.PipelineRef{
						Name: "prname",
					},
				},
			},
		}, {
			name: "no timeout",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: v1alpha1.PipelineRunSpec{
					PipelineRef: &v1alpha1.PipelineRef{
						Name: "prname",
					},
					Timeout: &metav1.Duration{Duration: 0},
				},
			},
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			if err := ts.pr.Validate(context.Background()); err != nil {
				t.Errorf("Unexpected PipelineRun.Validate() error = %v", err)
			}
		})
	}
}

func TestPipelineRunSpec_Invalidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    v1alpha1.PipelineRunSpec
		wantErr *apis.FieldError
	}{{
		name:    "Empty pipelineSpec",
		spec:    v1alpha1.PipelineRunSpec{},
		wantErr: apis.ErrMissingField("spec"),
	}, {
		name: "pipelineRef without Pipeline Name",
		spec: v1alpha1.PipelineRunSpec{
			PipelineRef: &v1alpha1.PipelineRef{},
		},
		wantErr: apis.ErrMissingField("spec.pipelineref.name", "spec.pipelinespec"),
	}, {
		name: "pipelineRef and pipelineSpec together",
		spec: v1alpha1.PipelineRunSpec{
			PipelineRef: &v1alpha1.PipelineRef{
				Name: "pipelinerefname",
			},
			PipelineSpec: &v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name: "mytask",
					TaskRef: &v1alpha1.TaskRef{
						Name: "mytask",
					},
				}}},
		},
		wantErr: apis.ErrDisallowedFields("spec.pipelinespec", "spec.pipelineref"),
	}, {
		name: "workspaces may only appear once",
		spec: v1alpha1.PipelineRunSpec{
			PipelineRef: &v1alpha1.PipelineRef{
				Name: "pipelinerefname",
			},
			Workspaces: []v1alpha1.WorkspaceBinding{{
				Name:     "ws",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}, {
				Name:     "ws",
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}},
		},
		wantErr: &apis.FieldError{
			Message: `workspace "ws" provided by pipelinerun more than once, at index 0 and 1`,
			Paths:   []string{"spec.workspaces"},
		},
	}}
	for _, ps := range tests {
		t.Run(ps.name, func(t *testing.T) {
			err := ps.spec.Validate(context.Background())
			if d := cmp.Diff(ps.wantErr.Error(), err.Error()); d != "" {
				t.Errorf("PipelineRunSpec.Validate/%s (-want, +got) = %v", ps.name, d)
			}
		})
	}
}

func TestPipelineRunSpec_Validate(t *testing.T) {
	tests := []struct {
		name string
		spec v1alpha1.PipelineRunSpec
	}{{
		name: "PipelineRun without pipelineRef",
		spec: v1alpha1.PipelineRunSpec{
			PipelineSpec: &v1alpha1.PipelineSpec{
				Tasks: []v1alpha1.PipelineTask{{
					Name: "mytask",
					TaskRef: &v1alpha1.TaskRef{
						Name: "mytask",
					},
				}},
			},
		},
	}}
	for _, ps := range tests {
		t.Run(ps.name, func(t *testing.T) {
			if err := ps.spec.Validate(context.Background()); err != nil {
				t.Errorf("PipelineRunSpec.Validate/%s (-want, +got) = %v", ps.name, err)
			}
		})
	}
}
