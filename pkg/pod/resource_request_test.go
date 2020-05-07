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
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
	return x.Cmp(y) == 0
})

func TestResolveResourceRequests_No_LimitRange(t *testing.T) {
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
			// ResourceCPU max request
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("10"),
					corev1.ResourceMemory:           zeroQty,
					corev1.ResourceEphemeralStorage: zeroQty,
				},
			},
		}, {
			// ResourceMemory max request
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              zeroQty,
					corev1.ResourceMemory:           resource.MustParse("10Gi"),
					corev1.ResourceEphemeralStorage: zeroQty,
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("11Gi"),
				},
			},
		}, {
			// ResourceEphemeralStorage max request
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              zeroQty,
					corev1.ResourceMemory:           zeroQty,
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Gi"),
				},
			},
		}},
	}, {
		desc: "Max requests all with step2",
		in: []corev1.Container{{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("10"),
					corev1.ResourceMemory:           resource.MustParse("10Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
			},
		}, {
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("11"),
					corev1.ResourceMemory:           resource.MustParse("11Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("101Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("11Gi"),
				},
			},
		}, {
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
		want: []corev1.Container{{
			// All zeroed out since step 2 has max requests
			Resources: corev1.ResourceRequirements{
				Requests: allZeroQty(),
			},
		}, {
			// All max requests
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("11"),
					corev1.ResourceMemory:           resource.MustParse("11Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("101Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("11Gi"),
				},
			},
		}, {
			// All zeroed out since step 2 has max requests
			Resources: corev1.ResourceRequirements{
				Requests: allZeroQty(),
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Gi"),
				},
			},
		}},
	},
		{
			desc: "Only one step container with only memory request value filled out",
			in: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("10Gi"),
					},
				},
			}},
			want: []corev1.Container{{
				// ResourceMemory max request set. zeroQty for non set resources.
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              zeroQty,
						corev1.ResourceMemory:           resource.MustParse("10Gi"),
						corev1.ResourceEphemeralStorage: zeroQty,
					},
				},
			}},
		}, {
			desc: "Only one step container with all request values filled out",
			in: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("10"),
						corev1.ResourceMemory:           resource.MustParse("10Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
					},
				},
			}},
			want: []corev1.Container{{
				// All max values set
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("10"),
						corev1.ResourceMemory:           resource.MustParse("10Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
					},
				},
			}},
		},
	} {
		t.Run(c.desc, func(t *testing.T) {
			got := resolveResourceRequests(c.in, allZeroQty())
			if d := cmp.Diff(c.want, got, resourceQuantityCmp); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestResolveResourceRequests_LimitRange(t *testing.T) {
	for _, c := range []struct {
		desc     string
		in, want []corev1.Container
	}{{
		desc: "three steps, no requests, apply minimum to all",
		in:   []corev1.Container{{}, {}, {}},
		want: []corev1.Container{{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("99Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100m"),
				},
			},
		}, {
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("99Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100m"),
				},
			},
		}, {
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("99Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100m"),
				},
			},
		}},
	}, {
		desc: "three steps, no requests, apply minimum values when not max values",
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
			// ResourceCPU max request
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("10"),
					corev1.ResourceMemory:           resource.MustParse("99Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100m"),
				},
			},
		}, {
			// ResourceMemory max request
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("10Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("11Gi"),
				},
			},
		}, {
			// ResourceEphemeralStorage max request
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("100m"),
					corev1.ResourceMemory:           resource.MustParse("99Mi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("100Gi"),
				},
			},
		}},
	}, {
		desc: "Only one step container with all request values filled out, no min values",
		in: []corev1.Container{{
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("10"),
					corev1.ResourceMemory:           resource.MustParse("10Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
			},
		}},
		want: []corev1.Container{{
			// All max values set
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("10"),
					corev1.ResourceMemory:           resource.MustParse("10Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("100Gi"),
				},
			},
		}},
	},
	} {
		t.Run(c.desc, func(t *testing.T) {
			got := resolveResourceRequests(c.in, corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("100m"),
				corev1.ResourceMemory:           resource.MustParse("99Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("100m"),
			},
			)
			if d := cmp.Diff(c.want, got, resourceQuantityCmp); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
