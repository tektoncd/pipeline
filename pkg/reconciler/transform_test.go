/*
Copyright 2025 The Tekton Authors

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

package reconciler_test

import (
	"testing"

	reconciler "github.com/tektoncd/pipeline/pkg/reconciler"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStripManagedFields(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
	}{
		{
			name: "strips managedFields from TaskRun",
			obj: &v1.TaskRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-taskrun",
					Namespace: "default",
					Labels:    map[string]string{"app": "test"},
					ManagedFields: []metav1.ManagedFieldsEntry{
						{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
					},
				},
			},
		},
		{
			name: "strips managedFields from PipelineRun",
			obj: &v1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-pipelinerun",
					Namespace: "default",
					ManagedFields: []metav1.ManagedFieldsEntry{
						{Manager: "tekton-pipelines-controller", Operation: metav1.ManagedFieldsOperationUpdate},
						{Manager: "kubectl", Operation: metav1.ManagedFieldsOperationApply},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reconciler.StripManagedFields(tt.obj)
			if err != nil {
				t.Fatalf("StripManagedFields() returned error: %v", err)
			}
			accessor, ok := result.(metav1.ObjectMetaAccessor)
			if !ok {
				t.Fatal("result does not implement ObjectMetaAccessor")
			}
			if mf := accessor.GetObjectMeta().GetManagedFields(); mf != nil {
				t.Errorf("ManagedFields = %v, want nil", mf)
			}
			// Verify other metadata is preserved.
			meta := accessor.GetObjectMeta()
			if meta.GetName() == "" {
				t.Error("Name was cleared, expected it to be preserved")
			}
			if meta.GetNamespace() != "default" {
				t.Errorf("Namespace = %q, want %q", meta.GetNamespace(), "default")
			}
		})
	}
}

func TestStripManagedFields_NonAccessor(t *testing.T) {
	// A plain string does not implement ObjectMetaAccessor; it should pass
	// through unchanged without error.
	obj := "not-a-k8s-object"
	result, err := reconciler.StripManagedFields(obj)
	if err != nil {
		t.Fatalf("StripManagedFields() returned error: %v", err)
	}
	if result != obj {
		t.Errorf("result = %v, want original object %v", result, obj)
	}
}
