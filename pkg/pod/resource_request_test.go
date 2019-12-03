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

package pod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
	return x.Cmp(y) == 0
})

func TestResolveResourceRequests(t *testing.T) {
	for _, c := range []struct {
		desc     string
		in, want []corev1.Container
	}{{
		desc: "three steps, no requests",
		in:   []corev1.Container{{}, {}, {}},
		want: []corev1.Container{{
			Resources: corev1.ResourceRequirements{Requests: allZeroQty()},
		}, {
			Resources: corev1.ResourceRequirements{Requests: allZeroQty()},
		}, {
			Resources: corev1.ResourceRequirements{Requests: allZeroQty()},
		}},
	}, {
		desc: "requests are moved, limits aren't changed",
		in: []corev1.Container{{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("10"),
				},
			},
		}, {
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("10Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("11Gi"),
				},
			},
		}, {
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Gi"),
				},
			},
		}},
		want: []corev1.Container{{
			// All zeroed out.
			Resources: corev1.ResourceRequirements{Requests: allZeroQty()},
		}, {
			// Requests zeroed out, limits remain.
			Resources: corev1.ResourceRequirements{
				Requests: allZeroQty(),
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("11Gi"),
				},
			},
		}, {
			// Requests to the max, limits remain.
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("10"),
					corev1.ResourceMemory:           resource.MustParse("10Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Gi"),
				},
			},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := resolveResourceRequests(c.in)
			if d := cmp.Diff(c.want, got, resourceQuantityCmp); d != "" {
				t.Errorf("Diff(-want, +got): %s", d)
			}
		})
	}
}
