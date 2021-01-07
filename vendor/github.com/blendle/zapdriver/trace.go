package zapdriver

import (
	"fmt"

	"go.uber.org/zap"
)

const (
	traceKey        = "logging.googleapis.com/trace"
	spanKey         = "logging.googleapis.com/spanId"
	traceSampledKey = "logging.googleapis.com/trace_sampled"
)

// TraceContext adds the correct Stackdriver "trace", "span", "trace_sampled fields
//
// see: https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
func TraceContext(trace string, spanId string, sampled bool, projectName string) []zap.Field {
	return []zap.Field{
		zap.String(traceKey, fmt.Sprintf("projects/%s/traces/%s", projectName, trace)),
		zap.String(spanKey, spanId),
		zap.Bool(traceSampledKey, sampled),
	}
}
