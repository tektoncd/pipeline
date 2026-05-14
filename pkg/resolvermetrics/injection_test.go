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

package resolvermetrics

import (
	"context"
	"testing"

	_ "knative.dev/pkg/system/testing"
)

func TestWithClientStoresRecorder(t *testing.T) {
	_, cleanup := setupTestMeter(t)
	defer cleanup()

	ctx := WithClient(context.Background())
	rec := ctx.Value(RecorderKey{})
	if rec == nil {
		t.Fatal("WithClient did not store recorder in context")
	}
	if _, ok := rec.(*Recorder); !ok {
		t.Fatalf("expected *Recorder, got %T", rec)
	}
}

func TestGetReturnsRecorder(t *testing.T) {
	_, cleanup := setupTestMeter(t)
	defer cleanup()

	ctx := WithClient(context.Background())
	rec := Get(ctx)
	if rec == nil {
		t.Fatal("Get returned nil")
	}
	if !rec.initialized {
		t.Fatal("Get returned uninitialized recorder")
	}
}

func TestGetReturnsNilForMissingContext(t *testing.T) {
	rec := Get(context.Background())
	if rec != nil {
		t.Fatal("expected nil for missing context, got non-nil")
	}
}
