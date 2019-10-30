package pipelinerun

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

func newPipelineRun(completionTime apis.VolatileTime, ttl *int64) *apispipeline.PipelineRun {
	pr := tb.PipelineRun("test-pipeline-run-with-annotations-hello-world-1-9l9zj", "foo",
		tb.PipelineRunLabel("tekton.dev/pipeline", "test-pipeline"),
		tb.PipelineRunLabel(pipeline.GroupName+pipeline.PipelineTaskLabelKey, "hello-world-1"),
		tb.PipelineRunLabel("tekton.dev/pipelineRun", "test-pipeline-run-with-annotations"),
		tb.PipelineRunAnnotation("PipelineRunAnnotation", "PipelineRunValue"),
		tb.PipelineRunSpec(
			"hello-world",
			tb.PipelineRunServiceAccountName("test-sa"),
		),
	)

	if !completionTime.Inner.IsZero() {
		c := apis.Condition{Type: apis.ConditionSucceeded, Status: v1.ConditionTrue, LastTransitionTime: completionTime}
		pr.Status.Conditions = append(pr.Status.Conditions, c)
	}

	if ttl != nil {
		int64(pr.Spec.ExpirationSecondsTTL.Duration) = *ttl
	}

	return pr
}

func durationPointer(n int) *time.Duration {
	s := time.Duration(n) * time.Second
	return &s
}

func TestTimeLeft(t *testing.T) {
	now := apis.VolatileTime{Inner: metav1.Now()}

	PrTestCases := []struct {
		name             string
		completionTime   apis.VolatileTime
		failedTime       metav1.Time
		ttl              *int64
		since            *time.Time
		expectErr        bool
		expectErrStr     string
		expectedTimeLeft *time.Duration
	}{
		{
			name:         "Error case: PipelineRun unfinished",
			ttl:          utilpointer.Int64Ptr(10),
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "should not be cleaned up",
		},
		{
			name:           "Error case: PipelineRun completed now, no TTL",
			completionTime: now,
			since:          &now.Inner.Time,
			expectErr:      true,
			expectErrStr:   "should not be cleaned up",
		},
		{
			name:             "PipelineRun completed now, 0s TTL",
			completionTime:   now,
			ttl:              utilpointer.Int64Ptr(0),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name:             "PipelineRun completed now, 10s TTL",
			completionTime:   now,
			ttl:              utilpointer.Int64Ptr(10),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name:             "PipelineRun completed 10s ago, 15s TTL",
			completionTime:   apis.VolatileTime{Inner: metav1.NewTime(now.Inner.Add(-10 * time.Second))},
			ttl:              utilpointer.Int64Ptr(15),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
		{
			name: "Error case: PipelineRun failed now, no TTL",
			//failedTime:   now,
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "should not be cleaned up",
		},
		{
			name: "PipelineRun failed now, 0s TTL",
			//failedTime:       now,
			ttl:              utilpointer.Int64Ptr(0),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name: "PipelineRun failed now, 10s TTL",
			//failedTime:       now,
			ttl:              utilpointer.Int64Ptr(10),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name: "PipelineRun failed 10s ago, 15s TTL",
			//failedTime:       metav1.NewTime(now.Add(-10 * time.Second)),
			ttl:              utilpointer.Int64Ptr(15),
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
	}
	for _, tc := range PrTestCases {
		pr := newPipelineRun(tc.completionTime, tc.ttl)
		reconcile := Reconciler{}
		gotPrTimeLeft, gotPrErr := reconcile.prTimeLeft(pr, tc.since)

		if tc.expectErr != (gotPrErr != nil) {
			t.Errorf("%s: expected error is %t, got %t, error: %v", tc.name, tc.expectErr, gotPrErr != nil, gotPrErr)
		}
		if tc.expectErr && len(tc.expectErrStr) == 0 {
			t.Errorf("%s: invalid test setup; error message must not be empty for error cases", tc.name)
		}
		if tc.expectErr && !strings.Contains(gotPrErr.Error(), tc.expectErrStr) {
			t.Errorf("%s: expected error message contains %q, got %v", tc.name, tc.expectErrStr, gotPrErr)
		}
		if !tc.expectErr {
			if *gotPrTimeLeft != *tc.expectedTimeLeft {
				t.Errorf("%s: expected time left %v, got %v", tc.name, tc.expectedTimeLeft, gotPrTimeLeft)
			}
		}
	}
}
