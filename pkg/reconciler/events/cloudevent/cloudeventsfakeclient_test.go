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

package cloudevent_test

import (
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/tektoncd/pipeline/pkg/reconciler/events/cloudevent"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestSend_Success(t *testing.T) {
	sendEvents := []event.Event{
		{
			Context: event.EventContextV1{
				ID: "test-event",
			}.AsV1(),
		}, {
			Context: event.EventContextV1{
				ID: "test-event2",
			}.AsV1(),
		},
	}

	wantEvents := []string{"Context Attributes,", "Context Attributes,"}

	// Setup the context and seed test event
	ctx, _ := rtesting.SetupFakeContext(t)
	ctx = cloudevent.WithFakeClient(ctx, &cloudevent.FakeClientBehaviour{SendSuccessfully: true}, len(wantEvents))
	fakeClient := cloudevent.Get(ctx).(cloudevent.FakeClient)

	for _, e := range sendEvents {
		err := fakeClient.Send(ctx, e)
		if err != nil {
			t.Fatalf("got err %v", err)
		}
	}
	fakeClient.CheckCloudEventsUnordered(t, "send cloud events", wantEvents)
}

func TestSend_Error(t *testing.T) {
	sendEvent := event.Event{
		Context: event.EventContextV1{
			ID: "test-event",
		}.AsV1(),
	}

	// Setup the context and seed test event
	ctx, _ := rtesting.SetupFakeContext(t)
	ctx = cloudevent.WithFakeClient(ctx, &cloudevent.FakeClientBehaviour{SendSuccessfully: true} /* expectedEventCount= */, 0)
	fakeClient := cloudevent.Get(ctx)
	// the channel size is 0 so no more events can be sent
	err := fakeClient.Send(ctx, sendEvent)
	if err == nil {
		t.Fatalf("want err but got nil")
	}
}
