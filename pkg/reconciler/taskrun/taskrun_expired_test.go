package taskrun

import (
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	apispipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"testing"
)

func newTaskRun(completionTime, failedTime apis.VolatileTime, ttl *metav1.Duration) *apispipeline.TaskRun {
	tr := tb.TaskRun("test-task-run-with-expiration-ttl", "foo",
		//tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-with-annotations",
		//	tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
		//	tb.Controller, tb.BlockOwnerDeletion,
		//),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "hello-world-1"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-annotations"),
		tb.TaskRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccountName("test-sa"),
		),
	)

	if !completionTime.Inner.IsZero() {
		c := apis.Condition{Type: apis.ConditionSucceeded, Status: v1.ConditionTrue, LastTransitionTime: completionTime}
		tr.Status.Conditions = append(tr.Status.Conditions, c)
	}

	if !failedTime.Inner.IsZero() {
		c := apis.Condition{Type: apis.ConditionSucceeded, Status: v1.ConditionFalse, LastTransitionTime: failedTime}
		tr.Status.Conditions = append(tr.Status.Conditions, c)
	}

	if ttl != nil {
		tr.Spec.ExpirationSecondsTTL = ttl
	}

	return tr
}

func durationPointer(n int) *time.Duration {
	s := time.Duration(n) * time.Second
	return &s
}

func TestTimeLeft(t *testing.T) {
	now := apis.VolatileTime{Inner: metav1.Now()}

	testCases := []struct {
		name             string
		completionTime   apis.VolatileTime
		failedTime       apis.VolatileTime
		ttl              *metav1.Duration
		since            *time.Time
		expectErr        bool
		expectErrStr     string
		expectedTimeLeft *time.Duration
	}{
		{
			name:         "Error case: TaskRun unfinished",
			ttl:          &metav1.Duration{Duration: 10 * time.Second},
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "TaskRun foo/test-task-run-with-expiration-ttl should not be cleaned up",
		},
		{
			name:           "Error case: TaskRun completed now, no TTL",
			completionTime: now,
			since:          &now.Inner.Time,
			expectErr:      true,
			expectErrStr:   "TaskRun foo/test-task-run-with-expiration-ttl should not be cleaned up",
		},
		{
			name:             "TaskRun completed now, 0s TTL",
			completionTime:   now,
			ttl:              &metav1.Duration{Duration: 0 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name:             "TaskRun completed now, 10s TTL",
			completionTime:   now,
			ttl:              &metav1.Duration{Duration: 10 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name:             "TaskRun completed 10s ago, 15s TTL",
			completionTime:   apis.VolatileTime{Inner: metav1.NewTime(now.Inner.Add(-10 * time.Second))},
			ttl:              &metav1.Duration{Duration: 15 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
		{
			name:         "Error case: TaskRun failed now, no TTL",
			failedTime:   now,
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "TaskRun foo/test-task-run-with-expiration-ttl should not be cleaned up",
		},
		{
			name:             "TaskRun failed now, 0s TTL",
			failedTime:       now,
			ttl:              &metav1.Duration{Duration: 0 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name:             "TaskRun failed now, 10s TTL",
			failedTime:       now,
			ttl:              &metav1.Duration{Duration: 10 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name:             "TaskRun failed 10s ago, 15s TTL",
			failedTime:       apis.VolatileTime{Inner: metav1.NewTime(now.Inner.Add(-10 * time.Second))},
			ttl:              &metav1.Duration{Duration: 15 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
	}
	for _, tc := range testCases {
		tr := newTaskRun(tc.completionTime, tc.failedTime, tc.ttl)
		ts := tb.Task("hello-world", "foo")

		d := test.Data{
			TaskRuns: []*apispipeline.TaskRun{tr},
			Tasks:    []*apispipeline.Task{ts},
		}
		testAssets, cancel := getTaskRunController(t, d)
		defer cancel()
		c, ok := testAssets.Controller.Reconciler.(*Reconciler)
		if !ok {
			t.Errorf("failed to construct instance of taskrun reconciler")
			return
		}

		// Prevent backoff timer from starting
		c.timeoutHandler.SetTaskRunCallbackFunc(nil)

		gotTrTimeLeft, gotTrErr := c.trTimeLeft(tr, tc.since)

		if tc.expectErr != (gotTrErr != nil) {
			t.Errorf("%s: expected error is %t, got %t, error: %v", tc.name, tc.expectErr, gotTrErr != nil, gotTrErr)
		}
		if tc.expectErr && len(tc.expectErrStr) == 0 {
			t.Errorf("%s: invalid test setup; error message must not be empty for error cases", tc.name)
		}
		if tc.expectErr && !strings.Contains(gotTrErr.Error(), tc.expectErrStr) {
			t.Errorf("%s: expected error message contains %q, got %v", tc.name, tc.expectErrStr, gotTrErr)
		}
		if !tc.expectErr {
			if *gotTrTimeLeft != *tc.expectedTimeLeft {
				t.Errorf("%s: expected time left %v, got %v", tc.name, tc.expectedTimeLeft, gotTrTimeLeft)
			}
		}
	}
}
