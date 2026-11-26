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

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"
)

func TestTemplateValidateResources(t *testing.T) {
	tests := []struct {
		name    string
		tpl     *Template
		wantErr *apis.FieldError
	}{{
		name: "nil template",
	}, {
		name: "no resources",
		tpl:  &Template{},
	}, {
		name: "valid requests and limits",
		tpl: &Template{Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
				"hugepages-2Mi":       resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
				"hugepages-2Mi":       resource.MustParse("100Mi"),
			},
		}},
	}, {
		name: "requests without limits",
		tpl: &Template{Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("4Gi")},
		}},
	}, {
		// Supported resource names and fields vary across Kubernetes versions,
		// so they are left for Kubernetes to validate.
		name: "resource names and claims are not validated",
		tpl: &Template{Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceEphemeralStorage: resource.MustParse("1Gi")},
			Limits:   corev1.ResourceList{"example.com/device": resource.MustParse("1")},
			Claims:   []corev1.ResourceClaim{{Name: "device"}},
		}},
	}, {
		name: "request greater than limit",
		tpl: &Template{Resources: &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}},
		wantErr: apis.ErrInvalidValue("2", "resources.requests[cpu]", "must be less than or equal to cpu limit of 1"),
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tpl.ValidateResources()
			if d := cmp.Diff(tc.wantErr.Error(), err.Error()); d != "" {
				t.Errorf("ValidateResources() (-want, +got): %s", d)
			}
		})
	}
}
