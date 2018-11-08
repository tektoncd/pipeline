/*
Copyright 2018 The Knative Authors

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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterResourceValidation_Invalid(t *testing.T) {
	tests := []struct {
		name string
		res  PipelineResource
		want *apis.FieldError
	}{
		{
			name: "cluster with invalid url",
			res: PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-resource",
					Namespace: "foo",
				},
				Spec: PipelineResourceSpec{
					Type: PipelineResourceTypeCluster,
					Params: []Param{{
						Name:  "url",
						Value: "10.10.10",
					}, {
						Name:  "username",
						Value: "admin",
					}, {
						Name:  "cadata",
						Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					}, {
						Name:  "token",
						Value: "my-token",
					},
					},
				},
			},
			want: apis.ErrInvalidValue("10.10.10", "URL"),
		},
		{
			name: "cluster with missing username",
			res: PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-resource",
					Namespace: "foo",
				},
				Spec: PipelineResourceSpec{
					Type: PipelineResourceTypeCluster,
					Params: []Param{{
						Name:  "url",
						Value: "http://10.10.10.10",
					}, {
						Name:  "cadata",
						Value: "bXktY2x1c3Rlci1jZXJ0Cg",
					}, {
						Name:  "token",
						Value: "my-token",
					},
					},
				},
			},
			want: apis.ErrMissingField("username param"),
		},
		{
			name: "cluster with missing cadata",
			res: PipelineResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-resource",
					Namespace: "foo",
				},
				Spec: PipelineResourceSpec{
					Type: PipelineResourceTypeCluster,
					Params: []Param{{
						Name:  "url",
						Value: "http://10.10.10.10",
					}, {
						Name:  "username",
						Value: "admin",
					}, {
						Name:  "token",
						Value: "my-token",
					},
					},
				},
			},
			want: apis.ErrMissingField("CAData param"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.res.Validate()
			if d := cmp.Diff(err.Error(), tt.want.Error()); d != "" {
				t.Errorf("PipleineResource.Validate/%s (-want, +got) = %v", tt.name, d)
			}
		})
	}
}

func TestClusterResourceValidation_Valid(t *testing.T) {
	res := &PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-resource",
			Namespace: "foo",
		},
		Spec: PipelineResourceSpec{
			Type: PipelineResourceTypeCluster,
			Params: []Param{{
				Name:  "url",
				Value: "http://10.10.10.10",
			}, {
				Name:  "username",
				Value: "admin",
			}, {
				Name:  "cadata",
				Value: "bXktY2x1c3Rlci1jZXJ0Cg",
			}, {
				Name:  "token",
				Value: "my-token",
			},
			},
		},
	}
	if err := res.Validate(); err != nil {
		t.Errorf("Unexpected PipelineRun.Validate() error = %v", err)
	}
}
