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

package errors

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testPipeline = v1alpha1.Pipeline{
	TypeMeta: metav1.TypeMeta{
		Kind:       "foo",
		APIVersion: "tekton.test/v1test1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-pipeline",
	},
}

func TestNewDuplicatePipelineTask(t *testing.T) {
	err := NewDuplicatePipelineTask(&testPipeline, "duplicate")
	expectedDetails := &metav1.StatusDetails{
		Name:  "test-pipeline",
		Group: "tekton.test",
		Kind:  "foo",
		Causes: []metav1.StatusCause{
			{Type: metav1.CauseTypeFieldValueDuplicate,
				Message: `Duplicate value: "duplicate"`,
				Field:   "spec.tasks.name",
			},
		},
	}
	if d := cmp.Diff(expectedDetails, err.Status().Details); d != "" {
		t.Errorf("expected %s, got diff %s", expectedDetails, d)
	}
}

func TestNewPipelineTaskNotFound(t *testing.T) {
	err := NewPipelineTaskNotFound(&testPipeline, "not-found")
	expectedDetails := &metav1.StatusDetails{
		Name:  "test-pipeline",
		Group: "tekton.test",
		Kind:  "foo",
		Causes: []metav1.StatusCause{
			{Type: metav1.CauseTypeFieldValueNotFound,
				Message: `Not found: "not-found"`,
				Field:   "spec.tasks.name",
			},
		},
	}
	if d := cmp.Diff(expectedDetails, err.Status().Details); d != "" {
		t.Errorf("expected %s, got diff %s", expectedDetails, d)
	}
}

func TestNewInvalidPipeline(t *testing.T) {
	err := NewInvalidPipeline(&testPipeline, "cycle found")
	expectedDetails := &metav1.StatusDetails{
		Name:  "test-pipeline",
		Group: "tekton.test",
		Kind:  "foo",
		Causes: []metav1.StatusCause{
			{Type: metav1.CauseType("InternalError"),
				Message: "Internal error: cycle found",
			},
		},
	}
	if d := cmp.Diff(err.Status().Details, expectedDetails); d != "" {
		t.Errorf("expected %s, got diff %s", "", d)
	}
}
