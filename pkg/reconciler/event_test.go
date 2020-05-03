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

package reconciler

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
)

func TestEmitEvent(t *testing.T) {
	testcases := []struct {
		name          string
		before        *apis.Condition
		after         *apis.Condition
		expectEvent   bool
		expectedEvent string
	}{{
		name: "unknown to true with message",
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionTrue,
			Message: "all done",
		},
		expectEvent:   true,
		expectedEvent: "Normal Succeeded all done",
	}, {
		name: "true to true",
		before: &apis.Condition{
			Type:               apis.ConditionSucceeded,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now())},
		},
		after: &apis.Condition{
			Type:               apis.ConditionSucceeded,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: apis.VolatileTime{Inner: metav1.NewTime(time.Now().Add(5 * time.Minute))},
		},
		expectEvent:   false,
		expectedEvent: "",
	}, {
		name: "false to false",
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		},
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		},
		expectEvent:   false,
		expectedEvent: "",
	}, {
		name: "unknown to unknown",
		before: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  "",
			Message: "",
		},
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Reason:  "foo",
			Message: "bar",
		},
		expectEvent:   true,
		expectedEvent: "Normal foo bar",
	}, {
		name:  "true to nil",
		after: nil,
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		expectEvent:   false,
		expectedEvent: "",
	}, {
		name:   "nil to true",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		expectEvent:   true,
		expectedEvent: "Normal Succeeded ",
	}, {
		name:   "nil to unknown with message",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Message: "just starting",
		},
		expectEvent:   true,
		expectedEvent: "Normal Started ",
	}, {
		name: "unknown to false with message",
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
		},
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: "really bad",
		},
		expectEvent:   true,
		expectedEvent: "Warning Failed really bad",
	}, {
		name:   "nil to false",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		},
		expectEvent:   true,
		expectedEvent: "Warning Failed ",
	}}

	for _, ts := range testcases {
		fr := record.NewFakeRecorder(1)
		tr := &corev1.Pod{}
		EmitEvent(fr, ts.before, ts.after, tr)
		timer := time.NewTimer(1 * time.Second)

		select {
		case event := <-fr.Events:
			if event == "" {
				// The fake recorder reported empty, it should not happen
				t.Fatalf("Expected event but got empty for %s", ts.name)
			}
			if !ts.expectEvent {
				// The fake recorder reported an event which we did not expect
				t.Errorf("Unxpected event \"%s\" but got one for %s", event, ts.name)
			}
			if !(event == ts.expectedEvent) {
				t.Errorf("Expected event \"%s\" but got \"%s\" instead for %s", ts.expectedEvent, event, ts.name)
			}
		case <-timer.C:
			if ts.expectEvent {
				// The fake recorder did not report, the timer timeout expired
				t.Errorf("Expected event but got none for %s", ts.name)
			}
		}
	}
}
