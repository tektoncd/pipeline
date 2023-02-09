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

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestResourceValidation_Invalid(t *testing.T) {
	tests := []struct {
		name string
		res  *v1alpha1.PipelineResource
		want *apis.FieldError
	}{{
		name: "invalid resource type",
		res: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.PipelineResourceSpec{
				Type: "not-supported",
			},
		},
		want: apis.ErrInvalidValue("spec.type", "not-supported"),
	}, {
		name: "missing spec",
		res: &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
		},
		want: apis.ErrMissingField("spec.type"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.res.Validate(context.Background())
			if d := cmp.Diff(tt.want.Error(), err.Error()); d != "" {
				t.Errorf("Didn't get expected error for %s %s", tt.name, diff.PrintWantGot(d))
			}
		})
	}
}

func TestAllowedGCSStorageType(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		want        bool
	}{
		{name: "storage with gcs type",
			storageType: "gcs",
			want:        true,
		},
		{name: "storage with incorrent type",
			storageType: "t",
			want:        false,
		},
		{name: "storage with empty type",
			storageType: "",
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if v1alpha1.AllowedStorageType(tt.storageType) != tt.want {
				t.Errorf("PipleineResource.allowedStorageType should return %t, got %t for %s", tt.want, !tt.want, tt.storageType)
			}
		})
	}
}
