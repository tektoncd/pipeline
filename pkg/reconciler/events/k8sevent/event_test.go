/*
Copyright 2022 The Tekton Authors

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

package k8sevent_test

import (
	"errors"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	k8sevents "github.com/tektoncd/pipeline/pkg/reconciler/events/k8sevent"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestEmitK8sEventsOnConditions(t *testing.T) {
	testcases := []struct {
		name       string
		before     *apis.Condition
		after      *apis.Condition
		wantEvents []string
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
		wantEvents: []string{"Normal Succeeded all done"},
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
		wantEvents: []string{},
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
		wantEvents: []string{},
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
		wantEvents: []string{"Normal foo bar"},
	}, {
		name:  "true to nil",
		after: nil,
		before: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		wantEvents: []string{},
	}, {
		name:   "nil to true",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		},
		wantEvents: []string{"Normal Succeeded "},
	}, {
		name:   "nil to unknown with message",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionUnknown,
			Message: "just starting",
		},
		wantEvents: []string{"Normal Started "},
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
		wantEvents: []string{"Warning Failed really bad"},
	}, {
		name:   "nil to false",
		before: nil,
		after: &apis.Condition{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionFalse,
		},
		wantEvents: []string{"Warning Failed "},
	}, {
		name:   "match wildcard events through escaping",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: "contains a * character",
		},
		wantEvents: []string{"Warning Failed contains a \\* character"},
	}, {
		name:   "match wildcard events literally",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: "contains a * character",
		},
		wantEvents: []string{"Warning Failed contains a * character"},
	}, {
		name:   "match contains parenthesis events through escaping",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: "contains (parenthesis)",
		},
		wantEvents: []string{"Warning Failed contains \\(parenthesis\\)"},
	}, {
		name:   "match contains parenthesis events through literally",
		before: nil,
		after: &apis.Condition{
			Type:    apis.ConditionSucceeded,
			Status:  corev1.ConditionFalse,
			Message: "contains (parenthesis)",
		},
		wantEvents: []string{"Warning Failed contains (parenthesis)"},
	}}

	for _, ts := range testcases {
		tr := &corev1.Pod{}
		ctx, _ := rtesting.SetupFakeContext(t)
		recorder := controller.GetEventRecorder(ctx).(*record.FakeRecorder)
		k8sevents.EmitK8sEvents(ctx, ts.before, ts.after, tr)
		err := k8sevents.CheckEventsOrdered(t, recorder.Events, ts.name, ts.wantEvents)
		if err != nil {
			t.Errorf(err.Error())
		}
	}
}

func TestEmitK8sEvents(t *testing.T) {
	objectStatus := duckv1.Status{
		Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1.PipelineRunReasonStarted.String(),
		}},
	}
	object := &v1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink: "/pipelineruns/test1",
		},
		Status: v1.PipelineRunStatus{Status: objectStatus},
	}
	after := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Message: "just starting",
	}
	testcases := []struct {
		name       string
		data       map[string]string
		wantEvents []string
	}{{
		name:       "without sink",
		data:       map[string]string{},
		wantEvents: []string{"Normal Started"},
	}, {
		name:       "with empty string sink",
		data:       map[string]string{"default-cloud-events-sink": ""},
		wantEvents: []string{"Normal Started"},
	}, {
		name:       "with sink",
		data:       map[string]string{"default-cloud-events-sink": "http://mysink"},
		wantEvents: []string{"Normal Started"},
	}}

	for _, tc := range testcases {
		// Setup the context and seed test data
		ctx, _ := rtesting.SetupFakeContext(t)

		// Setup the config and add it to the context
		defaults, _ := config.NewDefaultsFromMap(tc.data)
		cfg := &config.Config{
			Defaults:     defaults,
			FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
		}
		ctx = config.ToContext(ctx, cfg)

		recorder := controller.GetEventRecorder(ctx).(*record.FakeRecorder)
		k8sevents.EmitK8sEvents(ctx, nil, after, object)
		if err := k8sevents.CheckEventsOrdered(t, recorder.Events, tc.name, tc.wantEvents); err != nil {
			t.Fatalf(err.Error())
		}
	}
}

func TestEmitError(t *testing.T) {
	testcases := []struct {
		name       string
		err        error
		wantEvents []string
	}{{
		name:       "with error",
		err:        errors.New("something went wrong"),
		wantEvents: []string{"Warning Error something went wrong"},
	}, {
		name:       "without error",
		err:        nil,
		wantEvents: []string{},
	}}

	for _, ts := range testcases {
		fr := record.NewFakeRecorder(1)
		tr := &corev1.Pod{}
		k8sevents.EmitError(fr, ts.err, tr)
		err := k8sevents.CheckEventsOrdered(t, fr.Events, ts.name, ts.wantEvents)
		if err != nil {
			t.Errorf(err.Error())
		}
	}
}
