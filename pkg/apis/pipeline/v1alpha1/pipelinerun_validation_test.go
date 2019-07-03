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
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					ServiceAccount: "foo",
				},
			},
			want: apis.ErrMissingField("pipelinerun.spec.Pipelineref.Name"),
		}, {
			name: "negative pipeline timeout",
			pr: v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pipelinelineName",
				},
				Spec: v1alpha1.PipelineRunSpec{
					PipelineRef: v1alpha1.PipelineRef{
						Name: "prname",
					},
					Timeout: &metav1.Duration{Duration: -48 * time.Hour},
				},
			},
			want: apis.ErrInvalidValue("-48h0m0s should be > 0", "spec.timeout"),
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.pr.Validate(context.Background())
			if d := cmp.Diff(err.Error(), ts.want.Error()); d != "" {
				t.Errorf("PipelineRun.Validate/%s (-want, +got) = %v", ts.name, d)
			}
		})
	}
}

func TestPipelineRun_Validate(t *testing.T) {
	tr := v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinelineName",
		},
		Spec: v1alpha1.PipelineRunSpec{
			PipelineRef: v1alpha1.PipelineRef{
				Name: "prname",
			},
			Results: &v1alpha1.Results{
				URL:  "http://www.google.com",
				Type: "gcs",
			},
		},
	}
	if err := tr.Validate(context.Background()); err != nil {
		t.Errorf("Unexpected PipelineRun.Validate() error = %v", err)
	}
}
