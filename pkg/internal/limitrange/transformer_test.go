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
package limitrange

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakelimitrangeinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/limitrange/fake"
	fakeserviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
)

var resourceQuantityCmp = cmp.Comparer(func(x, y resource.Quantity) bool {
	return x.Cmp(y) == 0
})

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
				Name:  "foo",
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
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{},
				},
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
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.Quantity{},
						corev1.ResourceMemory:           resource.Quantity{},
						corev1.ResourceEphemeralStorage: resource.Quantity{},
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.Quantity{},
						corev1.ResourceMemory:           resource.Quantity{},
						corev1.ResourceEphemeralStorage: resource.Quantity{},
					},
				},
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
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}},
			Containers: []corev1.Container{{
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
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("500m"),
						corev1.ResourceMemory:           resource.MustParse("50Mi"),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(536870912, resource.BinarySI),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
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
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
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
				Name:  "foo",
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
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
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
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("400Mi"),
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
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			ctx, cancel := setup(t,
				[]corev1.ServiceAccount{{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}}},
				[]corev1.LimitRange{{ObjectMeta: metav1.ObjectMeta{Name: "limitrange", Namespace: "default"},
					Spec: corev1.LimitRangeSpec{
						Limits: tc.limitranges,
					},
				}},
			)
			defer cancel()
			f := NewTransformer(ctx, "default", fakelimitrangeinformer.Get(ctx).Lister())
			got, err := f(&corev1.Pod{
				Spec: tc.podspec,
			})
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
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
				Name:  "foo",
				Image: "baz",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}, {
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				// No resources set
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{},
				},
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
					Requests: corev1.ResourceList{},
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
				Name:  "foo",
				Image: "baz",
			}, {
				Name:  "bar",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.Quantity{},
						corev1.ResourceMemory:           resource.Quantity{},
						corev1.ResourceEphemeralStorage: resource.Quantity{},
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.Quantity{},
						corev1.ResourceMemory:           resource.Quantity{},
						corev1.ResourceEphemeralStorage: resource.Quantity{},
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.Quantity{},
						corev1.ResourceMemory:           resource.Quantity{},
						corev1.ResourceEphemeralStorage: resource.Quantity{},
					},
				},
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
				Name:  "foo",
				Image: "baz",
			}, {
				Name:  "bar",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("333m"),
						corev1.ResourceMemory:           *resource.NewQuantity(34952533, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(357913941, resource.BinarySI),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("333m"),
						corev1.ResourceMemory:           *resource.NewQuantity(34952533, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(357913941, resource.BinarySI),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("333m"),
						corev1.ResourceMemory:           *resource.NewQuantity(34952533, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(357913941, resource.BinarySI),
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
				Name:  "foo",
				Image: "baz",
			}, {
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("333m"),
						corev1.ResourceMemory:           *resource.NewQuantity(34952533, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(357913941, resource.BinarySI),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("333m"),
						corev1.ResourceMemory:           *resource.NewQuantity(34952533, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(357913941, resource.BinarySI),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("333m"),
						corev1.ResourceMemory:           *resource.NewQuantity(34952533, resource.BinarySI),
						corev1.ResourceEphemeralStorage: *resource.NewQuantity(357913941, resource.BinarySI),
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
				Name:  "foo",
				Image: "baz",
			}, {
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("100Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
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
				Name:  "foo",
				Image: "baz",
			}, {
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}, {
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("700m"),
						corev1.ResourceMemory:           resource.MustParse("70Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("700Mi"),
					},
				},
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			ctx, cancel := setup(t,
				[]corev1.ServiceAccount{{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}}},
				[]corev1.LimitRange{{ObjectMeta: metav1.ObjectMeta{Name: "limitrange", Namespace: "default"},
					Spec: corev1.LimitRangeSpec{
						Limits: tc.limitranges,
					},
				}},
			)
			defer cancel()
			f := NewTransformer(ctx, "default", fakelimitrangeinformer.Get(ctx).Lister())
			got, err := f(&corev1.Pod{
				Spec: tc.podspec,
			})
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
			// We only care about the request and limit, ignoring the rest of the spec
			cmpRequestsAndLimits(t, tc.want, got.Spec)
		})
	}
}

func TestTransformerOneContainerMultipleLimitRange(t *testing.T) {
	for _, tc := range []struct {
		description string
		limitranges []corev1.LimitRange
		podspec     corev1.PodSpec
		want        corev1.PodSpec
	}{{
		description: "limitRange with default requests and limits, min and max and no resources on containers",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
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
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Max: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("4"),
						corev1.ResourceMemory:           resource.MustParse("400Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("4Gi"),
					},
					Min: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("800m"),
						corev1.ResourceMemory:           resource.MustParse("80Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("800Mi"),
					},
					Default: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("3"),
						corev1.ResourceMemory:           resource.MustParse("300Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("3Gi"),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1500m"),
						corev1.ResourceMemory:           resource.MustParse("150Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1.5Gi"),
					},
				}},
			},
		}},
		podspec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Name:  "bar",
				Image: "foo",
			}},
			Containers: []corev1.Container{{
				Name:  "foo",
				Image: "baz",
			}},
		},
		want: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("800m"),
						corev1.ResourceMemory:           resource.MustParse("80Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("800Mi"),
					},
				},
			}},
			Containers: []corev1.Container{{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("2"),
						corev1.ResourceMemory:           resource.MustParse("200Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("800m"),
						corev1.ResourceMemory:           resource.MustParse("80Mi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("800Mi"),
					},
				},
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			ctx, cancel := setup(t,
				[]corev1.ServiceAccount{{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}}},
				tc.limitranges,
			)
			defer cancel()
			f := NewTransformer(ctx, "default", fakelimitrangeinformer.Get(ctx).Lister())
			got, err := f(&corev1.Pod{
				Spec: tc.podspec,
			})
			if err != nil {
				t.Fatalf("Transformer failed: %v", err)
			}
			// We only care about the request and limit, ignoring the rest of the spec
			cmpRequestsAndLimits(t, tc.want, got.Spec)
		})
	}
}

func cmpRequestsAndLimits(t *testing.T, want, got corev1.PodSpec) {
	// diff init containers
	if len(want.InitContainers) != len(got.InitContainers) {
		t.Errorf("Expected %d init containers, got %d", len(want.InitContainers), len(got.InitContainers))
	} else {
		for i, c := range got.InitContainers {
			if d := cmp.Diff(want.InitContainers[i].Resources, c.Resources, resourceQuantityCmp); d != "" {
				t.Errorf("Diff initcontainers[%d] %s", i, diff.PrintWantGot(d))
			}
		}
	}

	// diff containers
	if len(want.Containers) != len(got.Containers) {
		t.Errorf("Expected %d containers, got %d", len(want.Containers), len(got.Containers))
	} else {
		for i, c := range got.Containers {
			if d := cmp.Diff(want.Containers[i].Resources, c.Resources, resourceQuantityCmp); d != "" {
				t.Errorf("Diff containers[%d] %s", i, diff.PrintWantGot(d))
			}
		}
	}
}

func setup(t *testing.T, serviceaccounts []corev1.ServiceAccount, limitranges []corev1.LimitRange) (context.Context, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	kubeclient := fakekubeclient.Get(ctx)
	// LimitRange
	limitRangeInformer := fakelimitrangeinformer.Get(ctx)
	kubeclient.PrependReactor("*", "limitranges", test.AddToInformer(t, limitRangeInformer.Informer().GetIndexer()))
	for _, tl := range limitranges {
		if _, err := kubeclient.CoreV1().LimitRanges(tl.Namespace).Create(ctx, &tl, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	// ServiceAccount
	serviceAccountInformer := fakeserviceaccountinformer.Get(ctx)
	kubeclient.PrependReactor("*", "serviceaccounts", test.AddToInformer(t, serviceAccountInformer.Informer().GetIndexer()))
	for _, ts := range serviceaccounts {
		if _, err := kubeclient.CoreV1().ServiceAccounts(ts.Namespace).Create(ctx, &ts, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	kubeclient.ClearActions()
	return ctx, cancel
}
