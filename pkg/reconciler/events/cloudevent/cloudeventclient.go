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
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterClient(func(ctx context.Context, _ *rest.Config) context.Context {
		return withCloudEventClient(ctx)
	})
	injection.Dynamic.RegisterDynamicClient(withCloudEventClient)
}

// ceKey is used to associate the CloudEventClient inside the context.Context
type ceKey struct{}

func withCloudEventClient(ctx context.Context) context.Context {
	logger := logging.FromContext(ctx)

	// When KeepAlive is enabled the connections are not reused - see
	// Bug https://github.com/tektoncd/pipeline/issues/3190. This causes the
	// number of connections to keep growing, even if when we limit max idle
	// connections in the transport.
	// TODO(afrittoli) Re-enable keep alive and ensure connections are reused
	// See feature https://github.com/tektoncd/pipeline/issues/3204
	var useOnceTransport http.RoundTripper = &http.Transport{
		DisableKeepAlives: true,
	}

	p, err := cloudevents.NewHTTP(cloudevents.WithRoundTripper(useOnceTransport))
	if err != nil {
		logger.Panicf("Error creating the cloudevents http protocol: %s", err)
	}

	cloudEventClient, err := cloudevents.NewClient(p, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())
	if err != nil {
		logger.Panicf("Error creating the cloudevents client: %s", err)
	}

	celient := CloudClient{
		client: cloudEventClient,
	}
	return context.WithValue(ctx, ceKey{}, celient)
}

// CloudClient is a wrapper of CloudEvents client and implements addCount and decreaseCount
type CloudClient struct {
	client client.Client
}

// AddCount does nothing
func (c CloudClient) addCount() {
}

// DecreaseCount does nothing
func (c CloudClient) decreaseCount() {
}

// Send invokes call client.Send
func (c CloudClient) Send(ctx context.Context, event cloudevents.Event) protocol.Result {
	return c.client.Send(ctx, event)
}

// Request invokes client.Request
func (c CloudClient) Request(ctx context.Context, event event.Event) (*cloudevents.Event, protocol.Result) {
	return c.client.Request(ctx, event)
}

// StartReceiver invokes client.StartReceiver
func (c CloudClient) StartReceiver(ctx context.Context, fn interface{}) error {
	return c.client.StartReceiver(ctx, fn)
}

// Get extracts the cloudEventClient client from the context.
func Get(ctx context.Context) CEClient {
	untyped := ctx.Value(ceKey{})
	if untyped == nil {
		logging.FromContext(ctx).Errorf(
			"Unable to fetch client from context.")
		return nil
	}
	return untyped.(CEClient)
}

// ToContext adds the cloud events client to the context
func ToContext(ctx context.Context, cec CEClient) context.Context {
	return context.WithValue(ctx, ceKey{}, cec)
}
