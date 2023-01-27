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
	"fmt"
	"regexp"
	"sync"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

// FakeClientBehaviour defines how the client will behave
type FakeClientBehaviour struct {
	SendSuccessfully bool
}

// FakeClient is a fake CloudEvent client for unit testing
// Holding a pointer to the behaviour allows to change the behaviour of a client
type FakeClient struct {
	behaviour *FakeClientBehaviour
	// Modelled after k8s.io/client-go fake recorder
	events chan string
	// waitGroup is used to block until all events have been sent
	waitGroup *sync.WaitGroup
}

// newFakeClient is a FakeClient factory, it returns a client for the target
func newFakeClient(behaviour *FakeClientBehaviour, expectedEventCount int) CEClient {
	return FakeClient{
		behaviour: behaviour,
		// set buffersize to length of want events to make sure no extra events are sent
		events:    make(chan string, expectedEventCount),
		waitGroup: &sync.WaitGroup{},
	}
}

var _ cloudevents.Client = (*FakeClient)(nil)

// Send fakes the Send method from cloudevents.Client
func (c FakeClient) Send(ctx context.Context, event cloudevents.Event) protocol.Result {
	if c.behaviour.SendSuccessfully {
		// This is to prevent extra events are sent. We don't read events from channel before we call CheckCloudEventsUnordered
		if len(c.events) < cap(c.events) {
			c.events <- event.String()
			return nil
		}
		return fmt.Errorf("channel is full of size:%v, but extra event wants to be sent:%v", cap(c.events), event)
	}
	return fmt.Errorf("had to fail. Event ID: %s", event.ID())
}

// Request fakes the Request method from cloudevents.Client
func (c FakeClient) Request(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, protocol.Result) {
	if c.behaviour.SendSuccessfully {
		if len(c.events) < cap(c.events) {
			c.events <- fmt.Sprintf("%v", event.String())
			return &event, nil
		}
		return nil, fmt.Errorf("channel is full of size:%v, but extra event wants to be sent:%v", cap(c.events), event)
	}
	return nil, fmt.Errorf("had to fail. Event ID: %s", event.ID())
}

// StartReceiver fakes StartReceiver method from cloudevents.Client
func (c FakeClient) StartReceiver(ctx context.Context, fn interface{}) error {
	return nil
}

// addCount can be used to add the count when each event is going to be sent
func (c FakeClient) addCount() {
	c.waitGroup.Add(1)
}

// decreaseCount can be used to the decrease the count when each event is sent
func (c FakeClient) decreaseCount() {
	c.waitGroup.Done()
}

// WithFakeClient adds to the context a fake client with the desired behaviour and expectedEventCount
func WithFakeClient(ctx context.Context, behaviour *FakeClientBehaviour, expectedEventCount int) context.Context {
	return context.WithValue(ctx, ceKey{}, newFakeClient(behaviour, expectedEventCount))
}

// CheckCloudEventsUnordered checks that all events in wantEvents, and no others,
// were received via the given chan, in any order.
// Block until all events have been sent.
func (c *FakeClient) CheckCloudEventsUnordered(t *testing.T, testName string, wantEvents []string) {
	t.Helper()
	c.waitGroup.Wait()
	expected := append([]string{}, wantEvents...)
	channelEvents := len(c.events)

	// extra events are prevented in FakeClient's Send function.
	// fewer events are detected because we collect all events from channel and compare with wantEvents

	for eventCount := 0; eventCount < channelEvents; eventCount++ {
		event := <-c.events
		if len(expected) == 0 {
			t.Errorf("extra event received: %q", event)
		}
		found := false
		for wantIdx, want := range expected {
			matching, err := regexp.MatchString(want, event)
			if err != nil {
				t.Errorf("something went wrong matching an event: %s", err)
			}
			if matching {
				found = true
				// Remove event from list of those we expect to receive
				expected[wantIdx] = expected[len(expected)-1]
				expected = expected[:len(expected)-1]
				break
			}
		}
		if !found {
			t.Errorf("unexpected event received: %q", event)
		}
	}
	if len(expected) != 0 {
		t.Errorf("%d events %#v are not received", len(expected), expected)
	}
}
