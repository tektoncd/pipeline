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
	"context"
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()

	logtesting "knative.dev/pkg/logging/testing"
)

func TestGetWithContextValue(t *testing.T) {
	logger := logtesting.TestLogger(t)
	ctx := t.Context()

	// Create a cache and inject it into context
	testCache := newResolverCache(100, defaultExpiration).withLogger(logger)
	ctx = context.WithValue(ctx, resolverCacheKey{}, testCache)

	// Get cache from context
	resolverCache := Get(ctx)
	if resolverCache == nil {
		t.Error("Expected resolver cache but got nil")
	}

	if resolverCache != testCache {
		t.Error("Expected injected cache but got different instance")
	}
}

func TestCacheSharing(t *testing.T) {
	// Create two different contexts
	ctx1 := logtesting.TestContextWithLogger(t)
	ctx2 := logtesting.TestContextWithLogger(t)

	// Get cache from first context
	cache1 := Get(ctx1)
	if cache1 == nil {
		t.Fatal("Expected cache from ctx1")
	}

	// Get cache from second context
	cache2 := Get(ctx2)
	if cache2 == nil {
		t.Fatal("Expected cache from ctx2")
	}

	// Verify they share the same underlying cache storage by adding data
	// to cache1 and checking it appears in cache2
	testParams := []pipelinev1.Param{
		{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://example.com"}},
	}
	testResource := &mockResolvedResource{data: []byte("test-data")}

	// Add to cache1
	cache1.Add("test-resolver", testParams, testResource)

	// Verify it exists in cache2 (proving they share the same underlying storage)
	retrieved, found := cache2.Get("test-resolver", testParams)
	if !found {
		t.Fatal("Expected to find resource in cache2 that was added to cache1 - caches are not shared")
	}

	if string(retrieved.Data()) != string(testResource.Data()) {
		t.Errorf("Expected data %q, got %q", string(testResource.Data()), string(retrieved.Data()))
	}
}
