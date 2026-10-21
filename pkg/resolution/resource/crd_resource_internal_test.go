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

package resource

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestOwnerRefsAreEqual(t *testing.T) {
	tests := []struct {
		name string
		a    metav1.OwnerReference
		b    metav1.OwnerReference
		want bool
	}{
		{
			name: "both Controller nil",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
			},
			want: true,
		},
		{
			name: "both Controller non-nil same value",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			want: true,
		},
		{
			name: "both Controller non-nil different value",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(false),
			},
			want: false,
		},
		{
			name: "a Controller nil b Controller non-nil",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			want: false,
		},
		{
			name: "a Controller non-nil b Controller nil",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
			},
			want: false,
		},
		{
			name: "same Controller different APIVersion",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			b: metav1.OwnerReference{
				APIVersion: "v1beta1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			want: false,
		},
		{
			name: "same Controller different Name",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "other-taskrun",
				UID:        "abc-123",
				Controller: ptr.To(true),
			},
			want: false,
		},
		{
			name: "fully identical with all fields",
			a: metav1.OwnerReference{
				APIVersion:         "v1",
				Kind:               "TaskRun",
				Name:               "my-taskrun",
				UID:                "abc-123",
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			},
			b: metav1.OwnerReference{
				APIVersion:         "v1",
				Kind:               "TaskRun",
				Name:               "my-taskrun",
				UID:                "abc-123",
				Controller:         ptr.To(true),
				BlockOwnerDeletion: ptr.To(true),
			},
			want: true,
		},
		{
			name: "both BlockOwnerDeletion nil",
			a: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
			},
			b: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "TaskRun",
				Name:       "my-taskrun",
				UID:        "abc-123",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ownerRefsAreEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("ownerRefsAreEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
