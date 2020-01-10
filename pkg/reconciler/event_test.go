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
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
)

func TestEmitEvent(t *testing.T) {
	testcases := []struct {
		name        string
		before      *apis.Condition
		after       *apis.Condition
		expectEvent bool
	}{{
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
	}, {
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
		expectEvent: false,
	}, {
		name:  "true to nil",
		after: nil,
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		expectEvent: true,
	}, {
		name:   "nil to true",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		expectEvent: true,
	}}

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
