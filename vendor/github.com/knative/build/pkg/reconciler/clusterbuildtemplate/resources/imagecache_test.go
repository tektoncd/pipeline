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
	"github.com/knative/build/pkg/system"
	caching "github.com/knative/caching/pkg/apis/caching/v1alpha1"
)

func TestMakeImageCache(t *testing.T) {
	boolTrue := true

	// We lean on the BuildTemplate's testing for this in the same way that
	// we lean on its implementation.
	tests := []struct {
		name string
		bt   *v1alpha1.ClusterBuildTemplate
		want []caching.Image
	}{{
		name: "single container",
		bt: &v1alpha1.ClusterBuildTemplate{
			ObjectMeta: metav1.ObjectMeta{
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
				Namespace: system.Namespace,
				Name:      "bar-asdf-00000",
				Labels: map[string]string{
					"controller": "1234",
					"version":    "asdf",
				},
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         "build.knative.dev/v1alpha1",
					Kind:               "ClusterBuildTemplate",
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
