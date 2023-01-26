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

package cloudevent

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/k8sevent"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestSendCloudEventWithRetries(t *testing.T) {
	objectStatus := duckv1.Status{
		Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionTrue,
		}},
	}

	tests := []struct {
		name            string
		clientBehaviour FakeClientBehaviour
		object          objectWithCondition
		wantCEvents     []string
		wantEvents      []string
	}{{
		name: "test-send-cloud-event-taskrun",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: true,
		},
		object: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				SelfLink: "/taskruns/test1",
			},
			Status: v1beta1.TaskRunStatus{Status: objectStatus},
		},
		wantCEvents: []string{"Context Attributes,"},
		wantEvents:  []string{},
	}, {
		name: "test-send-cloud-event-pipelinerun",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: true,
		},
		object: &v1beta1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				SelfLink: "/pipelineruns/test1",
			},
			Status: v1beta1.PipelineRunStatus{Status: objectStatus},
		},
		wantCEvents: []string{"Context Attributes,"},
		wantEvents:  []string{},
	}, {
		name: "test-send-cloud-event-failed",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: false,
		},
		object: &v1beta1.PipelineRun{
			Status: v1beta1.PipelineRunStatus{Status: objectStatus},
		},
		wantCEvents: []string{},
		wantEvents:  []string{"Warning Cloud Event Failure"},
	}, {
		name: "test-send-cloud-event-run",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: true,
		},
		object:      &v1alpha1.Run{},
		wantCEvents: []string{"Context Attributes,"},
		wantEvents:  []string{},
	}, {
		name: "test-send-cloud-event-customrun",
		clientBehaviour: FakeClientBehaviour{
			SendSuccessfully: true,
		},
		object:      &v1beta1.CustomRun{},
		wantCEvents: []string{"Context Attributes,"},
		wantEvents:  []string{},
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupFakeContext(t, tc.clientBehaviour, true, len(tc.wantCEvents))
			if err := SendCloudEventWithRetries(ctx, tc.object); err != nil {
				t.Fatalf("Unexpected error sending cloud events: %v", err)
			}
			ceClient := Get(ctx).(FakeClient)
			ceClient.CheckCloudEventsUnordered(t, tc.name, tc.wantCEvents)
			recorder := controller.GetEventRecorder(ctx).(*k8sevent.FakeRecorder)
			if err := recorder.CheckEventsOrdered(t, recorder.Events, tc.name, tc.wantEvents); err != nil {
				t.Fatalf(err.Error())
			}
		})
	}
}

func TestSendCloudEventWithRetriesInvalid(t *testing.T) {
	tests := []struct {
		name       string
		object     objectWithCondition
		wantCEvent string
		wantEvent  string
	}{{
		name: "test-send-cloud-event-invalid-taskrun",
		object: &v1beta1.TaskRun{
			Status: v1beta1.TaskRunStatus{},
		},
		wantCEvent: "Context Attributes,",
		wantEvent:  "",
	}, {
		name: "test-send-cloud-event-pipelinerun",
		object: &v1beta1.PipelineRun{
			Status: v1beta1.PipelineRunStatus{},
		},
		wantCEvent: "Context Attributes,",
		wantEvent:  "",
	}}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupFakeContext(t, FakeClientBehaviour{
				SendSuccessfully: true,
			}, true, 1)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			err := SendCloudEventWithRetries(ctx, tc.object)
			if err == nil {
				t.Fatalf("Expected an error sending cloud events for invalid object, got none")
			}
		})
	}
}

func TestSendCloudEventWithRetriesNoClient(t *testing.T) {
	ctx := setupFakeContext(t, FakeClientBehaviour{}, false, 0)
	err := SendCloudEventWithRetries(ctx, &v1beta1.TaskRun{Status: v1beta1.TaskRunStatus{}})
	if err == nil {
		t.Fatalf("Expected an error sending cloud events with no client in the context, got none")
	}
	if d := cmp.Diff("No cloud events client found in the context", err.Error()); d != "" {
		t.Fatalf("Unexpected error message %s", diff.PrintWantGot(d))
	}
}

func TestEmitCloudEvents(t *testing.T) {
	object := &v1alpha1.Run{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink: "/run/test1",
		},
		Status: v1alpha1.RunStatus{},
	}
	testcases := []struct {
		name            string
		data            map[string]string
		wantEvents      []string
		wantCloudEvents []string
	}{{
		name:            "without sink",
		data:            map[string]string{},
		wantEvents:      []string{},
		wantCloudEvents: []string{},
	}, {
		name:            "with empty string sink",
		data:            map[string]string{"default-cloud-events-sink": ""},
		wantEvents:      []string{},
		wantCloudEvents: []string{},
	}, {
		name:            "with sink",
		data:            map[string]string{"default-cloud-events-sink": "http://mysink"},
		wantEvents:      []string{},
		wantCloudEvents: []string{`(?s)dev.tekton.event.run.started.v1.*test1`},
	}}

	for _, tc := range testcases {
		// Setup the context and seed test data
		ctx, _ := k8sevent.SetupFakeContext(t)
		ctx = WithClient(ctx, &FakeClientBehaviour{SendSuccessfully: true}, len(tc.wantCloudEvents))
		fakeClient := Get(ctx).(FakeClient)

		// Setup the config and add it to the context
		defaults, _ := config.NewDefaultsFromMap(tc.data)
		featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{})
		cfg := &config.Config{
			Defaults:     defaults,
			FeatureFlags: featureFlags,
		}
		ctx = config.ToContext(ctx, cfg)

		recorder := controller.GetEventRecorder(ctx).(*k8sevent.FakeRecorder)
		EmitCloudEvents(ctx, object)
		if err := recorder.CheckEventsOrdered(t, recorder.Events, tc.name, tc.wantEvents); err != nil {
			t.Fatalf(err.Error())
		}
		fakeClient.CheckCloudEventsUnordered(t, tc.name, tc.wantCloudEvents)
	}
}

func TestEmitCloudEventsWhenConditionChange(t *testing.T) {
	objectStatus := duckv1.Status{
		Conditions: []apis.Condition{{
			Type:   apis.ConditionSucceeded,
			Status: corev1.ConditionUnknown,
			Reason: v1beta1.PipelineRunReasonStarted.String(),
		}},
	}
	object := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			SelfLink: "/pipelineruns/test1",
		},
		Status: v1beta1.PipelineRunStatus{Status: objectStatus},
	}
	after := &apis.Condition{
		Type:    apis.ConditionSucceeded,
		Status:  corev1.ConditionUnknown,
		Message: "just starting",
	}

	data := map[string]string{"default-cloud-events-sink": "http://mysink"}
	wantCloudEvents := []string{`(?s)dev.tekton.event.pipelinerun.started.v1.*test1`}

	// Setup the context and seed test data
	ctx, _ := k8sevent.SetupFakeContext(t)
	ctx = WithClient(ctx, &FakeClientBehaviour{SendSuccessfully: true}, len(wantCloudEvents))
	fakeClient := Get(ctx).(FakeClient)

	// Setup the config and add it to the context
	defaults, _ := config.NewDefaultsFromMap(data)
	featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{})
	cfg := &config.Config{
		Defaults:     defaults,
		FeatureFlags: featureFlags,
	}
	ctx = config.ToContext(ctx, cfg)

	EmitCloudEventsWhenConditionChange(ctx, nil, after, object)
	fakeClient.CheckCloudEventsUnordered(t, "with sink", wantCloudEvents)
}

func setupFakeContext(t *testing.T, behaviour FakeClientBehaviour, withClient bool, expectedEventCount int) context.Context {
	t.Helper()
	ctx, _ := rtesting.SetupFakeContext(t)
	if withClient {
		ctx = WithClient(ctx, &behaviour, expectedEventCount)
	}
	return ctx
}
