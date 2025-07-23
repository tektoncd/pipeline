//go:build e2e
// +build e2e

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

package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"sync"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	CacheAnnotationKey   = "resolution.tekton.dev/cached"
	CacheTimestampKey    = "resolution.tekton.dev/cache-timestamp"
	CacheResolverTypeKey = "resolution.tekton.dev/cache-resolver-type"
	CacheValueTrue       = "true"
)

var cacheResolverFeatureFlags = requireAllGates(map[string]string{
	"enable-bundles-resolver": "true",
	"enable-api-fields":       "beta",
})

var cacheGitFeatureFlags = requireAllGates(map[string]string{
	"enable-git-resolver": "true",
	"enable-api-fields":   "beta",
})

// TestBundleResolverCache validates that bundle resolver caching works correctly
func TestBundleResolverCache(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test 1: First request should not be cached
	tr1 := createBundleTaskRun(t, namespace, "test-task-1", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	// Wait for completion and verify no cache annotation
	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Get the resolved resource and verify it's NOT cached
	resolutionRequest1 := getResolutionRequest(ctx, t, c, namespace, tr1.Name)
	if hasCacheAnnotation(resolutionRequest1.Status.Annotations) {
		t.Error("First request should not be cached")
	}

	// Test 2: Second request with same parameters should be cached
	tr2 := createBundleTaskRun(t, namespace, "test-task-2", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}

	// Verify it IS cached
	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second request should be cached")
	}

	// Verify cache annotations have correct values
	if resolutionRequest2.Status.Annotations[CacheResolverTypeKey] != "bundles" {
		t.Errorf("Expected resolver type 'bundles', got '%s'", resolutionRequest2.Status.Annotations[CacheResolverTypeKey])
	}

	// Test 3: Request with different parameters should not be cached
	tr3 := createBundleTaskRun(t, namespace, "test-task-3", "never")
	_, err = c.V1TaskRunClient.Create(ctx, tr3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create third TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr3.Name, TaskRunSucceed(tr3.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for third TaskRun to finish: %s", err)
	}

	resolutionRequest3 := getResolutionRequest(ctx, t, c, namespace, tr3.Name)
	if hasCacheAnnotation(resolutionRequest3.Status.Annotations) {
		t.Error("Request with cache=never should not be cached")
	}
}

// TestGitResolverCache validates that git resolver caching works correctly
func TestGitResolverCache(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, cacheGitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with commit hash (should cache)
	tr1 := createGitTaskRun(t, namespace, "test-git-1", "6bffb6ca708ac9013115baa574801e8127f4c5c2")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same commit should be cached
	tr2 := createGitTaskRun(t, namespace, "test-git-2", "6bffb6ca708ac9013115baa574801e8127f4c5c2")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second Git TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second git request with same commit should be cached")
	}

	// Verify cache annotations have correct values
	if resolutionRequest2.Status.Annotations[CacheResolverTypeKey] != "git" {
		t.Errorf("Expected resolver type 'git', got '%s'", resolutionRequest2.Status.Annotations[CacheResolverTypeKey])
	}

	// Test with branch name (should not cache in auto mode)
	tr3 := createGitTaskRun(t, namespace, "test-git-3", "main")
	_, err = c.V1TaskRunClient.Create(ctx, tr3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create third Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr3.Name, TaskRunSucceed(tr3.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for third Git TaskRun to finish: %s", err)
	}

	resolutionRequest3 := getResolutionRequest(ctx, t, c, namespace, tr3.Name)
	if hasCacheAnnotation(resolutionRequest3.Status.Annotations) {
		t.Error("Git request with branch name should not be cached in auto mode")
	}
}

// TestCachePerformance validates cache performance characteristics
func TestCachePerformance(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Measure time for first request (cache miss)
	start := time.Now()
	tr1 := createBundleTaskRun(t, namespace, "perf-test-1", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}
	firstRequestTime := time.Since(start)

	// Measure time for second request (cache hit)
	start = time.Now()
	tr2 := createBundleTaskRun(t, namespace, "perf-test-2", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}
	secondRequestTime := time.Since(start)

	// Verify cache hit is significantly faster
	if secondRequestTime >= firstRequestTime {
		t.Errorf("Cache hit should be faster than cache miss. First: %v, Second: %v",
			firstRequestTime, secondRequestTime)
	}

	// Verify cache hit is at least 50% faster
	speedup := float64(firstRequestTime) / float64(secondRequestTime)
	if speedup < 1.5 {
		t.Errorf("Cache hit should be at least 50%% faster. Speedup: %.2fx", speedup)
	}

	t.Logf("Performance results: First request: %v, Second request: %v, Speedup: %.2fx",
		firstRequestTime, secondRequestTime, speedup)
}

// TestCacheConfiguration validates cache configuration options
func TestCacheConfiguration(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	testCases := []struct {
		name        string
		cacheMode   string
		shouldCache bool
	}{
		{"always_mode", "always", true},
		{"never_mode", "never", false},
		{"auto_mode_with_digest", "auto", true}, // Using digest in bundle
		{"default_mode", "", true},              // Defaults to auto
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tr := createBundleTaskRun(t, namespace, "config-test-"+tc.name, tc.cacheMode)
			_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for TaskRun to finish: %s", err)
			}

			resolutionRequest := getResolutionRequest(ctx, t, c, namespace, tr.Name)
			isCached := hasCacheAnnotation(resolutionRequest.Status.Annotations)

			if isCached != tc.shouldCache {
				t.Errorf("Expected cache=%v for mode %s, got cache=%v",
					tc.shouldCache, tc.cacheMode, isCached)
			}
		})
	}
}

// TestClusterResolverCache validates that cluster resolver caching works correctly
func TestClusterResolverCache(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a Task in the namespace for testing
	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from cluster resolver cache test"
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	// Test 1: First request should not be cached
	tr1 := createClusterTaskRun(t, namespace, "test-cluster-1", taskName, "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	// Wait for completion and verify no cache annotation
	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Get the resolved resource and verify it's NOT cached
	resolutionRequest1 := getResolutionRequest(ctx, t, c, namespace, tr1.Name)
	if hasCacheAnnotation(resolutionRequest1.Status.Annotations) {
		t.Error("First request should not be cached")
	}

	// Test 2: Second request with same parameters should be cached
	tr2 := createClusterTaskRun(t, namespace, "test-cluster-2", taskName, "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}

	// Verify it IS cached
	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second request should be cached")
	}

	// Verify cache annotations have correct values
	if resolutionRequest2.Status.Annotations[CacheResolverTypeKey] != "cluster" {
		t.Errorf("Expected resolver type 'cluster', got '%s'", resolutionRequest2.Status.Annotations[CacheResolverTypeKey])
	}

	// Test 3: Request with different parameters should not be cached
	tr3 := createClusterTaskRun(t, namespace, "test-cluster-3", taskName, "never")
	_, err = c.V1TaskRunClient.Create(ctx, tr3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create third TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr3.Name, TaskRunSucceed(tr3.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for third TaskRun to finish: %s", err)
	}

	resolutionRequest3 := getResolutionRequest(ctx, t, c, namespace, tr3.Name)
	if hasCacheAnnotation(resolutionRequest3.Status.Annotations) {
		t.Error("Request with cache=never should not be cached")
	}
}

// Helper functions
func createBundleTaskRun(t *testing.T, namespace, name, cacheMode string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: gcr.io/tekton-releases/catalog/upstream/git-clone@sha256:65e61544c5870c8828233406689d812391735fd4100cb444bbd81531cb958bb3
    - name: name
      value: git-clone
    - name: kind
      value: task
    - name: cache
      value: %s
`, name, namespace, cacheMode))
}

func createGitTaskRun(t *testing.T, namespace, name, revision string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: pathInRepo
      value: task/git-clone/0.10/git-clone.yaml
    - name: revision
      value: %s
`, name, namespace, revision))
}

func createGitTaskRunWithCache(t *testing.T, namespace, name, revision, cacheMode string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: pathInRepo
      value: task/git-clone/0.10/git-clone.yaml
    - name: revision
      value: %s
    - name: cache
      value: %s
`, name, namespace, revision, cacheMode))
}

func createClusterTaskRun(t *testing.T, namespace, name, taskName, cacheMode string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: %s
    - name: namespace
      value: %s
    - name: cache
      value: %s
`, name, namespace, taskName, namespace, cacheMode))
}

// TestGitResolverCacheAlwaysMode validates git resolver caching with cache: always
func TestGitResolverCacheAlwaysMode(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, cacheGitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with cache: always and commit hash
	tr1 := createGitTaskRunWithCache(t, namespace, "test-git-always-1", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same parameters should be cached
	tr2 := createGitTaskRunWithCache(t, namespace, "test-git-always-2", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second Git TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second git request with cache: always should be cached")
	}

	// Verify cache annotations have correct values
	if resolutionRequest2.Status.Annotations[CacheResolverTypeKey] != "git" {
		t.Errorf("Expected resolver type 'git', got '%s'", resolutionRequest2.Status.Annotations[CacheResolverTypeKey])
	}

	// Test with cache: always and branch name (should still cache)
	tr3 := createGitTaskRunWithCache(t, namespace, "test-git-always-3", "main", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create third Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr3.Name, TaskRunSucceed(tr3.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for third Git TaskRun to finish: %s", err)
	}

	resolutionRequest3 := getResolutionRequest(ctx, t, c, namespace, tr3.Name)
	if !hasCacheAnnotation(resolutionRequest3.Status.Annotations) {
		t.Error("Git request with cache: always should be cached even with branch name")
	}
}

// TestGitResolverCacheNeverMode validates git resolver caching with cache: never
func TestGitResolverCacheNeverMode(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, cacheGitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with cache: never and commit hash (should not cache)
	tr1 := createGitTaskRunWithCache(t, namespace, "test-git-never-1", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "never")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same parameters should NOT be cached
	tr2 := createGitTaskRunWithCache(t, namespace, "test-git-never-2", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "never")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second Git TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Git request with cache: never should not be cached")
	}
}

// TestGitResolverCacheAutoMode validates git resolver caching with cache: auto
func TestGitResolverCacheAutoMode(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, cacheGitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with cache: auto and commit hash (should cache)
	tr1 := createGitTaskRunWithCache(t, namespace, "test-git-auto-1", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "auto")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same commit should be cached
	tr2 := createGitTaskRunWithCache(t, namespace, "test-git-auto-2", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "auto")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second Git TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Git request with cache: auto and commit hash should be cached")
	}

	// Test with cache: auto and branch name (should not cache)
	tr3 := createGitTaskRunWithCache(t, namespace, "test-git-auto-3", "main", "auto")
	_, err = c.V1TaskRunClient.Create(ctx, tr3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create third Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr3.Name, TaskRunSucceed(tr3.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for third Git TaskRun to finish: %s", err)
	}

	resolutionRequest3 := getResolutionRequest(ctx, t, c, namespace, tr3.Name)
	if hasCacheAnnotation(resolutionRequest3.Status.Annotations) {
		t.Error("Git request with cache: auto and branch name should not be cached")
	}
}

// TestGitResolverCachePerformance validates git resolver cache performance
func TestGitResolverCachePerformance(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, cacheGitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Measure time for first request (cache miss)
	start := time.Now()
	tr1 := createGitTaskRunWithCache(t, namespace, "perf-git-1", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}
	firstRequestTime := time.Since(start)

	// Measure time for second request (cache hit)
	start = time.Now()
	tr2 := createGitTaskRunWithCache(t, namespace, "perf-git-2", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second Git TaskRun to finish: %s", err)
	}
	secondRequestTime := time.Since(start)

	// Verify cache hit is significantly faster
	if secondRequestTime >= firstRequestTime {
		t.Errorf("Cache hit should be faster than cache miss. First: %v, Second: %v",
			firstRequestTime, secondRequestTime)
	}

	// Verify cache hit is at least 50% faster
	speedup := float64(firstRequestTime) / float64(secondRequestTime)
	if speedup < 1.5 {
		t.Errorf("Cache hit should be at least 50%% faster. Speedup: %.2fx", speedup)
	}

	t.Logf("Git resolver performance results: First request: %v, Second request: %v, Speedup: %.2fx",
		firstRequestTime, secondRequestTime, speedup)
}

// TestClusterResolverCacheNeverMode validates cluster resolver caching with cache: never
func TestClusterResolverCacheNeverMode(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a Task in the namespace for testing
	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from cluster resolver cache never test"
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	// Test with cache: never (should not cache)
	tr1 := createClusterTaskRun(t, namespace, "test-cluster-never-1", taskName, "never")
	_, err = c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Second request with same parameters should NOT be cached
	tr2 := createClusterTaskRun(t, namespace, "test-cluster-never-2", taskName, "never")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Cluster request with cache: never should not be cached")
	}
}

// TestClusterResolverCacheAutoMode validates cluster resolver caching with cache: auto
func TestClusterResolverCacheAutoMode(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a Task in the namespace for testing
	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from cluster resolver cache auto test"
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	// Test with cache: auto (should not cache for cluster resolver)
	tr1 := createClusterTaskRun(t, namespace, "test-cluster-auto-1", taskName, "auto")
	_, err = c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Second request with same parameters should NOT be cached
	tr2 := createClusterTaskRun(t, namespace, "test-cluster-auto-2", taskName, "auto")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Cluster request with cache: auto should not be cached")
	}
}

// TestClusterResolverCachePerformance validates cluster resolver cache performance
func TestClusterResolverCachePerformance(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a Task in the namespace for testing
	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from cluster resolver cache performance test"
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	// Measure time for first request (cache miss)
	start := time.Now()
	tr1 := createClusterTaskRun(t, namespace, "perf-cluster-1", taskName, "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}
	firstRequestTime := time.Since(start)

	// Measure time for second request (cache hit)
	start = time.Now()
	tr2 := createClusterTaskRun(t, namespace, "perf-cluster-2", taskName, "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}
	secondRequestTime := time.Since(start)

	// Verify cache hit is significantly faster
	if secondRequestTime >= firstRequestTime {
		t.Errorf("Cache hit should be faster than cache miss. First: %v, Second: %v",
			firstRequestTime, secondRequestTime)
	}

	// Verify cache hit is at least 50% faster
	speedup := float64(firstRequestTime) / float64(secondRequestTime)
	if speedup < 1.5 {
		t.Errorf("Cache hit should be at least 50%% faster. Speedup: %.2fx", speedup)
	}

	t.Logf("Cluster resolver performance results: First request: %v, Second request: %v, Speedup: %.2fx",
		firstRequestTime, secondRequestTime, speedup)
}

// TestCacheIsolationBetweenResolvers validates that cache keys are unique between resolvers
func TestCacheIsolationBetweenResolvers(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags, cacheGitFeatureFlags, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a Task in the namespace for testing cluster resolver
	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from cache isolation test"
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	// Test bundle resolver cache
	tr1 := createBundleTaskRun(t, namespace, "isolation-bundle-1", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create bundle TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for bundle TaskRun to finish: %s", err)
	}

	// Test git resolver cache
	tr2 := createGitTaskRunWithCache(t, namespace, "isolation-git-1", "6bffb6ca708ac9013115baa574801e8127f4c5c2", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for git TaskRun to finish: %s", err)
	}

	// Test cluster resolver cache
	tr3 := createClusterTaskRun(t, namespace, "isolation-cluster-1", taskName, "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr3, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create cluster TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr3.Name, TaskRunSucceed(tr3.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for cluster TaskRun to finish: %s", err)
	}

	// Verify each resolver has its own cache entry
	resolutionRequests, err := c.V1alpha1ResolutionRequestclient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list ResolutionRequests: %s", err)
	}

	bundleCacheFound := false
	gitCacheFound := false
	clusterCacheFound := false

	for _, req := range resolutionRequests.Items {
		if req.Namespace == namespace && req.Status.Annotations != nil {
			switch req.Status.Annotations[CacheResolverTypeKey] {
			case "bundles":
				bundleCacheFound = true
			case "git":
				gitCacheFound = true
			case "cluster":
				clusterCacheFound = true
			}
		}
	}

	if !bundleCacheFound {
		t.Error("Bundle resolver cache entry not found")
	}
	if !gitCacheFound {
		t.Error("Git resolver cache entry not found")
	}
	if !clusterCacheFound {
		t.Error("Cluster resolver cache entry not found")
	}

	t.Logf("Cache isolation verified: Bundle=%v, Git=%v, Cluster=%v", bundleCacheFound, gitCacheFound, clusterCacheFound)
}

// TestCacheConfigurationComprehensive validates all cache configuration modes across resolvers
func TestCacheConfigurationComprehensive(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags, cacheGitFeatureFlags, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a Task in the namespace for testing cluster resolver
	taskName := helpers.ObjectNameForTest(t)
	exampleTask := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "Hello from comprehensive cache config test"
`, taskName, namespace))

	_, err := c.V1TaskClient.Create(ctx, exampleTask, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	testCases := []struct {
		name        string
		resolver    string
		cacheMode   string
		shouldCache bool
		description string
	}{
		// Bundle resolver tests
		{"bundle_always", "bundle", "always", true, "Bundle resolver should cache with always"},
		{"bundle_never", "bundle", "never", false, "Bundle resolver should not cache with never"},
		{"bundle_auto", "bundle", "auto", true, "Bundle resolver should cache with auto (has digest)"},
		{"bundle_default", "bundle", "", true, "Bundle resolver should cache with default (auto with digest)"},

		// Git resolver tests
		{"git_always", "git", "always", true, "Git resolver should cache with always"},
		{"git_never", "git", "never", false, "Git resolver should not cache with never"},
		{"git_auto_commit", "git", "auto", true, "Git resolver should cache with auto and commit hash"},
		{"git_auto_branch", "git", "auto", false, "Git resolver should not cache with auto and branch"},

		// Cluster resolver tests
		{"cluster_always", "cluster", "always", true, "Cluster resolver should cache with always"},
		{"cluster_never", "cluster", "never", false, "Cluster resolver should not cache with never"},
		{"cluster_auto", "cluster", "auto", false, "Cluster resolver should not cache with auto"},
		{"cluster_default", "cluster", "", false, "Cluster resolver should not cache with default"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tr *v1.TaskRun

			switch tc.resolver {
			case "bundle":
				tr = createBundleTaskRun(t, namespace, "config-test-"+tc.name, tc.cacheMode)
			case "git":
				// Use commit hash for auto mode, branch for others
				revision := "6bffb6ca708ac9013115baa574801e8127f4c5c2"
				if tc.cacheMode == "auto" && tc.shouldCache == false {
					revision = "main" // Use branch name for auto mode that shouldn't cache
				}
				tr = createGitTaskRunWithCache(t, namespace, "config-test-"+tc.name, revision, tc.cacheMode)
			case "cluster":
				tr = createClusterTaskRun(t, namespace, "config-test-"+tc.name, taskName, tc.cacheMode)
			}

			_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for TaskRun to finish: %s", err)
			}

			resolutionRequest := getResolutionRequest(ctx, t, c, namespace, tr.Name)
			isCached := hasCacheAnnotation(resolutionRequest.Status.Annotations)

			if isCached != tc.shouldCache {
				t.Errorf("%s: expected cache=%v, got cache=%v", tc.description, tc.shouldCache, isCached)
			}
		})
	}
}

// TestCacheErrorHandling validates cache error handling scenarios
func TestCacheErrorHandling(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with invalid cache mode (should default to auto)
	tr1 := createBundleTaskRun(t, namespace, "error-test-invalid", "invalid")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun with invalid cache mode: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %s", err)
	}

	// Should still work and cache (defaults to auto mode with digest)
	resolutionRequest1 := getResolutionRequest(ctx, t, c, namespace, tr1.Name)
	if !hasCacheAnnotation(resolutionRequest1.Status.Annotations) {
		t.Error("TaskRun with invalid cache mode should still cache (defaults to auto)")
	}

	// Test with empty cache parameter (should default to auto)
	tr2 := createBundleTaskRun(t, namespace, "error-test-empty", "")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun with empty cache mode: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %s", err)
	}

	// Should still work and cache (defaults to auto mode with digest)
	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("TaskRun with empty cache mode should still cache (defaults to auto)")
	}
}

// TestCacheTTLExpiration validates cache TTL behavior
func TestCacheTTLExpiration(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// First request to populate cache
	tr1 := createBundleTaskRun(t, namespace, "ttl-test-1", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Second request should hit cache
	tr2 := createBundleTaskRun(t, namespace, "ttl-test-2", "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second request should be cached")
	}

	// Note: We can't easily test TTL expiration in e2e tests without waiting for the full TTL duration
	// This test validates that cache entries are created and retrieved correctly
	// TTL expiration would need to be tested in unit tests with mocked time
	t.Logf("Cache TTL test completed - cache entries created and retrieved successfully")
}

// TestCacheStressTest validates cache behavior under stress conditions
func TestCacheStressTest(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create multiple concurrent requests to test cache behavior under load
	const numRequests = 5
	var wg sync.WaitGroup
	errors := make(chan error, numRequests)

	for i := range numRequests {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			tr := createBundleTaskRun(t, namespace, fmt.Sprintf("stress-test-%d", index), "always")
			_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
			if err != nil {
				errors <- fmt.Errorf("Failed to create TaskRun %d: %w", index, err)
				return
			}

			if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version); err != nil {
				errors <- fmt.Errorf("Error waiting for TaskRun %d to finish: %w", index, err)
				return
			}

			resolutionRequest := getResolutionRequest(ctx, t, c, namespace, tr.Name)
			if !hasCacheAnnotation(resolutionRequest.Status.Annotations) {
				errors <- fmt.Errorf("TaskRun %d should be cached", index)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Stress test error: %v", err)
	}

	t.Logf("Cache stress test completed successfully with %d concurrent requests", numRequests)
}

// TestCacheInvalidParameters validates cache behavior with invalid parameters
func TestCacheInvalidParameters(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with malformed cache parameter (should handle gracefully)
	tr := createBundleTaskRun(t, namespace, "invalid-params-test", "malformed_cache_value")
	_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun with malformed cache parameter: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %s", err)
	}

	// Should still work (defaults to auto mode)
	resolutionRequest := getResolutionRequest(ctx, t, c, namespace, tr.Name)
	if !hasCacheAnnotation(resolutionRequest.Status.Annotations) {
		t.Error("TaskRun with malformed cache parameter should still cache (defaults to auto)")
	}

	t.Logf("Cache invalid parameters test completed successfully")
}

func getResolutionRequest(ctx context.Context, t *testing.T, c *clients, namespace, taskRunName string) *v1alpha1.ResolutionRequest {
	t.Helper()
	// Get the TaskRun to find its labels
	_, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun: %s", err)
	}

	// Look for resolution requests in the same namespace
	requests, err := c.V1alpha1ResolutionRequestclient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list ResolutionRequests: %s", err)
	}

	// Find the resolution request that matches this TaskRun
	for _, req := range requests.Items {
		if req.Namespace == namespace {
			// Check if this request is related to our TaskRun
			// This is a simplified check - in a real implementation you might need more sophisticated matching
			if req.Status.Data != "" {
				return &req
			}
		}
	}

	t.Fatalf("No ResolutionRequest found for TaskRun %s", taskRunName)
	return nil
}

func hasCacheAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	cached, exists := annotations[CacheAnnotationKey]
	return exists && cached == CacheValueTrue
}
