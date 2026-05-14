/*
Copyright 2026 The Tekton Authors

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

package resolvermetrics

import (
	"context"

	"k8s.io/client-go/rest"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// RecorderKey is used for associating the Recorder inside the context.Context.
type RecorderKey struct{}

func init() {
	injection.Default.RegisterClient(func(ctx context.Context, _ *rest.Config) context.Context { return WithClient(ctx) })
}

// WithClient creates a new metrics recorder and adds it to the context.
func WithClient(ctx context.Context) context.Context {
	rec, err := NewRecorder()
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to create resolver metrics recorder: %v", err)
	}
	return context.WithValue(ctx, RecorderKey{}, rec)
}

// Get extracts the resolver metrics Recorder from the context.
func Get(ctx context.Context) *Recorder {
	untyped := ctx.Value(RecorderKey{})
	if untyped == nil {
		logging.FromContext(ctx).Warn("Unable to fetch *resolvermetrics.Recorder from context, metrics will be disabled")
		return nil
	}
	return untyped.(*Recorder)
}
