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
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceValidation_Invalid(t *testing.T) {
	tests := []struct {
		name string
		res  *v1alpha1.PipelineResource
		want *apis.FieldError
	}{
		{
			name: "cluster with invalid url",
			res: tb.PipelineResource("test-cluster-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeCluster,
				tb.PipelineResourceSpecParam("name", "test_cluster_resource"),
				tb.PipelineResourceSpecParam("url", "10.10.10"),
				tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
				tb.PipelineResourceSpecParam("username", "admin"),
				tb.PipelineResourceSpecParam("token", "my-token"),
			)),
			want: apis.ErrInvalidValue("10.10.10", "URL"),
		},
		{
			name: "cluster with missing username",
			res: tb.PipelineResource("test-cluster-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeCluster,
				tb.PipelineResourceSpecParam("name", "test_cluster_resource"),
				tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
				tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
				tb.PipelineResourceSpecParam("token", "my-token"),
			)),
			want: apis.ErrMissingField("username param"),
		},
		{
			name: "cluster with missing name",
			res: tb.PipelineResource("test-cluster-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeCluster,
				tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
				tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
				tb.PipelineResourceSpecParam("token", "my-token"),
			)),
			want: apis.ErrMissingField("name param"),
		},
		{
			name: "cluster with missing cadata",
			res: tb.PipelineResource("test-cluster-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeCluster,
				tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
				tb.PipelineResourceSpecParam("Name", "admin"),
				tb.PipelineResourceSpecParam("token", "my-token"),
				tb.PipelineResourceSpecParam("username", "admin"),
			)),
			want: apis.ErrMissingField("CAData param"),
		}, {
			name: "storage with no type",
			res: tb.PipelineResource("storage-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("no-type-param", "something"),
			)),
			want: apis.ErrMissingField("spec.params.type"),
		}, {
			name: "storage with unimplemented type",
			res: tb.PipelineResource("storage-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("type", "not-implemented-yet"),
			)),
			want: apis.ErrInvalidValue("not-implemented-yet", "spec.params.type"),
		}, {
			name: "storage with gcs type with no location param",
			res: tb.PipelineResource("storage-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("type", "gcs"),
			)),
			want: apis.ErrMissingField("spec.params.location"),
		}, {
			name: "storage with gcs type with empty location param",
			res: tb.PipelineResource("storage-resource", "foo", tb.PipelineResourceSpec(
				v1alpha1.PipelineResourceTypeStorage,
				tb.PipelineResourceSpecParam("type", "gcs"),
				tb.PipelineResourceSpecParam("location", ""),
			)),
			want: apis.ErrMissingField("spec.params.location"),
		}, {
			name: "invalid resource type",
			res: &v1alpha1.PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name: "invalid-resource",
				},
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
			if d := cmp.Diff(err.Error(), tt.want.Error()); d != "" {
				t.Errorf("PipelineResource.Validate/%s (-want, +got) = %v", tt.name, d)
			}
		})
	}
}

func TestClusterResourceValidation_Valid(t *testing.T) {
	res := tb.PipelineResource("test-cluster-resource", "foo", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeCluster,
		tb.PipelineResourceSpecParam("name", "test_cluster_resource"),
		tb.PipelineResourceSpecParam("url", "http://10.10.10.10"),
		tb.PipelineResourceSpecParam("cadata", "bXktY2x1c3Rlci1jZXJ0Cg"),
		tb.PipelineResourceSpecParam("username", "admin"),
		tb.PipelineResourceSpecParam("token", "my-token"),
	))
	if err := res.Validate(context.Background()); err != nil {
		t.Errorf("Unexpected PipelineRun.Validate() error = %v", err)
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
