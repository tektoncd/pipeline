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

package events_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	events "github.com/tektoncd/pipeline/pkg/reconciler/events"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/k8sevent"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestEmit(t *testing.T) {
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
		name            string
		data            map[string]string
		wantEvents      []string
		wantCloudEvents []string
	}{{
		name:            "without sink",
		data:            map[string]string{},
		wantEvents:      []string{"Normal Started"},
		wantCloudEvents: []string{},
	}, {
		name:            "with empty string sink",
		data:            map[string]string{"default-cloud-events-sink": ""},
		wantEvents:      []string{"Normal Started"},
		wantCloudEvents: []string{},
	}, {
		name:            "with sink",
		data:            map[string]string{"default-cloud-events-sink": "http://mysink"},
		wantEvents:      []string{"Normal Started"},
		wantCloudEvents: []string{`(?s)dev.tekton.event.pipelinerun.started.v1.*test1`},
	}}

	for _, tc := range testcases {
		// Setup the context and seed test data
		ctx, _ := rtesting.SetupFakeContext(t)
		ctx = cloudevent.WithFakeClient(ctx, &cloudevent.FakeClientBehaviour{SendSuccessfully: true}, len(tc.wantCloudEvents))
		fakeClient := cloudevent.Get(ctx).(cloudevent.FakeClient)

		// Setup the config and add it to the context
		defaults, _ := config.NewDefaultsFromMap(tc.data)
		cfg := &config.Config{
			Defaults:     defaults,
			FeatureFlags: config.DefaultFeatureFlags.DeepCopy(),
		}
		ctx = config.ToContext(ctx, cfg)

		recorder := controller.GetEventRecorder(ctx).(*record.FakeRecorder)
		events.Emit(ctx, nil, after, object)
		if err := k8sevent.CheckEventsOrdered(t, recorder.Events, tc.name, tc.wantEvents); err != nil {
			t.Fatalf(err.Error())
		}
		fakeClient.CheckCloudEventsUnordered(t, tc.name, tc.wantCloudEvents)
	}
}
