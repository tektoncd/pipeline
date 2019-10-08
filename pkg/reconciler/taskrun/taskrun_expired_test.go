package taskrun

import (
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	apispipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilpointer "k8s.io/utils/pointer"
	"knative.dev/pkg/apis"
	"testing"
)

func newTaskRun(completionTime apis.VolatileTime, ttl *int64) *apispipeline.TaskRun {
	tr := tb.TaskRun("test-pipeline-run-with-annotations-hello-world-1-9l9zj", "foo",
		tb.TaskRunOwnerReference("PipelineRun", "test-pipeline-run-with-annotations",
			tb.OwnerReferenceAPIVersion("tekton.dev/v1alpha1"),
			tb.Controller, tb.BlockOwnerDeletion,
		),
		tb.TaskRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.TaskRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "hello-world-1"),
		tb.TaskRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-annotations"),
		tb.TaskRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.TaskRunSpec(
			tb.TaskRunTaskRef("hello-world"),
			tb.TaskRunServiceAccount("test-sa"),
		),
	)

	if !completionTime.Inner.IsZero() {
		c := apis.Condition{Type: apis.ConditionSucceeded, Status: v1.ConditionTrue, LastTransitionTime: completionTime}
		tr.Status.Conditions = append(tr.Status.Conditions, c)
	}

	if ttl != nil {
		int64(tr.Spec.ExpirationSecondsTTL.Duration) = *ttl
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
		name           string
		completionTime apis.VolatileTime
		//failedTime       metav1.Time
		ttl              *int64
		since            *time.Time
		expectErr        bool
		expectErrStr     string
		expectedTimeLeft *time.Duration
	}{
		{
			name:         "Error case: TaskRun unfinished",
			ttl:          utilpointer.Int64Ptr(10),
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "should not be cleaned up",
		},
		{
			name:           "Error case: TaskRun completed now, no TTL",
			completionTime: now,
			since:          &now.Inner.Time,
			expectErr:      true,
			expectErrStr:   "should not be cleaned up",
		},
		{
			name:             "TaskRun completed now, 0s TTL",
			completionTime:   now,
			ttl:              utilpointer.Int64Ptr(0),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name:             "TaskRun completed now, 10s TTL",
			completionTime:   now,
			ttl:              utilpointer.Int64Ptr(10),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name:             "TaskRun completed 10s ago, 15s TTL",
			completionTime:   apis.VolatileTime{Inner: metav1.NewTime(now.Inner.Add(-10 * time.Second))},
			ttl:              utilpointer.Int64Ptr(15),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
		{
			name: "Error case: TaskRun failed now, no TTL",
			//failedTime:   now,
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "should not be cleaned up",
		},
		{
			name: "TaskRun failed now, 0s TTL",
			//failedTime:       now,
			ttl:              utilpointer.Int64Ptr(0),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name: "TaskRun failed now, 10s TTL",
			//failedTime:       now,
			ttl:              utilpointer.Int64Ptr(10),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name: "TaskRun failed 10s ago, 15s TTL",
			//failedTime:       metav1.NewTime(now.Add(-10 * time.Second)),
			ttl:              utilpointer.Int64Ptr(15),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
	}
	for _, tc := range testCases {
		tr := newTaskRun(tc.completionTime, tc.ttl)
		gotTrTimeLeft, gotTrErr := trTimeLeft(tr, tc.since)

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
