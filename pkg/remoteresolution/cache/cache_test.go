/*
Copyright 2024 The Tekton Authors

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

	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		name         string
		resolverType string
		params       []pipelinev1.Param
		wantErr      bool
	}{
		{
			name:         "empty params",
			resolverType: "http",
			params:       []pipelinev1.Param{},
			wantErr:      false,
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
			wantErr: false,
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
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateCacheKey(tt.resolverType, tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateCacheKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && key == "" {
				t.Error("GenerateCacheKey() returned empty key")
			}
		})
	}
}

func TestResolverCache(t *testing.T) {
	cache := NewResolverCache(DefaultMaxSize)

	// Test adding and getting a value
	key := "test-key"
	value := "test-value"
	cache.Add(key, value)

	if got, ok := cache.Get(key); !ok || got != value {
		t.Errorf("Get() = %v, %v, want %v, true", got, ok, value)
	}

	// Test expiration
	shortExpiration := 100 * time.Millisecond
	cache.AddWithExpiration("expiring-key", "expiring-value", shortExpiration)
	time.Sleep(shortExpiration + 50*time.Millisecond)

	if _, ok := cache.Get("expiring-key"); ok {
		t.Error("Get() returned true for expired key")
	}

	// Test global cache
	globalCache1 := GetGlobalCache()
	globalCache2 := GetGlobalCache()
	if globalCache1 != globalCache2 {
		t.Error("GetGlobalCache() returned different instances")
	}
}
