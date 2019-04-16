package reconciler

import (
	"testing"
	"time"

	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

func Test_EmitEvent(t *testing.T) {
	testcases := []struct {
		name        string
		before      *apis.Condition
		after       *apis.Condition
		expectEvent bool
	}{
		{
			name: "unknown to true",
			before: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionUnknown,
			},
			after: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: true,
		},
		{
			name: "true to true",
			before: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			after: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: false,
		},
		{
			name: "false to false",
			before: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			},
			after: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionFalse,
			},
			expectEvent: false,
		},
		{
			name:  "true to nil",
			after: nil,
			before: &apis.Condition{
				Type:   apis.ConditionSucceeded,
				Status: corev1.ConditionTrue,
			},
			expectEvent: true,
		},
		{
			name:   "nil to true",
			before: nil,
			after: &apis.Condition{
				Type:   apis.ConditionSucceeded,
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
