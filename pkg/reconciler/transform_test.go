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

package reconciler

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStripManagedFields(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
	}{
		{
			name: "strips managedFields from Pod",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					Labels:    map[string]string{"app": "test"},
					ManagedFields: []metav1.ManagedFieldsEntry{
						{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
						{Manager: "kubelet", Operation: metav1.ManagedFieldsOperationUpdate},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StripManagedFields(tt.obj)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			accessor, ok := result.(metav1.ObjectMetaAccessor)
			if !ok {
				t.Fatal("result does not implement ObjectMetaAccessor")
			}

			meta := accessor.GetObjectMeta()
			if meta.GetManagedFields() != nil {
				t.Error("managedFields should be nil after transform")
			}
			if meta.GetName() != "test-pod" {
				t.Errorf("name should be preserved, got %q", meta.GetName())
			}
			if meta.GetNamespace() != "default" {
				t.Errorf("namespace should be preserved, got %q", meta.GetNamespace())
			}
			if meta.GetLabels()["app"] != "test" {
				t.Error("labels should be preserved")
			}
		})
	}
}

func TestStripManagedFieldsNonAccessor(t *testing.T) {
	input := "not-a-kubernetes-object"
	result, err := StripManagedFields(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Error("non-accessor objects should pass through unchanged")
	}
}
