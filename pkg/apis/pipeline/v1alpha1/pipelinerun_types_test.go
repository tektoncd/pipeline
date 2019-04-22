package v1alpha1

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunStatusConditions(t *testing.T) {
	p := &PipelineRun{}
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
	p := &PipelineRun{
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
	p := &PipelineRun{
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

	p.Status.TaskRuns["fooTask"] = &PipelineRunTaskRunStatus{}

	p.Status.InitializeConditions()
	if len(p.Status.TaskRuns) != 1 {
		t.Fatalf("PipelineRun status getting reset")
	}
}

func TestPipelineRunIsDone(t *testing.T) {
	pr := &PipelineRun{}
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
	pr := &PipelineRun{
		Spec: PipelineRunSpec{
			Status: PipelineRunSpecStatusCancelled,
		},
	}
	if !pr.IsCancelled() {
		t.Fatal("Expected pipelinerun status to be cancelled")
	}
}

func TestPipelineRunKey(t *testing.T) {
	pr := &PipelineRun{
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
		prStatus      PipelineRunStatus
		expectedValue bool
	}{{
		name:          "prWithNoStartTime",
		prStatus:      PipelineRunStatus{},
		expectedValue: false,
	}, {
		name: "prWithStartTime",
		prStatus: PipelineRunStatus{
			StartTime: &metav1.Time{Time: time.Now()},
		},
		expectedValue: true,
	}, {
		name: "prWithZeroStartTime",
		prStatus: PipelineRunStatus{
			StartTime: &metav1.Time{},
		},
		expectedValue: false,
	}}
	for _, tc := range params {
		t.Run(tc.name, func(t *testing.T) {
			pr := &PipelineRun{
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
