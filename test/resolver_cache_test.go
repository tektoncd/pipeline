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
			tr := createBundleTaskRun(t, namespace, fmt.Sprintf("config-test-%s", tc.name), tc.cacheMode)
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

func createClusterTaskRun(t *testing.T, namespace, name, taskName, cacheMode string) *v1.TaskRun {
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

func getResolutionRequest(ctx context.Context, t *testing.T, c *clients, namespace, taskRunName string) *v1alpha1.ResolutionRequest {
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
	return exists && cached == "true"
}
