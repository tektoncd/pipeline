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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

const bufferSize = 100

// FakeClientBehaviour defines how the client will behave
type FakeClientBehaviour struct {
	SendSuccessfully bool
}

// FakeClient is a fake CloudEvent client for unit testing
// Holding  pointer to the behaviour allows to change the behaviour of a client
type FakeClient struct {
	behaviour *FakeClientBehaviour
	// Modelled after k8s.io/client-go fake recorder
	Events chan string
}

// newFakeClient is a FakeClient factory, it returns a client for the target
func newFakeClient(behaviour *FakeClientBehaviour) cloudevents.Client {
	return FakeClient{
		behaviour: behaviour,
		Events:    make(chan string, bufferSize),
	}
}

var _ cloudevents.Client = (*FakeClient)(nil)

// Send fakes the Send method from cloudevents.Client
func (c FakeClient) Send(ctx context.Context, event cloudevents.Event) protocol.Result {
	if c.behaviour.SendSuccessfully {
		c.Events <- fmt.Sprintf("%s", event.String())
		return nil
	}
	return fmt.Errorf("Had to fail. Event ID: %s", event.ID())
}

// Request fakes the Request method from cloudevents.Client
func (c FakeClient) Request(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, protocol.Result) {
	if c.behaviour.SendSuccessfully {
		c.Events <- fmt.Sprintf("%v", event.String())
		return &event, nil
	}
	return nil, fmt.Errorf("Had to fail. Event ID: %s", event.ID())
}

// StartReceiver fakes StartReceiver method from cloudevents.Client
func (c FakeClient) StartReceiver(ctx context.Context, fn interface{}) error {
	return nil
}

// WithClient adds to the context a fake client with the desired behaviour
func WithClient(ctx context.Context, behaviour *FakeClientBehaviour) context.Context {
	return context.WithValue(ctx, CECKey{}, newFakeClient(behaviour))
}
