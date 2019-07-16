package client

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// LatencyMs measures the latency in milliseconds for the CloudEvents
	// client methods.
	LatencyMs = stats.Float64("cloudevents.io/sdk-go/client/latency", "The latency in milliseconds for the CloudEvents client methods.", "ms")
)

var (
	// LatencyView is an OpenCensus view that shows client method latency.
	LatencyView = &view.View{
		Name:        "client/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of client for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type observed int32

// Adheres to Observable
var _ observability.Observable = observed(0)

const (
	reportSend observed = iota
	reportReceive
	reportReceiveFn
)

// TraceName implements Observable.TraceName
func (o observed) TraceName() string {
	switch o {
	case reportSend:
		return "client/send"
	case reportReceive:
		return "client/receive"
	case reportReceiveFn:
		return "client/receive/fn"
	default:
		return "client/unknown"
	}
}

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportSend:
		return "send"
	case reportReceive:
		return "receive"
	case reportReceiveFn:
		return "receive/fn"
	default:
		return "unknown"
	}
}

// LatencyMs implements Observable.LatencyMs
func (o observed) LatencyMs() *stats.Float64Measure {
	return LatencyMs
}
