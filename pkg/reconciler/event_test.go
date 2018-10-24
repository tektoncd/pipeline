package reconciler

import (
	"testing"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

func Test_EmitEvent(t *testing.T) {
	testcases := []struct {
		name        string
		before      *duckv1alpha1.Condition
		after       *duckv1alpha1.Condition
		expectEvent bool
	}{
		{
			name: "unknown to true",
			before: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionUnknown,
			},
			after: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: true,
		},
		{
			name: "true to true",
			before: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			after: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: false,
		},
		{
			name: "false to false",
			before: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			},
			after: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			},
			expectEvent: false,
		},
		{
			name:  "true to nil",
			after: nil,
			before: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: true,
		},
		{
			name:   "nil to true",
			before: nil,
			after: &duckv1alpha1.Condition{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: true,
		},
	}

	for _, ts := range testcases {
		fr := record.NewFakeRecorder(1)
		tr := &corev1.Pod{}
		EmitEvent(fr, ts.before, ts.after, tr)
		timer := time.NewTimer(1 * time.Second)

		select {
		case event := <-fr.Events:
			if ts.expectEvent && event == "" {
				t.Errorf("Expected event but got empty for %s", ts.name)
			}
		case <-timer.C:
			if !ts.expectEvent {
				t.Errorf("Unexpected event but got for %s", ts.name)
			}
		}
	}
}
