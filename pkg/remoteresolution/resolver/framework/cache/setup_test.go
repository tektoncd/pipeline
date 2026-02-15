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
	"errors"
	"testing"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	resolutionframework "github.com/tektoncd/pipeline/pkg/resolution/resolver/framework"
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

	resolverType := "test-resolver"
	testParams := []pipelinev1.Param{
		{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://example.com"}},
	}

	const testData = "test-data"
	resolveFn := func() (resolutionframework.ResolvedResource, error) {
		return &mockResolvedResource{data: []byte(testData)}, nil
	}
	resolveFnErr := func() (resolutionframework.ResolvedResource, error) {
		return nil, errors.New("resolution error")
	}

	// Add to cache1
	cache1.GetCachedOrResolveFromRemote(testParams, resolverType, resolveFn)

	// Verify it exists in cache2 (proving they share the same underlying storage)
	retrieved, err := cache2.GetCachedOrResolveFromRemote(testParams, resolverType, resolveFnErr)
	if err != nil {
		t.Fatalf("Expected to find resource in cache2 that was added to cache1 - caches are not shared: %v", err)
	}

	if string(retrieved.Data()) != testData {
		t.Errorf("Expected data %q, got %q", testData, string(retrieved.Data()))
	}
}
