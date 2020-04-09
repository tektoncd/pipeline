package observability

import (
	"context"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

// Observable represents the the customization used by the Reporter for a given
// measurement and trace for a single method.
type Observable interface {
	MethodName() string
	LatencyMs() *stats.Float64Measure
}

// Reporter represents a running latency counter. When Error or OK are
// called, the latency is calculated. Error or OK are only allowed to
// be called once.
type Reporter interface {
	Error()
	OK()
}

type reporter struct {
	ctx   context.Context
	on    Observable
	start time.Time
	once  sync.Once
}

// LatencyTags returns all tags used for Latency measurements.
func LatencyTags() []tag.Key {
	return []tag.Key{KeyMethod, KeyResult}
}

// EnableTracing is deprecated. Tracing is always enabled.
func EnableTracing(enabled bool) {}

// NewReporter creates and returns a reporter wrapping the provided Observable.
func NewReporter(ctx context.Context, on Observable) (context.Context, Reporter) {
	r := &reporter{
		ctx:   ctx,
		on:    on,
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
