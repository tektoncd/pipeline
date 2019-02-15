package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineRunStatusConditions(t *testing.T) {
	p := &PipelineRun{}
	foo := &duckv1alpha1.Condition{
		Type:   "Foo",
		Status: "True",
	}
	bar := &duckv1alpha1.Condition{
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
		APIVersion: "build-tekton.dev/v1alpha1",
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

	p.Status.TaskRuns["fooTask"] = TaskRunStatus{}

	p.Status.InitializeConditions()
	if len(p.Status.TaskRuns) != 1 {
		t.Fatalf("PipelineRun status getting reset")
	}
}
