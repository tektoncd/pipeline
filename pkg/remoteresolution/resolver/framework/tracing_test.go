/*
Copyright 2026 The Tekton Authors

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

// This file is in package framework (not framework_test) so it can access
// the unexported tracerProvider field on Reconciler.
package framework

import (
	"context"
	"errors"
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	fakerrclient "github.com/tektoncd/pipeline/pkg/client/resolution/clientset/versioned/fake"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestMarkFailedCreatesSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	rr := &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rr",
			Namespace: "default",
		},
		Status: v1beta1.ResolutionRequestStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   apis.ConditionSucceeded,
					Status: "Unknown",
				}},
			},
		},
	}

	fakeClient := fakerrclient.NewSimpleClientset(rr)
	r := &Reconciler{
		resolutionRequestClientSet: fakeClient,
		tracerProvider:             tp,
	}

	_ = r.MarkFailed(context.Background(), rr, errors.New("something went wrong"))

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span to be recorded, got none")
	}

	var found bool
	for _, s := range spans {
		if s.Name() == "MarkFailed" {
			found = true
		}
	}
	if !found {
		names := make([]string, len(spans))
		for i, s := range spans {
			names[i] = s.Name()
		}
		t.Errorf("MarkFailed span not found; recorded spans: %v", names)
	}
}

func TestWriteResolvedDataCreatesSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

	rr := &v1beta1.ResolutionRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rr",
			Namespace: "default",
		},
	}

	fakeClient := fakerrclient.NewSimpleClientset(rr)
	r := &Reconciler{
		resolutionRequestClientSet: fakeClient,
		tracerProvider:             tp,
	}

	resource := &fakeResolvedResource{data: []byte("resolved-content")}
	_ = r.writeResolvedData(context.Background(), rr, resource)

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span to be recorded, got none")
	}

	var found bool
	for _, s := range spans {
		if s.Name() == "writeResolvedData" {
			found = true
		}
	}
	if !found {
		names := make([]string, len(spans))
		for i, s := range spans {
			names[i] = s.Name()
		}
		t.Errorf("writeResolvedData span not found; recorded spans: %v", names)
	}
}

// fakeResolvedResource is a minimal ResolvedResource for use in tracing tests.
type fakeResolvedResource struct {
	data []byte
}

func (f *fakeResolvedResource) Data() []byte                     { return f.data }
func (f *fakeResolvedResource) Annotations() map[string]string   { return nil }
func (f *fakeResolvedResource) RefSource() *pipelinev1.RefSource { return nil }
