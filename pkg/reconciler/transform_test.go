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
		name        string
		obj         interface{}
		expectError bool
		expectMF    bool
	}{
		{
			name: "Pod with managed fields",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
					ManagedFields: []metav1.ManagedFieldsEntry{
						{
							Manager:   "controller",
							Operation: metav1.ManagedFieldsOperationApply,
						},
					},
				},
			},
			expectError: false,
			expectMF:    false,
		},
		{
			name: "Pod without managed fields",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			expectError: false,
			expectMF:    false,
		},
		{
			name:        "Non-ObjectMetaAccessor object",
			obj:         "not an object",
			expectError: false,
			expectMF:    false,
		},
		{
			name:        "Nil object",
			obj:         nil,
			expectError: false,
			expectMF:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StripManagedFields(tt.obj)

			if (err != nil) != tt.expectError {
				t.Errorf("StripManagedFields() error = %v, expectError = %v", err, tt.expectError)
			}

			if err == nil && result != nil {
				if accessor, ok := result.(metav1.ObjectMetaAccessor); ok {
					managedFields := accessor.GetObjectMeta().GetManagedFields()
					if len(managedFields) != 0 && !tt.expectMF {
						t.Errorf("StripManagedFields() expected ManagedFields to be empty, got %d entries", len(managedFields))
					}
				}
			}
		})
	}
}
