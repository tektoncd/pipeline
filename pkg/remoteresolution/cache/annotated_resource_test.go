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
	tests := []struct {
		name         string
		resolverType string
		hasExisting  bool
	}{
		{
			name:         "with existing annotations",
			resolverType: "bundles",
			hasExisting:  true,
		},
		{
			name:         "without existing annotations",
			resolverType: "git",
			hasExisting:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock resource
			mockAnnotations := make(map[string]string)
			if tt.hasExisting {
				mockAnnotations["existing-key"] = "existing-value"
			}

			mockResource := &mockResolvedResource{
				data:        []byte("test data"),
				annotations: mockAnnotations,
				refSource: &v1.RefSource{
					URI: "test-uri",
				},
			}

			// Create annotated resource
			annotated := NewAnnotatedResource(mockResource, tt.resolverType, CacheOperationStore)

			// Verify data is preserved
			if string(annotated.Data()) != "test data" {
				t.Errorf("Expected data 'test data', got '%s'", string(annotated.Data()))
			}

			// Verify annotations are added
			annotations := annotated.Annotations()
			if annotations[CacheAnnotationKey] != "true" {
				t.Errorf("Expected cache annotation to be 'true', got '%s'", annotations[CacheAnnotationKey])
			}

			if annotations[CacheResolverTypeKey] != tt.resolverType {
				t.Errorf("Expected resolver type '%s', got '%s'", tt.resolverType, annotations[CacheResolverTypeKey])
			}

			// Verify timestamp is added and valid
			timestamp := annotations[CacheTimestampKey]
			if timestamp == "" {
				t.Error("Expected cache timestamp to be set")
			}

			// Verify timestamp is valid RFC3339 format
			_, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				t.Errorf("Expected valid RFC3339 timestamp, got error: %v", err)
			}

			// Verify cache operation is set
			if annotations[CacheOperationKey] != CacheOperationStore {
				t.Errorf("Expected cache operation '%s', got '%s'", CacheOperationStore, annotations[CacheOperationKey])
			}

			// Verify existing annotations are preserved
			if tt.hasExisting {
				if annotations["existing-key"] != "existing-value" {
					t.Errorf("Expected existing annotation to be preserved, got '%s'", annotations["existing-key"])
				}
			}

			// Verify RefSource is preserved
			if annotated.RefSource().URI != "test-uri" {
				t.Errorf("Expected RefSource URI 'test-uri', got '%s'", annotated.RefSource().URI)
			}
		})
	}
}

func TestNewAnnotatedResourceWithNilAnnotations(t *testing.T) {
	// Create mock resource with nil annotations
	mockResource := &mockResolvedResource{
		data:        []byte("test data"),
		annotations: nil,
		refSource: &v1.RefSource{
			URI: "test-uri",
		},
	}

	// Create annotated resource
	annotated := NewAnnotatedResource(mockResource, "bundles", CacheOperationStore)

	// Verify annotations map is created
	annotations := annotated.Annotations()
	if annotations == nil {
		t.Error("Expected annotations map to be created")
	}

	// Verify cache annotations are added
	if annotations[CacheAnnotationKey] != "true" {
		t.Errorf("Expected cache annotation to be 'true', got '%s'", annotations[CacheAnnotationKey])
	}

	if annotations[CacheResolverTypeKey] != "bundles" {
		t.Errorf("Expected resolver type 'bundles', got '%s'", annotations[CacheResolverTypeKey])
	}

	// Verify cache operation is set
	if annotations[CacheOperationKey] != CacheOperationStore {
		t.Errorf("Expected cache operation '%s', got '%s'", CacheOperationStore, annotations[CacheOperationKey])
	}
}

func TestAnnotatedResourcePreservesOriginal(t *testing.T) {
	// Create mock resource
	mockResource := &mockResolvedResource{
		data: []byte("original data"),
		annotations: map[string]string{
			"original-key": "original-value",
		},
		refSource: &v1.RefSource{
			URI: "original-uri",
		},
	}

	// Create annotated resource
	annotated := NewAnnotatedResource(mockResource, "git", CacheOperationStore)

	// Verify original resource is not modified
	if string(mockResource.Data()) != "original data" {
		t.Error("Original resource data should not be modified")
	}

	if mockResource.Annotations()["original-key"] != "original-value" {
		t.Error("Original resource annotations should not be modified")
	}

	if mockResource.RefSource().URI != "original-uri" {
		t.Error("Original resource RefSource should not be modified")
	}

	// Verify annotated resource has both original and cache annotations
	annotations := annotated.Annotations()
	if annotations["original-key"] != "original-value" {
		t.Error("Annotated resource should preserve original annotations")
	}

	if annotations[CacheAnnotationKey] != "true" {
		t.Error("Annotated resource should have cache annotation")
	}

	// Verify cache operation is set correctly
	if annotations[CacheOperationKey] != CacheOperationStore {
		t.Errorf("Expected cache operation '%s', got '%s'", CacheOperationStore, annotations[CacheOperationKey])
	}
}

func TestNewAnnotatedResourceWithRetrieveOperation(t *testing.T) {
	// Create mock resource
	mockResource := &mockResolvedResource{
		data: []byte("test data"),
		annotations: map[string]string{
			"existing-key": "existing-value",
		},
		refSource: &v1.RefSource{
			URI: "test-uri",
		},
	}

	// Create annotated resource with retrieve operation
	annotated := NewAnnotatedResource(mockResource, "bundles", CacheOperationRetrieve)

	// Verify cache operation is set correctly
	annotations := annotated.Annotations()
	if annotations[CacheOperationKey] != CacheOperationRetrieve {
		t.Errorf("Expected cache operation '%s', got '%s'", CacheOperationRetrieve, annotations[CacheOperationKey])
	}

	// Verify other annotations are still set
	if annotations[CacheAnnotationKey] != "true" {
		t.Errorf("Expected cache annotation to be 'true', got '%s'", annotations[CacheAnnotationKey])
	}

	if annotations[CacheResolverTypeKey] != "bundles" {
		t.Errorf("Expected resolver type 'bundles', got '%s'", annotations[CacheResolverTypeKey])
	}
}
