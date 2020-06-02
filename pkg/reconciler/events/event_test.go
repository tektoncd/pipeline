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

package events

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
)

func TestEmit(t *testing.T) {
	testcases := []struct {
		name      string
		before    *apis.Condition
		after     *apis.Condition
		wantEvent string
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
		wantEvent: "Normal Succeeded all done",
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
		wantEvent: "",
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
		wantEvent: "",
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
		wantEvent: "Normal foo bar",
	}, {
		name:  "true to nil",
		after: nil,
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		wantEvent: "",
	}, {
		name:   "nil to true",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		wantEvent: "Normal Succeeded ",
	}, {
		name:   "nil to unknown with message",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Message: "just starting",
		},
		wantEvent: "Normal Started ",
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
		wantEvent: "Warning Failed really bad",
	}, {
		name:   "nil to false",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		},
		wantEvent: "Warning Failed ",
	}}

	for _, ts := range testcases {
		fr := record.NewFakeRecorder(1)
		tr := &corev1.Pod{}
		Emit(fr, ts.before, ts.after, tr)

		err := checkEvents(t, fr, ts.name, ts.wantEvent)
		if err != nil {
			t.Errorf(err.Error())
		}
	}
}

func TestEmitError(t *testing.T) {
	testcases := []struct {
		name      string
		err       error
		wantEvent string
	}{{
		name:      "with error",
		err:       errors.New("something went wrong"),
		wantEvent: "Warning Error something went wrong",
	}, {
		name:      "without error",
		err:       nil,
		wantEvent: "",
	}}

	for _, ts := range testcases {
		fr := record.NewFakeRecorder(1)
		tr := &corev1.Pod{}
		EmitError(fr, ts.err, tr)

		err := checkEvents(t, fr, ts.name, ts.wantEvent)
		if err != nil {
			t.Errorf(err.Error())
		}
	}
}

func checkEvents(t *testing.T, fr *record.FakeRecorder, testName string, wantEvent string) error {
	t.Helper()
	timer := time.NewTimer(1 * time.Second)
	select {
	case event := <-fr.Events:
		if wantEvent == "" {
			return fmt.Errorf("received event \"%s\" for %s but none expected", event, testName)
		}
		if !(strings.HasPrefix(event, wantEvent)) {
			return fmt.Errorf("expected event \"%s\" but got \"%s\" instead for %s", wantEvent, event, testName)
		}
	case <-timer.C:
		if wantEvent != "" {
			return fmt.Errorf("received no events for %s but %s expected", testName, wantEvent)
		}
	}
	return nil
}
