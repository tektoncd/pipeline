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

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
)

// FakeClientBehaviour defines how the client will behave
type FakeClientBehaviour struct {
	SendSuccessfully bool
}

// FakeClient is a fake CloudEvent client for unit testing
// Holding a pointer to the behaviour allows to change the behaviour of a client
type FakeClient struct {
	behaviour *FakeClientBehaviour
	event     cloudevents.Event
}

// NewFakeClient is a FakeClient factory, it returns a client for the target
func NewFakeClient(behaviour *FakeClientBehaviour) client.Client {
	c := FakeClient{
		behaviour: behaviour,
	}
	return c
}

// Send fakes the Send method from kncloudevents.NewDefaultClient
func (c FakeClient) Send(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, error) {
	c.event = event
	if c.behaviour.SendSuccessfully {
		return &event, nil
	}
	return nil, fmt.Errorf("%s had to fail", event.Context.GetID())
}

// StartReceiver fakes the StartReceiver method from kncloudevents.NewDefaultClient
func (c FakeClient) StartReceiver(ctx context.Context, fn interface{}) error {
	return nil
}

// WithClient adds to the context a fake client with the desired behaviour
func WithClient(ctx context.Context, behaviour *FakeClientBehaviour) context.Context {
	return context.WithValue(ctx, CECKey{}, NewFakeClient(behaviour))
}
