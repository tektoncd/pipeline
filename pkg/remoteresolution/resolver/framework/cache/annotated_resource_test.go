/*
Copyright 2025 The Tekton Authors

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

package cache

import (
	"testing"
	"time"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

// mockResolvedResource implements resolutionframework.ResolvedResource for testing
type mockResolvedResource struct {
	data        []byte
	annotations map[string]string
	refSource   *v1.RefSource
}

func (m *mockResolvedResource) Data() []byte {
	return m.data
}

func (m *mockResolvedResource) Annotations() map[string]string {
	return m.annotations
}

func (m *mockResolvedResource) RefSource() *v1.RefSource {
	return m.refSource
}

func TestNewAnnotatedResource(t *testing.T) {
	// GIVEN
	mockResource := &mockResolvedResource{
		data:        []byte("test data"),
		annotations: map[string]string{"existing-key": "existing-value"},
		refSource:   &v1.RefSource{URI: "test-uri"},
	}
	expectedTimestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC).Format(time.RFC3339)
	resolverType := "bundles"

	// WHEN
	annotated := newAnnotatedResource(mockResource, resolverType, cacheOperationStore, expectedTimestamp)

	// THEN
	if string(annotated.Data()) != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", string(annotated.Data()))
	}

	annotations := annotated.Annotations()
	if annotations[cacheAnnotationKey] != "true" {
		t.Errorf("Expected cache annotation to be 'true', got '%s'", annotations[cacheAnnotationKey])
	}

	if annotations[cacheResolverTypeKey] != resolverType {
		t.Errorf("Expected resolver type '%s', got '%s'", resolverType, annotations[cacheResolverTypeKey])
	}

	if annotations[cacheTimestampKey] != expectedTimestamp {
		t.Errorf("Expected cache timestamp to be %s, got %s", expectedTimestamp, annotations[cacheTimestampKey])
	}

	if _, err := time.Parse(time.RFC3339, annotations[cacheTimestampKey]); err != nil {
		t.Errorf("Expected valid RFC3339 timestamp, got error: %v", err)
	}

	if annotations[cacheOperationKey] != cacheOperationStore {
		t.Errorf("Expected cache operation '%s', got '%s'", cacheOperationStore, annotations[cacheOperationKey])
	}

	if annotations["existing-key"] != "existing-value" {
		t.Errorf("Expected existing annotation to be preserved, got '%s'", annotations["existing-key"])
	}

	if annotated.RefSource().URI != "test-uri" {
		t.Errorf("Expected RefSource URI 'test-uri', got '%s'", annotated.RefSource().URI)
	}
}
