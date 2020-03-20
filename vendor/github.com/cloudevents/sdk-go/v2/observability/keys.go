package observability

import (
	"go.opencensus.io/tag"
)

var (
	// KeyMethod is the tag used for marking method on a metric.
	KeyMethod, _ = tag.NewKey("method")
	// KeyResult is the tag used for marking result on a metric.
	KeyResult, _ = tag.NewKey("result")
)

const (
	// ClientSpanName is the key used to start spans from the client.
	ClientSpanName = "cloudevents.client"

	// ResultError is a shared result tag value for error.
	ResultError = "error"
	// ResultOK is a shared result tag value for success.
	ResultOK = "success"
)
