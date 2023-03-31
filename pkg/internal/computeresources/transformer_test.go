/*
Copyright 2020 The Tekton Authors

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

package computeresources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/compare"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTransformerOneContainer(t *testing.T) {
	for _, tc := range []struct {
		description string
		limitranges []corev1.LimitRangeItem
		podspec     corev1.PodSpec
		want        corev1.PodSpec
	}{{
		description: "no limit range, no change",
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				// No resources set
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with default limits and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
		},
	}, {
		description: "limitRange with default requests and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with default requests and limits and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with not default but min and max and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Min: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
			Max: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with partial min and max and partial resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Min: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
			Max: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("4"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("400Mi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory:           resource.MustParse("4Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("400Mi"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("4"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("400Mi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory:           resource.MustParse("4Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("400Mi"),
						corev1.ResourceCPU:    resource.MustParse("1"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with default requests and limits, min and max and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Max: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("3"),
				corev1.ResourceMemory:           resource.MustParse("300Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("3Gi"),
			},
			Min: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("700m"),
				corev1.ResourceMemory:           resource.MustParse("70Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
			},
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with min and requests on containers < min",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Min: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with min and limits on containers < min",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Min: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("800m"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("1"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with max and limits on containers > max",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Max: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("2"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with max and requests on containers > max",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Max: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("2"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("3"),
					},
				},
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			pod := corev1.Pod{Spec: tc.podspec}
			limitRange := corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "limitrange", Namespace: "default"},
				Spec: corev1.LimitRangeSpec{
					Limits: tc.limitranges,
				}}
			got := transformPodBasedOnLimitRange(&pod, &limitRange)
			// We only care about the request and limit, ignoring the rest of the spec
			cmpRequestsAndLimits(t, tc.want, got.Spec)
		})
	}
}

func TestTransformerMultipleContainer(t *testing.T) {
	for _, tc := range []struct {
		description string
		limitranges []corev1.LimitRangeItem
		podspec     corev1.PodSpec
		want        corev1.PodSpec
	}{{
		description: "no limit range, no change",
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}, {
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				// No resources set
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}, {
				// No resources set
				Resources: corev1.ResourceRequirements{},
			}},
		},
	}, {
		description: "limitRange with default limits and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-bar",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}, {
				Resources: corev1.ResourceRequirements{},
			}},
		},
	}, {
		description: "limitRange with default requests and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-bar",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}},
		},
	}, {
		description: "limitRange with default requests and limits and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}},
		},
	}, {
		description: "limitRange with not default but min and max and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Min: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
			Max: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with default requests and limits, min and max and no resources on containers",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Max: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("3"),
				corev1.ResourceMemory:           resource.MustParse("300Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("3Gi"),
			},
			Min: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("700m"),
				corev1.ResourceMemory:           resource.MustParse("70Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
			},
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}},
		},
	}, {
		description: "limitRange with default requests and no resources on 2 step containers and a sidecar",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-bar",
				Image: "baz",
			}, {
				Name:  "sidecar-fizz",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Name: "step-foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}, {
				Name: "step-bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}, {
				Name:      "sidecar-fizz",
				Resources: corev1.ResourceRequirements{},
			}},
		},
	}, {
		description: "limitRange with default requests and no resources on 2 step containers and a sidecar with resources",
		limitranges: []corev1.LimitRangeItem{{
			Type: corev1.LimitTypeContainer,
			Default: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("2"),
				corev1.ResourceMemory:           resource.MustParse("200Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
			},
			DefaultRequest: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("100Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "step-foo",
				Image: "baz",
			}, {
				Name:  "step-bar",
				Image: "baz",
			}, {
				Name:  "sidecar-fizz",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("4"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("400Mi"),
					},
				},
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{},
			}},
			Containers: []corev1.Container{{
				Name: "step-foo",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}, {
				Name: "step-bar",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}, {
				Name: "sidecar-fizz",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: resource.MustParse("4"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("400Mi"),
					}},
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			pod := corev1.Pod{Spec: tc.podspec}
			limitRange := corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "limitrange", Namespace: "default"},
				Spec: corev1.LimitRangeSpec{
					Limits: tc.limitranges,
				}}
			got := transformPodBasedOnLimitRange(&pod, &limitRange)
			// We only care about the request and limit, ignoring the rest of the spec
			cmpRequestsAndLimits(t, tc.want, got.Spec)
		})
	}
}

func cmpRequestsAndLimits(t *testing.T, want, got corev1.PodSpec) {
	t.Helper()
	// diff init containers
	if len(want.InitContainers) != len(got.InitContainers) {
		t.Errorf("Expected %d init containers, got %d", len(want.InitContainers), len(got.InitContainers))
	} else {
		for i, c := range got.InitContainers {
			// compare name only if present in "want" so we can be sure which container gets which resources
			if want.InitContainers[i].Name != "" {
				if d := cmp.Diff(want.InitContainers[i].Name, c.Name); d != "" {
					t.Errorf("Diff initcontainers[%d] %s", i, diff.PrintWantGot(d))
				}
			}

			if d := cmp.Diff(want.InitContainers[i].Resources, c.Resources, compare.ResourceQuantityCmp); d != "" {
				t.Errorf("Diff initcontainers[%d] %s", i, diff.PrintWantGot(d))
			}
		}
	}

	// diff containers
	if len(want.Containers) != len(got.Containers) {
		t.Errorf("Expected %d containers, got %d", len(want.Containers), len(got.Containers))
	} else {
		for i, c := range got.Containers {
			// compare name only if present in "want" so we can be sure which container gets which resources
			if want.Containers[i].Name != "" {
				if d := cmp.Diff(want.Containers[i].Name, c.Name); d != "" {
					t.Errorf("Diff containers[%d] %s", i, diff.PrintWantGot(d))
				}
			}

			if d := cmp.Diff(want.Containers[i].Resources, c.Resources, compare.ResourceQuantityCmp); d != "" {
				t.Errorf("Diff containers[%d] %s", i, diff.PrintWantGot(d))
			}
		}
	}
}
