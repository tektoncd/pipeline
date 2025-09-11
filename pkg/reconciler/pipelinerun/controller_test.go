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

package pipelinerun

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunFilterManagedBy(t *testing.T) {
	prManaged := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-managed",
		},
		Spec: v1.PipelineRunSpec{
			ManagedBy: &[]string{pipeline.ManagedBy}[0],
		},
	}
	prNotManaged := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-not-managed",
		},
		Spec: v1.PipelineRunSpec{
			ManagedBy: &[]string{"some-other-controller"}[0],
		},
	}
	prNilManagedBy := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-nil-managed-by",
		},
		Spec: v1.PipelineRunSpec{},
	}
	notAPipelineRun := "not-a-pipelinerun"

	testCases := []struct {
		name     string
		obj      interface{}
		expected bool
	}{
		{
			name:     "managed by tekton controller",
			obj:      prManaged,
			expected: true,
		},
		{
			name:     "not managed by tekton controller",
			obj:      prNotManaged,
			expected: false,
		},
		{
			name:     "nil managed by",
			obj:      prNilManagedBy,
			expected: true,
		},
		{
			name:     "not a pipelinerun",
			obj:      notAPipelineRun,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := pipelineRunFilterManagedBy(tc.obj)
			if result != tc.expected {
				t.Errorf("Expected %v, but got %v", tc.expected, result)
			}
		})
	}
}
