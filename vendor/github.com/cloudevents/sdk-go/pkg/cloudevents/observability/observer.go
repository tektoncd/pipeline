package observability

import (
	"context"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

// Observable represents the the customization used by the Reporter for a given
// measurement and trace for a single method.
type Observable interface {
	TraceName() string
	MethodName() string
	LatencyMs() *stats.Float64Measure
}

// Reporter represents a running latency counter and trace span. When Error or
// OK are called, the latency is calculated and the trace space is ended. Error
// or OK are only allowed to be called once.
type Reporter interface {
	Error()
	OK()
}

type reporter struct {
	ctx   context.Context
	span  *trace.Span
	on    Observable
	start time.Time
	once  sync.Once
}

// All tags used for Latency measurements.
func LatencyTags() []tag.Key {
	return []tag.Key{KeyMethod, KeyResult}
}

var (
	// Tracing is disabled by default. It is very useful for profiling an
	// application.
	tracingEnabled = false
)

// EnableTracing allows control over if tracing is enabled for the sdk.
// Default is false. This applies to all of the
// `github.com/cloudevents/sdk-go/...` package.
func EnableTracing(enabled bool) {
	tracingEnabled = enabled
}

// NewReporter creates and returns a reporter wrapping the provided Observable,
// and injects a trace span into the context.
func NewReporter(ctx context.Context, on Observable) (context.Context, Reporter) {
	var span *trace.Span
	if tracingEnabled {
		ctx, span = trace.StartSpan(ctx, on.TraceName())
	}
	r := &reporter{
		ctx:   ctx,
		on:    on,
		span:  span,
		start: time.Now(),
	}
	r.tagMethod()
	return ctx, r
}

func (r *reporter) tagMethod() {
	var err error
	r.ctx, err = tag.New(r.ctx, tag.Insert(KeyMethod, r.on.MethodName()))
	if err != nil {
		panic(err) // or ignore?
	}
}

func (r *reporter) record() {
	ms := float64(time.Since(r.start) / time.Millisecond)
	stats.Record(r.ctx, r.on.LatencyMs().M(ms))
	if r.span != nil {
		r.span.End()
	}
}

// Error records the result as an error.
func (r *reporter) Error() {
	r.once.Do(func() {
		r.result(ResultError)
	})
}

// OK records the result as a success.
func (r *reporter) OK() {
	r.once.Do(func() {
		r.result(ResultOK)
	})
}

func (r *reporter) result(v string) {
	var err error
	r.ctx, err = tag.New(r.ctx, tag.Insert(KeyResult, v))
	if err != nil {
		panic(err) // or ignore?
	}
	r.record()
}
