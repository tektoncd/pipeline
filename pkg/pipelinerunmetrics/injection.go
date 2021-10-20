/*
Copyright 2021 The Tekton Authors

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

package pipelinerunmetrics

import (
	"context"

	pipelineruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/pipelinerun"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1beta1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

func init() {
	injection.Default.RegisterClient(func(ctx context.Context, _ *rest.Config) context.Context { return WithClient(ctx) })
	injection.Default.RegisterInformer(WithInformer)

	injection.Dynamic.RegisterDynamicClient(WithClient)
	injection.Dynamic.RegisterDynamicInformer(func(ctx context.Context) context.Context {
		ctx, _ = WithInformer(ctx)
		return ctx
	})
}

// RecorderKey is used for associating the Recorder inside the context.Context.
type RecorderKey struct{}

// WithClient adds a metrics recorder to the given context
func WithClient(ctx context.Context) context.Context {
	rec, err := NewRecorder(ctx)
	if err != nil {
		logging.FromContext(ctx).Errorf("Failed to create pipelinerun metrics recorder %v", err)
	}
	return context.WithValue(ctx, RecorderKey{}, rec)
}

// Get extracts the pipelinerunmetrics.Recorder from the context.
func Get(ctx context.Context) *Recorder {
	untyped := ctx.Value(RecorderKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic("Unable to fetch *pipelinerunmetrics.Recorder from context.")
	}
	return untyped.(*Recorder)
}

// InformerKey is used for associating the Informer inside the context.Context.
type InformerKey struct{}

// WithInformer returns the given context, and a configured informer
func WithInformer(ctx context.Context) (context.Context, controller.Informer) {
	return ctx, &recorderInformer{
		ctx:     ctx,
		metrics: Get(ctx),
		lister:  pipelineruninformer.Get(ctx).Lister(),
	}
}

type recorderInformer struct {
	ctx     context.Context
	metrics *Recorder
	lister  listers.PipelineRunLister
}

var _ controller.Informer = (*recorderInformer)(nil)

// Run starts the recorder informer in a goroutine
func (ri *recorderInformer) Run(stopCh <-chan struct{}) {
	// Turn the stopCh into a context for reporting metrics.
	ctx, cancel := context.WithCancel(ri.ctx)
	go func() {
		<-stopCh
		cancel()
	}()

	go ri.metrics.ReportRunningPipelineRuns(ctx, ri.lister)
}

// HasSynced returns whether the informer has synced, which in this case will always be true.
func (ri *recorderInformer) HasSynced() bool {
	return true
}
