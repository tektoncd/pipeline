/*
Copyright 2023 The Tekton Authors

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

package tracing_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/tracing"
	"go.uber.org/zap"
)

func TestNewTracerProvider(t *testing.T) {
	tp := tracing.New("test-serice")

	tracer := tp.Tracer("tracer")
	_, span := tracer.Start(context.TODO(), "example")

	// tp.Tracer should return a nooptracer initially
	// recording is always false for spans created by nooptracer
	if span.IsRecording() {
		t.Fatalf("Span is recording before configuration")
	}
}

func TestOnStore(t *testing.T) {
	tp := tracing.New("test-service")

	cfg := &config.Tracing{
		Enabled: false,
	}

	tp.OnStore(zap.NewExample().Sugar())("config-tracing", cfg)

	tracer := tp.Tracer("tracer")
	_, span := tracer.Start(context.TODO(), "example")

	// tp.Tracer should return a nooptracer when tracing is disabled
	// recording is always false for spans created by nooptracer
	if span.IsRecording() {
		t.Fatalf("Span is recording with tracing disabled")
	}
}

func TestOnStoreWithEnabled(t *testing.T) {
	tp := tracing.New("test-service")

	cfg := &config.Tracing{
		Enabled:  true,
		Endpoint: "test-endpoint",
	}

	tp.OnStore(zap.NewExample().Sugar())("config-tracing", cfg)

	tracer := tp.Tracer("tracer")
	_, span := tracer.Start(context.TODO(), "example")

	if !span.IsRecording() {
		t.Fatalf("Span is not recording with tracing enabled")
	}
}
