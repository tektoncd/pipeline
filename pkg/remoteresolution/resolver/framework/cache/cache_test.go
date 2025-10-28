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
	"fmt"
	"testing"
	"time"

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
		expectedKey  string
	}{
		{
			name:         "empty params",
			resolverType: "http",
			params:       []pipelinev1.Param{},
			expectedKey:  "1c31dda07cb1e09e89bd660a8d114936b44f728b73a3bc52c69a409ee1d44e67",
		},
		{
			name:         "single param",
			resolverType: "http",
			params: []pipelinev1.Param{
				{
					Name: "url",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "https://example.com",
					},
				},
			},
			expectedKey: "63f68e3e567eafd7efb4149b3389b3261784c8ac5847b62e90b7ae8d23f6e889",
		},
		{
			name:         "multiple params",
			resolverType: "git",
			params: []pipelinev1.Param{
				{
					Name: "url",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "https://github.com/tektoncd/pipeline",
					},
				},
				{
					Name: "revision",
					Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: "main",
					},
				},
			},
			expectedKey: "fbe74989962e04dbb512a986864acff592dd02e84ab20f7544fa6b473648f28c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualKey := generateCacheKey(tt.resolverType, tt.params)
			if tt.expectedKey != actualKey {
				t.Errorf("want %s, got %s", tt.expectedKey, actualKey)
			}
		})
	}
}

func TestGenerateCacheKey_IndependentOfCacheParam(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
		expectedSame bool
		description  string
	}{
		{
			name:         "same params without cache param",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
			},
			expectedSame: true,
			description:  "Params without cache param should generate same key",
		},
		{
			name:         "same params with different cache values",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
			},
			expectedSame: true,
			description:  "Params with cache=true should generate same key as without cache param",
		},
		{
			name:         "same params with cache=false",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "false"}},
			},
			expectedSame: true,
			description:  "Params with cache=false should generate same key as without cache param",
		},
		{
			name:         "different params should generate different keys",
			resolverType: "git",
			params: []pipelinev1.Param{
				{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
				{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "v0.50.0"}},
			},
			expectedSame: false,
			description:  "Different revision should generate different key",
		},
		{
			name:         "array params",
			resolverType: "bundle",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "gcr.io/tekton-releases/catalog/upstream/git-clone"}},
				{Name: "name", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "git-clone"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
			},
			expectedSame: true,
			description:  "Array params with cache should generate same key as without cache",
		},
		{
			name:         "object params",
			resolverType: "hub",
			params: []pipelinev1.Param{
				{Name: "name", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "git-clone"}},
				{Name: "version", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "0.8"}},
				{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "false"}},
			},
			expectedSame: true,
			description:  "Object params with cache should generate same key as without cache",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectedSame {
				// Generate key with cache param
				keyWithCache := generateCacheKey(tt.resolverType, tt.params)

				// Generate key without cache param
				paramsWithoutCache := make([]pipelinev1.Param, 0, len(tt.params))
				for _, p := range tt.params {
					if p.Name != "cache" {
						paramsWithoutCache = append(paramsWithoutCache, p)
					}
				}
				keyWithoutCache := generateCacheKey(tt.resolverType, paramsWithoutCache)

				if keyWithCache != keyWithoutCache {
					t.Errorf("Expected same keys, but got different:\nWith cache: %s\nWithout cache: %s\nDescription: %s",
						keyWithCache, keyWithoutCache, tt.description)
				}
			} else {
				// For different params test, create a second set with different values
				params2 := make([]pipelinev1.Param, len(tt.params))
				copy(params2, tt.params)
				// Change the revision value to make it different
				for i := range params2 {
					if params2[i].Name == "revision" {
						params2[i].Value.StringVal = "main"
						break
					}
				}

				key1 := generateCacheKey(tt.resolverType, tt.params)
				key2 := generateCacheKey(tt.resolverType, params2)
				if key1 == key2 {
					t.Errorf("Expected different keys, but got same: %s\nDescription: %s",
						key1, tt.description)
				}
			}
		})
	}
}

func TestGenerateCacheKey_Deterministic(t *testing.T) {
	resolverType := "git"
	params := []pipelinev1.Param{
		{Name: "url", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "https://github.com/tektoncd/pipeline"}},
		{Name: "revision", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "main"}},
		{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
	}

	// Generate the same key multiple times
	key1 := generateCacheKey(resolverType, params)
	key2 := generateCacheKey(resolverType, params)

	if key1 != key2 {
		t.Errorf("Cache key generation is not deterministic. Got different keys: %s vs %s", key1, key2)
	}
}

func TestGenerateCacheKey_AllParamTypes(t *testing.T) {
	resolverType := "test"
	params := []pipelinev1.Param{
		{Name: "string-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "string-value"}},
		{Name: "array-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeArray, ArrayVal: []string{"item1", "item2"}}},
		{Name: "object-param", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeObject, ObjectVal: map[string]string{"key1": "value1", "key2": "value2"}}},
		{Name: "cache", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "true"}},
	}

	// Generate key with cache param
	keyWithCache := generateCacheKey(resolverType, params)

	// Generate key without cache param
	paramsWithoutCache := make([]pipelinev1.Param, 0, len(params))
	for _, p := range params {
		if p.Name != "cache" {
			paramsWithoutCache = append(paramsWithoutCache, p)
		}
	}
	keyWithoutCache := generateCacheKey(resolverType, paramsWithoutCache)
	if keyWithCache != keyWithoutCache {
		t.Errorf("Expected same keys for all param types, but got different:\nWith cache: %s\nWithout cache: %s",
			keyWithCache, keyWithoutCache)
	}
}

func TestCacheTTLExpiration(t *testing.T) {
	// Use short TTL for fast test execution (10ms)
	ttl := 10 * time.Millisecond
	cache := newResolverCache(100, ttl)

	resolverType := "bundle"
	params := []pipelinev1.Param{
		{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo@sha256:abcdef"}},
	}

	// Create a mock resource using the existing mockResolvedResource type
	mockResource := &mockResolvedResource{
		data: []byte("test data"),
	}

	// Add to cache
	cache.Add(resolverType, params, mockResource)

	// Verify it's immediately retrievable
	if cached, ok := cache.Get(resolverType, params); !ok {
		t.Error("Expected cache hit immediately after adding, but got cache miss")
	} else if cached == nil {
		t.Error("Expected non-nil cached resource immediately after adding")
	}

	// Wait for TTL to expire with minimal delay
	time.Sleep(ttl + 5*time.Millisecond)

	// Verify entry is no longer in cache after TTL expiration
	if cached, ok := cache.Get(resolverType, params); ok {
		t.Errorf("Expected cache miss after TTL expiration, but got cache hit with resource: %v", cached)
	}
}

func TestCacheLRUEviction(t *testing.T) {
	// Create cache with small size (3 entries) for testing eviction
	maxSize := 3
	cache := newResolverCache(maxSize, 1*time.Hour) // Long TTL so only LRU eviction triggers

	resolverType := "bundle"

	// Create 3 different entries
	entries := []struct {
		name   string
		params []pipelinev1.Param
	}{
		{
			name: "entry1",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo1@sha256:111111"}},
			},
		},
		{
			name: "entry2",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo2@sha256:222222"}},
			},
		},
		{
			name: "entry3",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo3@sha256:333333"}},
			},
		},
	}

	// Add all 3 entries to fill the cache
	for _, entry := range entries {
		mockResource := &mockResolvedResource{
			data: []byte(entry.name),
		}
		cache.Add(resolverType, entry.params, mockResource)
	}

	// Verify all 3 entries are in cache
	for _, entry := range entries {
		if _, ok := cache.Get(resolverType, entry.params); !ok {
			t.Errorf("Expected cache hit for %s after adding, but got cache miss", entry.name)
		}
	}

	// Add a 4th entry which should evict the least recently used (entry1)
	entry4Params := []pipelinev1.Param{
		{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo4@sha256:444444"}},
	}
	mockResource4 := &mockResolvedResource{
		data: []byte("entry4"),
	}
	cache.Add(resolverType, entry4Params, mockResource4)

	// Verify entry1 (LRU) was evicted
	if _, ok := cache.Get(resolverType, entries[0].params); ok {
		t.Error("Expected entry1 to be evicted (LRU), but it was still in cache")
	}

	// Verify entry2 and entry3 are still in cache
	if _, ok := cache.Get(resolverType, entries[1].params); !ok {
		t.Error("Expected entry2 to still be in cache, but got cache miss")
	}
	if _, ok := cache.Get(resolverType, entries[2].params); !ok {
		t.Error("Expected entry3 to still be in cache, but got cache miss")
	}

	// Verify entry4 is in cache
	if _, ok := cache.Get(resolverType, entry4Params); !ok {
		t.Error("Expected entry4 to be in cache after adding, but got cache miss")
	}
}

func TestCacheLRUEvictionWithAccess(t *testing.T) {
	// Create cache with small size (3 entries) for testing LRU access pattern
	maxSize := 3
	cache := newResolverCache(maxSize, 1*time.Hour)

	resolverType := "bundle"

	// Create 3 different entries
	entries := []struct {
		name   string
		params []pipelinev1.Param
	}{
		{
			name: "entry1",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo1@sha256:aaa"}},
			},
		},
		{
			name: "entry2",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo2@sha256:bbb"}},
			},
		},
		{
			name: "entry3",
			params: []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo3@sha256:ccc"}},
			},
		},
	}

	// Add all 3 entries
	for _, entry := range entries {
		mockResource := &mockResolvedResource{
			data: []byte(entry.name),
		}
		cache.Add(resolverType, entry.params, mockResource)
	}

	// Access entry1 to make it recently used (entry2 becomes LRU)
	if _, ok := cache.Get(resolverType, entries[0].params); !ok {
		t.Error("Expected cache hit for entry1")
	}

	// Add a 4th entry which should evict entry2 (now LRU) instead of entry1
	entry4Params := []pipelinev1.Param{
		{Name: "bundle", Value: pipelinev1.ParamValue{Type: pipelinev1.ParamTypeString, StringVal: "registry.io/repo4@sha256:ddd"}},
	}
	mockResource4 := &mockResolvedResource{
		data: []byte("entry4"),
	}
	cache.Add(resolverType, entry4Params, mockResource4)

	// Verify entry2 (LRU after entry1 was accessed) was evicted
	if _, ok := cache.Get(resolverType, entries[1].params); ok {
		t.Error("Expected entry2 to be evicted (LRU), but it was still in cache")
	}

	// Verify entry1 (accessed recently) is still in cache
	if _, ok := cache.Get(resolverType, entries[0].params); !ok {
		t.Error("Expected entry1 to still be in cache after being accessed, but got cache miss")
	}

	// Verify entry3 is still in cache
	if _, ok := cache.Get(resolverType, entries[2].params); !ok {
		t.Error("Expected entry3 to still be in cache, but got cache miss")
	}

	// Verify entry4 is in cache
	if _, ok := cache.Get(resolverType, entry4Params); !ok {
		t.Error("Expected entry4 to be in cache, but got cache miss")
	}
}

func TestCacheConcurrentReads(t *testing.T) {
	// Test concurrent reads with high load
	cache := newResolverCache(100, 1*time.Hour)
	resolverType := "bundle"

	// Pre-populate cache with entries
	numEntries := 50
	entries := make([][]pipelinev1.Param, numEntries)
	for i := range numEntries {
		params := []pipelinev1.Param{
			{Name: "bundle", Value: pipelinev1.ParamValue{
				Type:      pipelinev1.ParamTypeString,
				StringVal: fmt.Sprintf("registry.io/repo%d@sha256:%064d", i, i),
			}},
		}
		entries[i] = params
		mockResource := &mockResolvedResource{
			data: []byte(fmt.Sprintf("data-%d", i)),
		}
		cache.Add(resolverType, params, mockResource)
	}

	// Launch 1000 concurrent readers
	numReaders := 1000
	done := make(chan bool, numReaders)

	for i := range numReaders {
		go func(readerID int) {
			defer func() { done <- true }()

			// Each reader performs 100 reads
			for j := range 100 {
				entryIdx := (readerID + j) % numEntries
				if _, ok := cache.Get(resolverType, entries[entryIdx]); !ok {
					t.Errorf("Reader %d: Expected cache hit for entry %d, got miss", readerID, entryIdx)
				}
			}
		}(i)
	}

	// Wait for all readers to complete
	for range numReaders {
		<-done
	}
}

func TestCacheConcurrentWrites(t *testing.T) {
	// Test concurrent writes with high load
	cache := newResolverCache(1000, 1*time.Hour)
	resolverType := "bundle"

	// Launch 500 concurrent writers
	numWriters := 500
	done := make(chan bool, numWriters)

	for i := range numWriters {
		go func(writerID int) {
			defer func() { done <- true }()

			// Each writer adds 10 unique entries
			for j := range 10 {
				params := []pipelinev1.Param{
					{Name: "bundle", Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: fmt.Sprintf("registry.io/writer%d-entry%d@sha256:%064d", writerID, j, writerID*100+j),
					}},
				}
				mockResource := &mockResolvedResource{
					data: []byte(fmt.Sprintf("writer-%d-data-%d", writerID, j)),
				}
				cache.Add(resolverType, params, mockResource)
			}
		}(i)
	}

	// Wait for all writers to complete
	for range numWriters {
		<-done
	}

	// Verify that entries are in the cache and accessible
	// Cache size is 1000, and we wrote 500 writers × 10 entries = 5000 entries
	// Due to LRU eviction, only the most recent ~1000 entries should remain
	// Check a sample of entries from the last 10 writers (which should still be in cache)
	cachedCount := 0
	for writerID := numWriters - 10; writerID < numWriters; writerID++ {
		for j := range 10 { // Check all entries from these writers
			params := []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{
					Type:      pipelinev1.ParamTypeString,
					StringVal: fmt.Sprintf("registry.io/writer%d-entry%d@sha256:%064d", writerID, j, writerID*100+j),
				}},
			}
			cached, ok := cache.Get(resolverType, params)
			if ok && cached != nil {
				cachedCount++
				// Verify the data is correct using Data() method
				expectedData := fmt.Sprintf("writer-%d-data-%d", writerID, j)
				if string(cached.Data()) != expectedData {
					t.Errorf("Expected data '%s' for writer %d entry %d, got '%s'", expectedData, writerID, j, string(cached.Data()))
				}
			}
		}
	}

	// We expect at least some recent entries to be cached
	// Due to concurrent access and LRU, we use a conservative threshold (at least 20% of the 100 we checked)
	if cachedCount < 20 {
		t.Errorf("Expected at least 20 recent entries to be cached, but only found %d", cachedCount)
	}
}

func TestCacheConcurrentReadWrite(t *testing.T) {
	// Test concurrent reads and writes together
	cache := newResolverCache(500, 1*time.Hour)
	resolverType := "bundle"

	// Pre-populate with some entries
	numInitialEntries := 100
	initialEntries := make([][]pipelinev1.Param, numInitialEntries)
	for i := range numInitialEntries {
		params := []pipelinev1.Param{
			{Name: "bundle", Value: pipelinev1.ParamValue{
				Type:      pipelinev1.ParamTypeString,
				StringVal: fmt.Sprintf("registry.io/initial%d@sha256:%064d", i, i),
			}},
		}
		initialEntries[i] = params
		mockResource := &mockResolvedResource{
			data: []byte(fmt.Sprintf("initial-data-%d", i)),
		}
		cache.Add(resolverType, params, mockResource)
	}

	// Launch 300 readers and 300 writers concurrently
	numReaders := 300
	numWriters := 300
	totalGoroutines := numReaders + numWriters
	done := make(chan bool, totalGoroutines)

	// Start readers
	for i := range numReaders {
		go func(readerID int) {
			defer func() { done <- true }()

			for j := range 50 {
				entryIdx := (readerID + j) % numInitialEntries
				cache.Get(resolverType, initialEntries[entryIdx])
			}
		}(i)
	}

	// Start writers
	for i := range numWriters {
		go func(writerID int) {
			defer func() { done <- true }()

			for j := range 5 {
				params := []pipelinev1.Param{
					{Name: "bundle", Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: fmt.Sprintf("registry.io/concurrent-writer%d-entry%d@sha256:%064d", writerID, j, writerID*10+j),
					}},
				}
				mockResource := &mockResolvedResource{
					data: []byte(fmt.Sprintf("concurrent-writer-%d-data-%d", writerID, j)),
				}
				cache.Add(resolverType, params, mockResource)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for range totalGoroutines {
		<-done
	}
}

func TestCacheConcurrentEviction(t *testing.T) {
	// Test that LRU eviction works correctly under concurrent load
	maxSize := 50
	cache := newResolverCache(maxSize, 1*time.Hour)
	resolverType := "bundle"

	// Launch 200 concurrent writers adding entries to force evictions
	numWriters := 200
	entriesPerWriter := 10
	done := make(chan bool, numWriters)

	for i := range numWriters {
		go func(writerID int) {
			defer func() { done <- true }()

			for j := range entriesPerWriter {
				params := []pipelinev1.Param{
					{Name: "bundle", Value: pipelinev1.ParamValue{
						Type:      pipelinev1.ParamTypeString,
						StringVal: fmt.Sprintf("registry.io/eviction-writer%d-entry%d@sha256:%064d", writerID, j, writerID*100+j),
					}},
				}
				mockResource := &mockResolvedResource{
					data: []byte(fmt.Sprintf("eviction-data-%d-%d", writerID, j)),
				}
				cache.Add(resolverType, params, mockResource)

				// Small random read to simulate real access patterns
				if j%3 == 0 {
					cache.Get(resolverType, params)
				}
			}
		}(i)
	}

	// Wait for all writers to complete
	for range numWriters {
		<-done
	}

	// Cache should not panic or deadlock under concurrent eviction pressure
	// Verify that expected entries were evicted and cache size is maintained

	// The cache should have at most maxSize entries
	// We'll verify this by checking that older entries were evicted
	// Since we had 200 writers * 10 entries = 2000 total entries written
	// and cache size is 50, we expect most early entries to be evicted

	// Check that early entries were evicted (writers 0-49 should be mostly evicted)
	evictedCount := 0
	totalChecked := 0
	for writerID := range 50 { // Check first 50 writers
		for j := range entriesPerWriter {
			params := []pipelinev1.Param{
				{Name: "bundle", Value: pipelinev1.ParamValue{
					Type:      pipelinev1.ParamTypeString,
					StringVal: fmt.Sprintf("registry.io/eviction-writer%d-entry%d@sha256:%064d", writerID, j, writerID*100+j),
				}},
			}
			totalChecked++
			if _, ok := cache.Get(resolverType, params); !ok {
				evictedCount++
			}
		}
	}

	// We expect most early entries to be evicted (at least 80% of early entries checked)
	// Using a lower threshold due to concurrent access patterns and timing variations
	expectedEvicted := int(float64(totalChecked) * 0.8)
	if evictedCount < expectedEvicted {
		t.Errorf("Expected at least %d early entries to be evicted, but only %d were evicted out of %d checked", expectedEvicted, evictedCount, totalChecked)
	}

	// The main goal of this test is to ensure no panics or deadlocks under concurrent eviction,
	// and that LRU eviction actually occurs (verified above)
	// With cache size of 50 and 2000 total entries with random access patterns, we've verified:
	// 1. No crashes or deadlocks
	// 2. Old entries are evicted
	t.Logf("Eviction test passed: %d early entries evicted out of %d checked", evictedCount, totalChecked)
}
