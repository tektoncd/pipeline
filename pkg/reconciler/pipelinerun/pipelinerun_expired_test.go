package pipelinerun

import (
	"github.com/tektoncd/pipeline/test"
	"strings"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	apispipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"testing"
)

func newPipelineRun(completionTime, failedTime apis.VolatileTime, ttl *metav1.Duration) *apispipeline.PipelineRun {
	pr := tb.PipelineRun("test-pipeline-run-with-expiration-ttl", "foo",
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

	if !failedTime.Inner.IsZero() {
		c := apis.Condition{Type: apis.ConditionSucceeded, Status: v1.ConditionFalse, LastTransitionTime: failedTime}
		pr.Status.Conditions = append(pr.Status.Conditions, c)
	}

	if ttl != nil {
		pr.Spec.ExpirationSecondsTTL = ttl
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
		failedTime       apis.VolatileTime
		ttl              *metav1.Duration
		since            *time.Time
		expectErr        bool
		expectErrStr     string
		expectedTimeLeft *time.Duration
	}{
		{
			name:         "Error case: PipelineRun unfinished",
			ttl:          &metav1.Duration{Duration: 10 * time.Second},
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "PipelineRun foo/test-pipeline-run-with-expiration-ttl should not be cleaned up",
		},
		{
			name:           "Error case: PipelineRun completed now, no TTL",
			completionTime: now,
			since:          &now.Inner.Time,
			expectErr:      true,
			expectErrStr:   "PipelineRun foo/test-pipeline-run-with-expiration-ttl should not be cleaned up",
		},
		{
			name:             "PipelineRun completed now, 0s TTL",
			completionTime:   now,
			ttl:              &metav1.Duration{Duration: 0 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name:             "PipelineRun completed now, 10s TTL",
			completionTime:   now,
			ttl:              &metav1.Duration{Duration: 10 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name:             "PipelineRun completed 10s ago, 15s TTL",
			completionTime:   apis.VolatileTime{Inner: metav1.NewTime(now.Inner.Add(-10 * time.Second))},
			ttl:              &metav1.Duration{Duration: 15 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
		{
			name:         "Error case: PipelineRun failed now, no TTL",
			failedTime:   now,
			since:        &now.Inner.Time,
			expectErr:    true,
			expectErrStr: "PipelineRun foo/test-pipeline-run-with-expiration-ttl should not be cleaned up",
		},
		{
			name:             "PipelineRun failed now, 0s TTL",
			failedTime:       now,
			ttl:              &metav1.Duration{Duration: 0 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(0),
		},
		{
			name:             "PipelineRun failed now, 10s TTL",
			failedTime:       now,
			ttl:              &metav1.Duration{Duration: 10 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(10),
		},
		{
			name:             "PipelineRun failed 10s ago, 15s TTL",
			failedTime:       apis.VolatileTime{Inner: metav1.NewTime(now.Inner.Add(-10 * time.Second))},
			ttl:              &metav1.Duration{Duration: 15 * time.Second},
			since:            &now.Inner.Time,
			expectedTimeLeft: durationPointer(5),
		},
	}
	for _, tc := range PrTestCases {
		pr := newPipelineRun(tc.completionTime, tc.failedTime, tc.ttl)
		d := test.Data{
			PipelineRuns: []*apispipeline.PipelineRun{pr},
		}
		testAssets, cancel := getPipelineRunController(t, d)
		defer cancel()
		p, ok := testAssets.Controller.Reconciler.(*Reconciler)
		if !ok {
			t.Errorf("failed to construct instance of taskrun reconciler")
			return
		}

		// Prevent backoff timer from starting
		p.timeoutHandler.SetPipelineRunCallbackFunc(nil)
		gotPrTimeLeft, gotPrErr := p.prTimeLeft(pr, tc.since)

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
