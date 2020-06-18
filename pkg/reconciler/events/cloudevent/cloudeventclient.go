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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterClient(withCloudEventClient)
}

// CECKey is used to associate the CloudEventClient inside the context.Context
type CECKey struct{}

func withCloudEventClient(ctx context.Context, cfg *rest.Config) context.Context {
	logger := logging.FromContext(ctx)

	p, err := cloudevents.NewHTTP()

	if err != nil {
		logger.Panicf("Error creating the cloudevents http protocol: %s", err)
		return ctx
	}

	cloudEventClient, err := cloudevents.NewClient(p, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())
	if err != nil {
		logger.Panicf("Error creating the cloudevents client: %s", err)
	}

	return context.WithValue(ctx, CECKey{}, cloudEventClient)
}

// Get extracts the cloudEventClient client from the context.
func Get(ctx context.Context) CEClient {
	untyped := ctx.Value(CECKey{})
	if untyped == nil {
		logging.FromContext(ctx).Errorf(
			"Unable to fetch client from context.")
		return nil
	}
	return untyped.(CEClient)
}

// ToContext adds the cloud events client to the context
func ToContext(ctx context.Context, cec CEClient) context.Context {
	return context.WithValue(ctx, CECKey{}, cec)
}
