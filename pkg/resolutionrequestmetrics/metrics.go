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

// Package resolutionrequestmetrics records the end-to-end lifecycle of
// ResolutionRequest objects: how long a resolver takes to produce resolved
// data, and how long it takes for the ResolutionRequest to reach a terminal
// (Succeeded/Failed) state.
//
// Unlike pkg/pipelinerunmetrics and pkg/taskrunmetrics, this recorder is
// shared across two different binaries:
//   - cmd/resolvers calls RecordResolution when a resolver writes resolved
//     data to a ResolutionRequest, and RecordCompletion for resolver-side
//     failures that terminate a request before the pipelines controller's
//     own reconciler observes it.
//   - cmd/controller calls RecordCompletion when the pipelines controller
//     marks a ResolutionRequest Succeeded, or Failed due to a global
//     resolution timeout.
//
// Both binaries register instruments on the same "tekton_pipelines_controller"
// OTel meter name; NewRecorder's sync.Once ensures the instruments are only
// registered once per process regardless of how many callers invoke it.
package resolutionrequestmetrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	resolutioncommon "github.com/tektoncd/pipeline/pkg/resolution/common"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// CachedAnnotationKey is the annotation key the resolver framework's cache
// sets on a resolved resource to indicate it was served from cache rather
// than freshly resolved. The owning package
// (pkg/remoteresolution/resolver/framework/cache) keeps this unexported, so
// the value is duplicated here rather than imported.
const CachedAnnotationKey = "resolution.tekton.dev/cached"

// unknownResolverType is recorded for the resolver label when a
// ResolutionRequest is missing the resolution.tekton.dev/type label, which
// should not happen in practice (it is required by validation) but is
// guarded against defensively.
const unknownResolverType = "unknown"

// Recorder holds OpenTelemetry instruments for ResolutionRequest lifecycle
// metrics.
type Recorder struct {
	mutex       sync.Mutex
	initialized bool

	meter metric.Meter

	resolutionSecondsHistogram metric.Float64Histogram
	completionSecondsHistogram metric.Float64Histogram
}

var (
	once           sync.Once
	r              *Recorder
	errRegistering error
)

// NewRecorder creates (or returns the already-created) OpenTelemetry-based
// metrics recorder instance for this process. It is safe to call from
// multiple reconcilers: the underlying instruments are only registered once.
func NewRecorder(_ context.Context) (*Recorder, error) {
	once.Do(func() {
		var err error
		r, err = newRecorder(otel.GetMeterProvider())
		errRegistering = err
	})
	return r, errRegistering
}

func newRecorder(provider metric.MeterProvider) (*Recorder, error) {
	rec := &Recorder{initialized: true}
	if err := rec.configure(provider); err != nil {
		rec.initialized = false
		return rec, err
	}
	return rec, nil
}

// NewRecorderForTesting returns a freshly constructed Recorder bound to
// provider, bypassing the process-wide singleton NewRecorder normally
// returns. It exists so that package tests elsewhere in the codebase (e.g.
// for reconcilers that consume a *Recorder) can exercise metrics recording
// against an isolated OTel meter provider without interfering with, or being
// interfered by, other tests' use of the shared singleton. Production code
// should always use NewRecorder.
func NewRecorderForTesting(_ context.Context, provider metric.MeterProvider) (*Recorder, error) {
	return newRecorder(provider)
}

func (r *Recorder) configure(provider metric.MeterProvider) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.meter == nil {
		r.meter = provider.Meter("tekton_pipelines_controller")
	}

	if r.resolutionSecondsHistogram == nil {
		h, err := r.meter.Float64Histogram(
			"tekton_pipelines_controller_resolution_request_resolution_seconds",
			metric.WithDescription("Time for a resolver to produce resolved data for a ResolutionRequest, measured from the ResolutionRequest's creation time to when resolved data is written."),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600),
		)
		if err != nil {
			return fmt.Errorf("failed to create resolution_request_resolution_seconds histogram: %w", err)
		}
		r.resolutionSecondsHistogram = h
	}

	if r.completionSecondsHistogram == nil {
		h, err := r.meter.Float64Histogram(
			"tekton_pipelines_controller_resolution_request_completion_seconds",
			metric.WithDescription("End-to-end time for a ResolutionRequest, measured from its creation time to when it is marked Succeeded or Failed. Subtracting resolution_request_resolution_seconds approximates controller-side overhead on top of resolver time."),
			metric.WithUnit("s"),
			metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600),
		)
		if err != nil {
			return fmt.Errorf("failed to create resolution_request_completion_seconds histogram: %w", err)
		}
		r.completionSecondsHistogram = h
	}

	return nil
}

// Cached reports whether a resolved resource's annotations indicate it was
// served from the resolver framework's cache. Callers typically pass either
// the freshly-resolved resource's annotations (at the point resolved data is
// about to be written) or rr.Status.Annotations (once a ResolutionRequest has
// already been resolved).
func Cached(annotations map[string]string) bool {
	return annotations[CachedAnnotationKey] == "true"
}

// RecordResolution records how long it took a resolver to produce resolved
// data for rr, measured from rr's creation time to now. It should be called
// once, at the point resolved data is written to rr. r may be nil (e.g. if a
// caller was never wired up with a recorder), in which case this is a no-op.
func (r *Recorder) RecordResolution(ctx context.Context, rr *v1beta1.ResolutionRequest, cached bool) {
	if r == nil {
		return
	}
	r.record(ctx, r.resolutionSecondsHistogram, rr, cached)
}

// RecordCompletion records the end-to-end time for rr, measured from rr's
// creation time to now. It should be called once, at the point rr is marked
// Succeeded or Failed (whether by the pipelines controller's own
// ResolutionRequest reconciler, or by a resolver reporting a failure
// directly). r may be nil (e.g. if a caller was never wired up with a
// recorder), in which case this is a no-op.
func (r *Recorder) RecordCompletion(ctx context.Context, rr *v1beta1.ResolutionRequest, cached bool) {
	if r == nil {
		return
	}
	r.record(ctx, r.completionSecondsHistogram, rr, cached)
}

func (r *Recorder) record(ctx context.Context, hist metric.Float64Histogram, rr *v1beta1.ResolutionRequest, cached bool) {
	if !r.initialized || hist == nil || rr == nil {
		return
	}

	elapsed := time.Since(rr.CreationTimestamp.Time).Seconds()
	attrs := []attribute.KeyValue{
		attribute.String("resolver", resolverType(rr)),
		attribute.Bool("cached", cached),
		attribute.String("namespace", rr.Namespace),
	}

	hist.Record(ctx, elapsed, metric.WithAttributes(attrs...))
}

func resolverType(rr *v1beta1.ResolutionRequest) string {
	if t, ok := rr.Labels[resolutioncommon.LabelKeyResolverType]; ok && t != "" {
		return t
	}
	return unknownResolverType
}
