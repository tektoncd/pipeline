package cloudevents

import (
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// EventMarshalLatencyMs measures the latency in milliseconds for the
	// CloudEvents.Event marshal/unmarshalJSON methods.
	EventMarshalLatencyMs = stats.Float64(
		"cloudevents.io/sdk-go/event/json/latency",
		"The latency in milliseconds of (un)marshalJSON methods for CloudEvents.Event.",
		"ms")
)

var (
	// LatencyView is an OpenCensus view that shows CloudEvents.Event (un)marshalJSON method latency.
	EventMarshalLatencyView = &view.View{
		Name:        "event/json/latency",
		Measure:     EventMarshalLatencyMs,
		Description: "The distribution of latency inside of (un)marshalJSON methods for CloudEvents.Event.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type observed int32

// Adheres to Observable
var _ observability.Observable = observed(0)

const (
	reportMarshal observed = iota
	reportUnmarshal
)

// TraceName implements Observable.TraceName
func (o observed) TraceName() string {
	switch o {
	case reportMarshal:
		return "cloudevents/event/marshaljson"
	case reportUnmarshal:
		return "cloudevents/event/unmarshaljson"
	default:
		return "cloudevents/event/unknwown"
	}
}

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportMarshal:
		return "marshaljson"
	case reportUnmarshal:
		return "unmarshaljson"
	default:
		return "unknown"
	}
}

// LatencyMs implements Observable.LatencyMs
func (o observed) LatencyMs() *stats.Float64Measure {
	return EventMarshalLatencyMs
}

// eventJSONObserved is a wrapper to append version to observed.
type eventJSONObserved struct {
	// Method
	o observed
	// Version
	v string
}

// Adheres to Observable
var _ observability.Observable = (*eventJSONObserved)(nil)

// TraceName implements Observable.TraceName
func (c eventJSONObserved) TraceName() string {
	return fmt.Sprintf("%s/%s", c.o.TraceName(), c.v)
}

// MethodName implements Observable.MethodName
func (c eventJSONObserved) MethodName() string {
	return fmt.Sprintf("%s/%s", c.o.MethodName(), c.v)
}

// LatencyMs implements Observable.LatencyMs
func (c eventJSONObserved) LatencyMs() *stats.Float64Measure {
	return c.o.LatencyMs()
}
