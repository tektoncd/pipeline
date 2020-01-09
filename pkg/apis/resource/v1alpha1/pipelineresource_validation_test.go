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
	"knative.dev/pkg/apis"
)

func TestResourceValidation_Invalid(t *testing.T) {
	tests := []struct {
		name string
		res  *v1alpha1.PipelineResource
		want *apis.FieldError
	}{
		{
			name: "cluster with invalid url",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeCluster,
					Params: []v1alpha1.ResourceParam{{
						Name: "name", Value: "test_cluster_resource",
					}, {
						Name: "url", Value: "10.10.10",
					}, {
						Name: "cadata", Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					}, {
						Name: "username", Value: "admin",
					}, {
						Name: "token", Value: "my-token",
					}},
				},
			},
			want: apis.ErrInvalidValue("10.10.10", "URL"),
		},
		{
			name: "cluster with missing auth",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeCluster,
					Params: []v1alpha1.ResourceParam{{
						Name: "name", Value: "test_cluster_resource",
					}, {
						Name: "url", Value: "http://10.10.10.10",
					}},
				},
			},
			want: apis.ErrMissingField("username or CAData  or token param"),
		}, {
			name: "cluster with missing cadata",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeCluster,
					Params: []v1alpha1.ResourceParam{{
						Name: "url", Value: "http://10.10.10.10",
					}, {
						Name: "Name", Value: "admin",
					}, {
						Name: "username", Value: "admin",
					}, {
						Name: "token", Value: "my-token",
					}},
				},
			},
			want: apis.ErrMissingField("CAData param"),
		}, {
			name: "storage with no type",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeStorage,
					Params: []v1alpha1.ResourceParam{{
						Name: "no-type-param", Value: "something",
					}},
				},
			},
			want: apis.ErrMissingField("spec.params.type"),
		}, {
			name: "storage with unimplemented type",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeStorage,
					Params: []v1alpha1.ResourceParam{{
						Name: "type", Value: "not-implemented-yet",
					}},
				},
			},
			want: apis.ErrInvalidValue("not-implemented-yet", "spec.params.type"),
		}, {
			name: "storage with gcs type with no location param",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeStorage,
					Params: []v1alpha1.ResourceParam{{
						Name: "type", Value: "gcs",
					}},
				},
			},
			want: apis.ErrMissingField("spec.params.location"),
		}, {
			name: "storage with gcs type with empty location param",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeStorage,
					Params: []v1alpha1.ResourceParam{{
						Name: "type", Value: "gcs",
					}, {
						Name: "location", Value: "",
					}},
				},
			},
			want: apis.ErrMissingField("spec.params.location"),
		}, {
			name: "invalid resource type",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: "not-supported",
				},
			},
			want: apis.ErrInvalidValue("spec.type", "not-supported"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.res.Validate(context.Background())
			if d := cmp.Diff(tt.want.Error(), err.Error()); d != "" {
				t.Errorf("Didn't get expected error for %s (-want, +got) = %v", tt.name, d)
			}
		})
	}
}

func TestClusterResourceValidation_Valid(t *testing.T) {
	tests := []struct {
		name string
		res  *v1alpha1.PipelineResource
	}{
		{
			name: "success validate",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeCluster,
					Params: []v1alpha1.ResourceParam{{
						Name: "name", Value: "test_cluster_resource",
					}, {
						Name: "url", Value: "http://10.10.10.10",
					}, {
						Name: "cadata", Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					}, {
						Name: "username", Value: "admin",
					}, {
						Name: "token", Value: "my-token",
					}},
				},
			},
		},
		{
			name: "specify insecure without cadata",
			res: &v1alpha1.PipelineResource{
				Spec: v1alpha1.PipelineResourceSpec{
					Type: v1alpha1.PipelineResourceTypeCluster,
					Params: []v1alpha1.ResourceParam{{
						Name: "name", Value: "test_cluster_resource",
					}, {
						Name: "url", Value: "http://10.10.10.10",
					}, {
						Name: "username", Value: "admin",
					}, {
						Name: "token", Value: "my-token",
					}, {
						Name: "insecure", Value: "true",
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.res.Validate(context.Background()); err != nil {
				t.Errorf("Unexpected PipelineRun.Validate() error = %v", err)
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
		{name: "storage with build-gcs type",
			storageType: "build-gcs",
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
