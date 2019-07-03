/*
Copyright 2019 The Tekton Authors

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

package v1alpha1_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunStatusConditions(t *testing.T) {
	p := &v1alpha1.PipelineRun{}
	foo := &apis.Condition{
		Type:   "Foo",
		Status: "True",
	}
	bar := &apis.Condition{
		Type:   "Bar",
		Status: "True",
	}

	var ignoreVolatileTime = cmp.Comparer(func(_, _ apis.VolatileTime) bool {
		return true
	})

	// Add a new condition.
	p.Status.SetCondition(foo)

	fooStatus := p.Status.GetCondition(foo.Type)
	if d := cmp.Diff(fooStatus, foo, ignoreVolatileTime); d != "" {
		t.Errorf("Unexpected pipeline run condition type; want %v got %v; diff %v", fooStatus, foo, d)
	}

	// Add a second condition.
	p.Status.SetCondition(bar)

	barStatus := p.Status.GetCondition(bar.Type)

	if d := cmp.Diff(barStatus, bar, ignoreVolatileTime); d != "" {
		t.Fatalf("Unexpected pipeline run condition type; want %v got %v; diff %s", barStatus, bar, d)
	}
}

func TestPipelineRun_TaskRunref(t *testing.T) {
	p := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-ns",
		},
	}

	expectTaskRunRef := corev1.ObjectReference{
		APIVersion: "tekton.dev/v1alpha1",
		Kind:       "TaskRun",
		Namespace:  p.Namespace,
		Name:       p.Name,
	}

	if d := cmp.Diff(p.GetTaskRunRef(), expectTaskRunRef); d != "" {
		t.Fatalf("Taskrun reference mismatch; want %v got %v; diff %s", expectTaskRunRef, p.GetTaskRunRef(), d)
	}
}

func TestInitializeConditions(t *testing.T) {
	p := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-name",
			Namespace: "test-ns",
		},
	}
	p.Status.InitializeConditions()

	if p.Status.TaskRuns == nil {
		t.Fatalf("PipelineRun status not initialized correctly")
	}

	if p.Status.StartTime.IsZero() {
		t.Fatalf("PipelineRun StartTime not initialized correctly")
	}

	p.Status.TaskRuns["fooTask"] = &v1alpha1.PipelineRunTaskRunStatus{}

	p.Status.InitializeConditions()
	if len(p.Status.TaskRuns) != 1 {
		t.Fatalf("PipelineRun status getting reset")
	}
}

func TestPipelineRunIsDone(t *testing.T) {
	pr := &v1alpha1.PipelineRun{}
	foo := &apis.Condition{
		Type:   apis.ConditionSucceeded,
		Status: corev1.ConditionFalse,
	}
	pr.Status.SetCondition(foo)
	if !pr.IsDone() {
		t.Fatal("Expected pipelinerun status to be done")
	}
}

func TestPipelineRunIsCancelled(t *testing.T) {
	pr := &v1alpha1.PipelineRun{
		Spec: v1alpha1.PipelineRunSpec{
			Status: v1alpha1.PipelineRunSpecStatusCancelled,
		},
	}
	if !pr.IsCancelled() {
		t.Fatal("Expected pipelinerun status to be cancelled")
	}
}

func TestPipelineRunKey(t *testing.T) {
	pr := &v1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prunname",
			Namespace: "testns",
		},
	}
	expectedKey := "PipelineRun/testns/prunname"
	if pr.GetRunKey() != expectedKey {
		t.Fatalf("Expected taskrun key to be %s but got %s", expectedKey, pr.GetRunKey())
	}
}

func TestPipelineRunHasStarted(t *testing.T) {
	params := []struct {
		name          string
		prStatus      v1alpha1.PipelineRunStatus
		expectedValue bool
	}{{
		name:          "prWithNoStartTime",
		prStatus:      v1alpha1.PipelineRunStatus{},
		expectedValue: false,
	}, {
		name: "prWithStartTime",
		prStatus: v1alpha1.PipelineRunStatus{
			StartTime: &metav1.Time{Time: time.Now()},
		},
		expectedValue: true,
	}, {
		name: "prWithZeroStartTime",
		prStatus: v1alpha1.PipelineRunStatus{
			StartTime: &metav1.Time{},
		},
		expectedValue: false,
	}}
	for _, tc := range params {
		t.Run(tc.name, func(t *testing.T) {
			pr := &v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "prunname",
					Namespace: "testns",
				},
				Status: tc.prStatus,
			}
			if pr.HasStarted() != tc.expectedValue {
				t.Fatalf("Expected pipelinerun HasStarted() to return %t but got %t", tc.expectedValue, pr.HasStarted())
			}
		})
	}
}
