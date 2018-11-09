/*
Copyright 2018 The Knative Authors

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

// logging.go contains the logic to configure and interact with the
// logging and metrics libraries.

package logging

import (
	"flag"
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/knative/pkg/logging"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
)

const (
	// VerboseLogLevel defines verbose log level as 10
	VerboseLogLevel glog.Level = 10

	// 1 second was chosen arbitrarily
	metricViewReportingPeriod = 1 * time.Second
)

var baseLogger *BaseLogger

var exporter *zapMetricExporter

// zapMetricExporter is a stats and trace exporter that logs the
// exported data to the provided (probably test specific) zap logger.
// It conforms to the view.Exporter and trace.Exporter interfaces.
type zapMetricExporter struct {
	logger *zap.SugaredLogger
}

// BaseLogger is a common knative test files logger.
type BaseLogger struct {
	Logger *zap.SugaredLogger
}

// ExportView will emit the view data vd (i.e. the stats that have been
// recorded) to the zap logger.
func (e *zapMetricExporter) ExportView(vd *view.Data) {
	// We are not curretnly consuming these metrics, so for now we'll juse
	// dump the view.Data object as is.
	e.logger.Info(vd)
}

// ExportSpan will emit the trace data to the zap logger.
func (e *zapMetricExporter) ExportSpan(vd *trace.SpanData) {
	duration := vd.EndTime.Sub(vd.StartTime)
	// We will start the log entry with `metric` to identify it as a metric for parsing
	e.logger.Infof("metric %s %d %d %s", vd.Name, vd.StartTime.UnixNano(), vd.EndTime.UnixNano(), duration)
}

func newLogger(logLevel string) *BaseLogger {
	configJSONTemplate := `{
	  "level": "%s",
	  "encoding": "console",
	  "outputPaths": ["stdout"],
	  "errorOutputPaths": ["stderr"],
	  "encoderConfig": {
	    "timeKey": "ts",
	    "messageKey": "message",
	    "levelKey": "level",
	    "nameKey": "logger",
	    "callerKey": "caller",
	    "messageKey": "msg",
	    "stacktraceKey": "stacktrace",
	    "lineEnding": "",
	    "levelEncoder": "",
	    "timeEncoder": "iso8601",
	    "durationEncoder": "",
	    "callerEncoder": ""
	  }
	}`
	configJSON := fmt.Sprintf(configJSONTemplate, logLevel)
	l, _ := logging.NewLogger(string(configJSON), logLevel, zap.AddCallerSkip(1))
	return &BaseLogger{Logger: l}
}

// InitializeMetricExporter initializes the metric exporter logger
func InitializeMetricExporter() {
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	view.SetReportingPeriod(metricViewReportingPeriod)
}

// InitializeLogger initializes the base logger
func InitializeLogger(logVerbose bool) {
	logLevel := "info"
	if logVerbose {
		// Both gLog and "go test" use -v flag. The code below is a work around so that we can still set v value for gLog
		flag.StringVar(&logLevel, "logLevel", fmt.Sprint(VerboseLogLevel), "verbose log level")
		flag.Lookup("v").Value.Set(logLevel)
		glog.Infof("Logging set to verbose mode with logLevel %d", VerboseLogLevel)

		logLevel = "debug"
	}
	baseLogger = newLogger(logLevel)
}

// GetContextLogger creates a Named logger (https://godoc.org/go.uber.org/zap#Logger.Named)
// using the provided context as a name. This will also register the logger as a metric exporter,
// which is unfortunately global, so calling `GetContextLogger` will have the side effect of
// changing the context in which all metrics are logged from that point forward.
func GetContextLogger(context string) *BaseLogger {
	// If there was a previously registered exporter, unregister it so we only emit
	// the metrics in the current context.
	if exporter != nil {
		view.UnregisterExporter(exporter)
		trace.UnregisterExporter(exporter)
	}

	logger := baseLogger.Logger.Named(context)

	exporter = &zapMetricExporter{logger: logger}
	view.RegisterExporter(exporter)
	trace.RegisterExporter(exporter)

	return &BaseLogger{Logger: logger}
}

// Infof logs a templated message.
func (b *BaseLogger) Infof(template string, args ...interface{}) {
	b.Logger.Infof(template, args...)
}

// Info logs an info message.
func (b *BaseLogger) Info(args ...interface{}) {
	b.Logger.Info(args...)
}

// Fatal logs a fatal message.
func (b *BaseLogger) Fatal(args ...interface{}) {
	b.Logger.Fatal(args...)
}

// Fatalf logs a templated fatal message.
func (b *BaseLogger) Fatalf(template string, args ...interface{}) {
	b.Logger.Fatalf(template, args...)
}

// Debugf logs a templated debug message.
func (b *BaseLogger) Debugf(template string, args ...interface{}) {
	b.Logger.Debugf(template, args...)
}

// Debug logs a debug message.
func (b *BaseLogger) Debug(args ...interface{}) {
	b.Logger.Debug(args...)
}

// Errorf logs a templated error message.
func (b *BaseLogger) Errorf(template string, args ...interface{}) {
	b.Logger.Errorf(template, args...)
}

// Error logs an error message.
func (b *BaseLogger) Error(args ...interface{}) {
	b.Logger.Error(args...)
}
