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

package taskrun

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTaskRunFilterManagedBy(t *testing.T) {
	trManaged := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskrun-managed",
		},
		Spec: v1.TaskRunSpec{
			ManagedBy: &[]string{pipeline.ManagedBy}[0],
		},
	}
	trNotManaged := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskrun-not-managed",
		},
		Spec: v1.TaskRunSpec{
			ManagedBy: &[]string{"some-other-controller"}[0],
		},
	}
	trNilManagedBy := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "taskrun-nil-managed-by",
		},
		Spec: v1.TaskRunSpec{},
	}
	notATaskRun := "not-a-taskrun"

	testCases := []struct {
		name     string
		obj      interface{}
		expected bool
	}{
		{
			name:     "managed by tekton controller",
			obj:      trManaged,
			expected: true,
		},
		{
			name:     "not managed by tekton controller",
			obj:      trNotManaged,
			expected: false,
		},
		{
			name:     "nil managed by",
			obj:      trNilManagedBy,
			expected: true,
		},
		{
			name:     "not a taskrun",
			obj:      notATaskRun,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := taskRunFilterManagedBy(tc.obj)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}
