package codec

import (
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// LatencyMs measures the latency in milliseconds for the CloudEvents json codec methods.
	LatencyMs = stats.Float64("cloudevents.io/sdk-go/codec/json/latency", "The latency in milliseconds for the CloudEvents json codec methods.", "ms")
)

var (
	// LatencyView is an OpenCensus view that shows codec/json method latency.
	LatencyView = &view.View{
		Name:        "codec/json/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of the json codec for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type observed int32

// Adheres to Observable
var _ observability.Observable = observed(0)

const (
	reportEncode observed = iota
	reportDecode
)

// TraceName implements Observable.TraceName
func (o observed) TraceName() string {
	switch o {
	case reportEncode:
		return "codec/json/encode"
	case reportDecode:
		return "codec/json/decode"
	default:
		return "codec/unknown"
	}
}

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportEncode:
		return "encode"
	case reportDecode:
		return "decode"
	default:
		return "unknown"
	}
}

// LatencyMs implements Observable.LatencyMs
func (o observed) LatencyMs() *stats.Float64Measure {
	return LatencyMs
}

// codecObserved is a wrapper to append version to observed.
type codecObserved struct {
	// Method
	o observed
	// Version
	v string
}

// Adheres to Observable
var _ observability.Observable = (*codecObserved)(nil)

// TraceName implements Observable.TraceName
func (c codecObserved) TraceName() string {
	return fmt.Sprintf("%s/%s", c.o.TraceName(), c.v)
}

// MethodName implements Observable.MethodName
func (c codecObserved) MethodName() string {
	return fmt.Sprintf("%s/%s", c.o.MethodName(), c.v)
}

// LatencyMs implements Observable.LatencyMs
func (c codecObserved) LatencyMs() *stats.Float64Measure {
	return c.o.LatencyMs()
}
