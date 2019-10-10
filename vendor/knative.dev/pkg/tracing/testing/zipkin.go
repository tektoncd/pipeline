/*
Copyright 2019 The Knative Authors

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

package testing

import (
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinreporter "github.com/openzipkin/zipkin-go/reporter"
	"github.com/openzipkin/zipkin-go/reporter/recorder"
	"knative.dev/pkg/tracing"
	"knative.dev/pkg/tracing/config"
)

// FakeZipkineExporter is intended to capture the testing boilerplate of building
// up the ConfigOption to pass NewOpenCensusTracer and expose a mechanism for examining
// the traces it would have reported.  To set it up, use something like:
//    reporter, co := FakeZipkinExporter()
//    defer reporter.Close()
//    oct := NewOpenCensusTracer(co)
//    defer oct.Close()
//    // Do stuff.
//    spans := reporter.Flush()
//    // Check reported spans.
func FakeZipkinExporter() (*recorder.ReporterRecorder, tracing.ConfigOption) {
	// Create tracer with reporter recorder
	reporter := recorder.NewReporter()
	endpoint, _ := openzipkin.NewEndpoint("test", "localhost:1234")
	exp := tracing.WithZipkinExporter(func(cfg *config.Config) (zipkinreporter.Reporter, error) {
		return reporter, nil
	}, endpoint)

	return reporter, exp
}
