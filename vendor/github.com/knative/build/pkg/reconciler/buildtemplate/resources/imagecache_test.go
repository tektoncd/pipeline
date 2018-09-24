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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/build/pkg/apis/build/v1alpha1"
	caching "github.com/knative/caching/pkg/apis/caching/v1alpha1"
)

func TestMakeImageCacheFromSpec(t *testing.T) {
	boolTrue := true

	tests := []struct {
		name      string
		namespace string
		bt        *v1alpha1.BuildTemplate
		want      []caching.Image
	}{{
		name:      "no container",
		namespace: "foo",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{},
		},
	}, {
		name:      "single container",
		namespace: "foo",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "busybox",
				}},
			},
		},
		want: []caching.Image{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "busybox",
			},
		}},
	}, {
		name:      "duplicate container",
		namespace: "foo",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "busybox",
				}, {
					Image: "busybox",
				}},
			},
		},
		want: []caching.Image{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "busybox",
			},
		}},
	}, {
		name:      "multiple containers",
		namespace: "foo",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "busybox",
				}, {
					Image: "helloworld",
				}, {
					Image: "busybox",
				}},
			},
		},
		want: []caching.Image{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "busybox",
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-asdf-00001",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "helloworld",
			},
		}},
	}, {
		name:      "containers with substitutions",
		namespace: "foo",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{
				Parameters: []v1alpha1.ParameterSpec{{
					Name: "TAG",
				}},
				Steps: []corev1.Container{{
					Image: "busybox",
				}, {
					Image: "helloworld:${TAG}",
				}, {
					Image: "busybox",
				}},
			},
		},
		want: []caching.Image{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "busybox",
			},
		}},
	}, {
		name:      "single container, different namespace",
		namespace: "not-foo",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "busybox",
				}},
			},
		},
		want: []caching.Image{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "not-foo",
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "busybox",
			},
		}},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := MakeImageCachesFromSpec(test.namespace, test.bt)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("MakeImageCache (-want, +got) = %v", diff)
			}
		})
	}
}

func TestMakeImageCache(t *testing.T) {
	boolTrue := true

	// Test going through MakeImageCache, but prefer testing MakeImageCachesFromSpec.
	tests := []struct {
		name string
		bt   *v1alpha1.BuildTemplate
		want []caching.Image
	}{{
		name: "single container",
		bt: &v1alpha1.BuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "foo",
				Name:            "bar",
				UID:             "1234",
				ResourceVersion: "asdf",
			},
			Spec: v1alpha1.BuildTemplateSpec{
				Steps: []corev1.Container{{
					Image: "busybox",
				}},
			},
		},
		want: []caching.Image{{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "BuildTemplate",
					Name:               "bar",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				}},
			},
			Spec: caching.ImageSpec{
				Image: "busybox",
			},
		}},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := MakeImageCaches(test.bt)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("MakeImageCache (-want, +got) = %v", diff)
			}
		})
	}
}
