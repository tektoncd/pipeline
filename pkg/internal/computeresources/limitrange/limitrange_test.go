/*
Copyright 2022 The Tekton Authors

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
package limitrange_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/compare"
	"github.com/tektoncd/pipeline/pkg/internal/computeresources/limitrange"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	fakelimitrangeinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/limitrange/fake"
	fakeserviceaccountinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount/fake"
)

func setupTestData(t *testing.T, serviceaccounts []corev1.ServiceAccount, limitranges []corev1.LimitRange) (context.Context, func()) {
	t.Helper()
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

func TestGetVirtualLimitRange(t *testing.T) {
	limitRange1 := corev1.LimitRange{
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
			}}}}

	for _, tc := range []struct {
		description string
		limitranges []corev1.LimitRange
		want        *corev1.LimitRange
	}{{
		description: "No limitRange",
		want:        nil,
	}, {
		description: "Single LimitRange",
		limitranges: []corev1.LimitRange{limitRange1},
		want:        &limitRange1,
	}, {
		description: "multiple LimitRanges; min only",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Min:  createResourceList(7),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Min:  createResourceList(8),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Min:  createResourceList(8),
				}}}},
	}, {
		description: "multiple LimitRanges; max only",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Max:  createResourceList(7),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Max:  createResourceList(8),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Max:  createResourceList(7),
				}}}},
	}, {
		description: "multiple LimitRanges; maxLimitRequestRatio only",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:                 corev1.LimitTypeContainer,
					MaxLimitRequestRatio: createResourceList(7),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:                 corev1.LimitTypeContainer,
					MaxLimitRequestRatio: createResourceList(8),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:                 corev1.LimitTypeContainer,
					MaxLimitRequestRatio: createResourceList(7),
				}}}},
	}, {
		description: "multiple LimitRanges; default only",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:    corev1.LimitTypeContainer,
					Default: createResourceList(7),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:    corev1.LimitTypeContainer,
					Default: createResourceList(8),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:    corev1.LimitTypeContainer,
					Default: createResourceList(7),
				}}}},
	}, {
		description: "multiple LimitRanges; defaultRequest only",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:           corev1.LimitTypeContainer,
					DefaultRequest: createResourceList(7),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:           corev1.LimitTypeContainer,
					DefaultRequest: createResourceList(8),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:           corev1.LimitTypeContainer,
					DefaultRequest: createResourceList(7),
				}}}},
	}, {
		description: "multiple LimitRanges; all fields",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:                 corev1.LimitTypeContainer,
					Min:                  createResourceList(3),
					Max:                  createResourceList(10),
					MaxLimitRequestRatio: createResourceList(3),
					Default:              createResourceList(8),
					DefaultRequest:       createResourceList(7),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:                 corev1.LimitTypeContainer,
					Min:                  createResourceList(1),
					Max:                  createResourceList(5),
					MaxLimitRequestRatio: createResourceList(2),
					Default:              createResourceList(5),
					DefaultRequest:       createResourceList(3),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type:                 corev1.LimitTypeContainer,
					Min:                  createResourceList(3),
					Max:                  createResourceList(5),
					MaxLimitRequestRatio: createResourceList(2),
					Default:              createResourceList(5),
					DefaultRequest:       createResourceList(3),
				}}}},
	}, {
		description: "multiple contradictory LimitRanges",
		limitranges: []corev1.LimitRange{{
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange1", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Min:  createResourceList(6),
					Max:  createResourceList(10),
				}}}}, {
			ObjectMeta: metav1.ObjectMeta{Name: "limitrange2", Namespace: "default"},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Min:  createResourceList(1),
					Max:  createResourceList(5),
				}}}},
		},
		want: &corev1.LimitRange{
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{{
					Type: corev1.LimitTypeContainer,
					Min:  createResourceList(6),
					Max:  createResourceList(5),
				}}}},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			ctx, cancel := setupTestData(t,
				[]corev1.ServiceAccount{{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}}},
				tc.limitranges,
			)
			defer cancel()
			lister := fakelimitrangeinformer.Get(ctx).Lister()
			got, err := limitrange.GetVirtualLimitRange("default", lister)
			if err != nil {
				t.Errorf("Unexpected error %s", err)
			}
			if d := cmp.Diff(tc.want, got, compare.ResourceQuantityCmp, compare.EquateEmptyResourceList()); d != "" {
				t.Errorf("Unexpected output for virtual limit range: %s", d)
			}
		})
	}
}

func createResourceList(multiple int) corev1.ResourceList {
	cpuStr := strconv.Itoa(multiple*100) + "m"
	memStr := strconv.Itoa(multiple*10) + "Mi"
	ephemeralStr := strconv.Itoa(multiple*100) + "Mi"
	return corev1.ResourceList{
		corev1.ResourceCPU:              resource.MustParse(cpuStr),
		corev1.ResourceMemory:           resource.MustParse(memStr),
		corev1.ResourceEphemeralStorage: resource.MustParse(ephemeralStr),
	}
}
