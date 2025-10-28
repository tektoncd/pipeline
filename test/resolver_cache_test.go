//go:build e2e
// +build e2e

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

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"sync"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"
	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/framework/cache"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	cacheAnnotationKey   = "resolution.tekton.dev/cached"
	cacheResolverTypeKey = "resolution.tekton.dev/cache-resolver-type"
	cacheTimestampKey    = "resolution.tekton.dev/cache-timestamp"
	cacheValueTrue       = "true"
)

var cacheResolverFeatureFlags = requireAllGates(map[string]string{
	"enable-bundles-resolver": "true",
	"enable-api-fields":       "beta",
})

var cacheGitFeatureFlags = requireAllGates(map[string]string{
	"enable-git-resolver": "true",
	"enable-api-fields":   "beta",
})

// getResolverPodLogs gets logs from the tekton-resolvers pod
func getResolverPodLogs(ctx context.Context, t *testing.T, c *clients) string {
	t.Helper()

	resolverNamespace := resolverconfig.ResolversNamespace(system.Namespace())

	// List pods in the resolver namespace
	pods, err := c.KubeClient.CoreV1().Pods(resolverNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=resolvers",
	})
	if err != nil {
		t.Fatalf("Failed to list resolver pods in namespace %s: %v", resolverNamespace, err)
	}

	if len(pods.Items) == 0 {
		t.Fatalf("No resolver pods found in namespace %s", resolverNamespace)
	}

	// Get logs from the first resolver pod
	pod := pods.Items[0]
	req := c.KubeClient.CoreV1().Pods(resolverNamespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	logs, err := req.DoRaw(ctx)
	if err != nil {
		t.Fatalf("Failed to get logs from resolver pod %s: %v", pod.Name, err)
	}

	return string(logs)
}

// TestBundleResolverCache validates that bundle resolver caching works correctly
func TestBundleResolverCache(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	cache.Get(ctx).Clear()

	// Set up local bundle registry with different repositories for each task
	taskName1 := helpers.ObjectNameForTest(t) + "-1"
	taskName2 := helpers.ObjectNameForTest(t) + "-2"
	taskName3 := helpers.ObjectNameForTest(t) + "-3"
	repo1 := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/cachetest-" + helpers.ObjectNameForTest(t) + "-1"
	repo2 := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/cachetest-" + helpers.ObjectNameForTest(t) + "-2"
	repo3 := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/cachetest-" + helpers.ObjectNameForTest(t) + "-3"

	// Create different tasks for each test to ensure unique cache keys
	task1 := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: mirror.gcr.io/alpine
    script: 'echo Hello from cache test 1'
`, taskName1, namespace))

	task2 := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: mirror.gcr.io/alpine
    script: 'echo Hello from cache test 2'
`, taskName2, namespace))

	task3 := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: mirror.gcr.io/alpine
    script: 'echo Hello from cache test 3'
`, taskName3, namespace))

	// Set up the bundles in the local registry
	setupBundle(ctx, t, c, namespace, repo1, task1, nil)
	setupBundle(ctx, t, c, namespace, repo2, task2, nil)
	setupBundle(ctx, t, c, namespace, repo3, task3, nil)

	// Test 1: First request should have cache annotations (it stores in cache with "always" mode)
	tr1 := createBundleTaskRunLocal(t, namespace, "test-task-1", "always", repo1, taskName1)
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	// Wait for completion and verify cache annotations (first request stores in cache with "always" mode)
	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Add a small delay to ensure ResolutionRequest status is fully updated
	time.Sleep(2 * time.Second)

	// Get the resolved resource and verify it's cached (first request stores in cache with "always" mode)
	resolutionRequest1 := getResolutionRequest(ctx, t, c, namespace, tr1.Name)
	if !hasCacheAnnotation(resolutionRequest1.Status.Annotations) {
		t.Errorf("First request should have cache annotations when using cache=always mode. Annotations: %v", resolutionRequest1.Status.Annotations)
	}

	// Test 2: Second request with same parameters should be cached
	tr2 := createBundleTaskRunLocal(t, namespace, "test-task-2", "always", repo1, taskName1)
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create second TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second TaskRun to finish: %s", err)
	}

	// Add a small delay to ensure ResolutionRequest status is fully updated
	time.Sleep(2 * time.Second)

	// Verify it IS cached
	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second request should be cached")
	}

	// Verify cache annotations have correct values
	if resolutionRequest2.Status.Annotations[cacheResolverTypeKey] != "bundles" {
		t.Errorf("Expected resolver type 'bundles', got '%s'", resolutionRequest2.Status.Annotations[cacheResolverTypeKey])
	}

	// Verify timestamp annotation is present
	if timestamp, exists := resolutionRequest2.Status.Annotations[cacheTimestampKey]; !exists || timestamp == "" {
		t.Errorf("Expected cache timestamp annotation, got: %v", resolutionRequest2.Status.Annotations)
	}

	// Verify resolver logs show cache behavior
	logs := getResolverPodLogs(ctx, t, c)

	// Check for cache miss on first request (should see "Cache miss" followed by "Adding to cache")
	if !strings.Contains(logs, "Cache miss") {
		t.Error("Expected to find 'Cache miss' in resolver logs for first request")
	}
	if !strings.Contains(logs, "Adding to cache") {
		t.Error("Expected to find 'Adding to cache' in resolver logs for first request")
	}

	// Check for cache hit on second request
	if !strings.Contains(logs, "Cache hit") {
		t.Error("Expected to find 'Cache hit' in resolver logs for second request")
	}

	// Test 3: Request with different parameters should not be cached
	tr3 := createBundleTaskRunLocal(t, namespace, "test-task-3", "never", repo2, taskName2)
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
	tr1 := createGitTaskRun(t, namespace, "test-git-1", "d76b231a02268ef5d6398f134452b51febd7f084")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", tr1.Name, err)
	}

	// Wait for the first TaskRun to complete
	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %s", err)
	}

	// Second request with same commit should be cached
	tr2 := createGitTaskRun(t, namespace, "test-git-2", "d76b231a02268ef5d6398f134452b51febd7f084")
	_, err = c.V1TaskRunClient.Create(ctx, tr2, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun `%s`: %s", tr2.Name, err)
	}

	if err := WaitForTaskRunState(ctx, c, tr2.Name, TaskRunSucceed(tr2.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for second Git TaskRun to finish: %s", err)
	}

	resolutionRequest2 := getResolutionRequest(ctx, t, c, namespace, tr2.Name)
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("Second git request with same commit should be cached")
	}

	// Verify cache annotations have correct values
	if resolutionRequest2.Status.Annotations[cacheResolverTypeKey] != "git" {
		t.Errorf("Expected resolver type 'git', got '%s'", resolutionRequest2.Status.Annotations[cacheResolverTypeKey])
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

// TestCacheConfiguration validates cache configuration options
func TestResolverCacheConfiguration(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags, cacheGitFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	cache.Get(ctx).Clear()

	// Set up local bundle registry
	taskName := helpers.ObjectNameForTest(t)
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/cachetest-" + helpers.ObjectNameForTest(t)

	// Create a task for the test
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: mirror.gcr.io/alpine
    script: 'echo Hello from config test'
`, taskName, namespace))

	// Set up the bundle in the local registry
	setupBundle(ctx, t, c, namespace, repo, task, nil)

	// Get the digest of the published image
	digest := getImageDigest(ctx, t, c, namespace, repo)
	repoWithDigest := repo + "@" + digest

	// Get a commit hash for git tests
	commitHash := getGitCommitHash(ctx, t, c, namespace, "main")

	testCases := []struct {
		name        string
		cacheMode   string
		shouldCache bool
		description string
	}{
		// Bundle resolver tests
		{"bundle-always", "always", true, "Bundle resolver should cache with always"},
		{"bundle-never", "never", false, "Bundle resolver should not cache with never"},
		{"bundle-auto-no-digest", "auto", false, "Bundle resolver should not cache with auto (no digest)"},
		{"bundle-auto-with-digest", "auto", true, "Bundle resolver should cache with auto (with digest)"},
		{"bundle-default-no-digest", "", false, "Bundle resolver should not cache with default (auto with no digest)"},
		{"bundle-default-with-digest", "", true, "Bundle resolver should cache with default (auto with digest)"},

		// Git resolver tests
		{"git-always", "always", true, "Git resolver should cache with always"},
		{"git-never", "never", false, "Git resolver should not cache with never"},
		{"git-auto-branch", "auto", false, "Git resolver should not cache with auto (branch name)"},
		{"git-auto-commit", "auto", true, "Git resolver should cache with auto (commit hash)"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tr *v1.TaskRun

			switch {
			case strings.HasPrefix(tc.name, "bundle-"):
				// Use digest for positive test cases
				if strings.Contains(tc.name, "with-digest") {
					tr = createBundleTaskRunLocal(t, namespace, "config-test-"+tc.name, tc.cacheMode, repoWithDigest, taskName)
				} else {
					tr = createBundleTaskRunLocal(t, namespace, "config-test-"+tc.name, tc.cacheMode, repo, taskName)
				}
			case strings.HasPrefix(tc.name, "git-"):
				// Use commit hash for positive test cases
				if strings.Contains(tc.name, "commit") {
					tr = createGitTaskRunWithCache(t, namespace, "config-test-"+tc.name, commitHash, tc.cacheMode)
				} else {
					tr = createGitTaskRunWithCache(t, namespace, "config-test-"+tc.name, "main", tc.cacheMode)
				}
			}

			_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create TaskRun: %s", err)
			}

			// Wait for ResolutionRequest with annotations to be available (polling with timeout)
			var resolutionRequest *v1beta1.ResolutionRequest
			timeout := 30 * time.Second
			start := time.Now()

			for time.Since(start) < timeout {
				rr := getResolutionRequest(ctx, t, c, namespace, tr.Name)
				if rr != nil && rr.Status.Data != "" {
					resolutionRequest = rr
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			if resolutionRequest == nil {
				t.Fatalf("ResolutionRequest not found within timeout for TaskRun %s", tr.Name)
			}

			// For cache: never, ResolutionRequest should be created but without cache annotations
			if tc.cacheMode == "never" {
				if resolutionRequest == nil {
					t.Errorf("%s: expected ResolutionRequest but none found", tc.description)
					return
				}
				if hasCacheAnnotation(resolutionRequest.Status.Annotations) {
					t.Errorf("%s: expected no cache annotations for cache: never", tc.description)
				}
				return
			}

			// For other cache modes, we should have a ResolutionRequest
			if resolutionRequest == nil {
				t.Errorf("%s: expected ResolutionRequest but none found", tc.description)
				return
			}

			isCached := hasCacheAnnotation(resolutionRequest.Status.Annotations)

			if isCached != tc.shouldCache {
				t.Errorf("%s: expected cache=%v, got cache=%v", tc.description, tc.shouldCache, isCached)
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

	// Test 1: First request should be cached when cache=always
	tr1 := createClusterTaskRun(t, namespace, "test-cluster-1", taskName, "always")
	_, err = c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	// Wait for completion and verify cache annotation is present
	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Get the resolved resource and verify it IS cached (because cache=always)
	resolutionRequest1 := getResolutionRequest(ctx, t, c, namespace, tr1.Name)
	if !hasCacheAnnotation(resolutionRequest1.Status.Annotations) {
		t.Error("First request should be cached when cache=always")
	}

	// Verify cache annotations have correct values for first request
	if resolutionRequest1.Status.Annotations[cacheResolverTypeKey] != "cluster" {
		t.Errorf("Expected resolver type 'cluster', got '%s'", resolutionRequest1.Status.Annotations[cacheResolverTypeKey])
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
	if resolutionRequest2.Status.Annotations[cacheResolverTypeKey] != "cluster" {
		t.Errorf("Expected resolver type 'cluster', got '%s'", resolutionRequest2.Status.Annotations[cacheResolverTypeKey])
	}

	// Verify timestamp annotation is present
	if timestamp, exists := resolutionRequest2.Status.Annotations[cacheTimestampKey]; !exists || timestamp == "" {
		t.Errorf("Expected cache timestamp annotation, got: %v", resolutionRequest2.Status.Annotations)
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
  params:
  - name: url
    value: "https://github.com/tektoncd/pipeline.git"
  workspaces:
  - name: output
    emptyDir: {}
  taskRef:
    resolver: bundles
    params:
    - name: bundle
      value: ghcr.io/tektoncd/catalog/upstream/tasks/git-clone@sha256:65e61544c5870c8828233406689d812391735fd4100cb444bbd81531cb958bb3
    - name: name
      value: git-clone
    - name: kind
      value: task
    - name: cache
      value: %s
`, name, namespace, cacheMode))
}

func createBundleTaskRunLocal(t *testing.T, namespace, name, cacheMode, repo, taskName string) *v1.TaskRun {
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
      value: %s
    - name: name
      value: %s
    - name: kind
      value: task
    - name: cache
      value: %s
`, name, namespace, repo, taskName, cacheMode))
}

func createGitTaskRun(t *testing.T, namespace, name, revision string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output
      emptyDir: {}
  taskRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: pathInRepo
      value: task/git-clone/0.10/git-clone.yaml
    - name: revision
      value: %s
  params:
    - name: url
      value: https://github.com/tektoncd/pipeline
    - name: deleteExisting
      value: "true"
`, name, namespace, revision))
}

func createGitTaskRunWithCache(t *testing.T, namespace, name, revision, cacheMode string) *v1.TaskRun {
	t.Helper()
	return parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  workspaces:
    - name: output
      emptyDir: {}
  taskRef:
    resolver: git
    params:
    - name: url
      value: https://github.com/tektoncd/catalog.git
    - name: pathInRepo
      value: /task/git-clone/0.10/git-clone.yaml
    - name: revision
      value: %s
    - name: cache
      value: %s
  params:
    - name: url
      value: https://github.com/tektoncd/pipeline
    - name: deleteExisting
      value: "true"
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
	tr1 := createGitTaskRunWithCache(t, namespace, "test-git-always-1", "d76b231a02268ef5d6398f134452b51febd7f084", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same parameters should be cached
	tr2 := createGitTaskRunWithCache(t, namespace, "test-git-always-2", "d76b231a02268ef5d6398f134452b51febd7f084", "always")
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
	if resolutionRequest2.Status.Annotations[cacheResolverTypeKey] != "git" {
		t.Errorf("Expected resolver type 'git', got '%s'", resolutionRequest2.Status.Annotations[cacheResolverTypeKey])
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
	tr1 := createGitTaskRunWithCache(t, namespace, "test-git-never-1", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "never")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same parameters should NOT be cached
	tr2 := createGitTaskRunWithCache(t, namespace, "test-git-never-2", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "never")
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
	tr1 := createGitTaskRunWithCache(t, namespace, "test-git-auto-1", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "auto")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first Git TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first Git TaskRun to finish: %s", err)
	}

	// Second request with same commit should be cached
	tr2 := createGitTaskRunWithCache(t, namespace, "test-git-auto-2", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "auto")
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

// TestCacheIsolationBetweenResolvers validates that cache keys are unique between resolvers
func TestResolverCacheIsolation(t *testing.T) {
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
	tr2 := createGitTaskRunWithCache(t, namespace, "isolation-git-1", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "always")
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
	resolutionRequests, err := c.V1beta1ResolutionRequestclient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list ResolutionRequests: %s", err)
	}

	bundleCacheFound := false
	gitCacheFound := false
	clusterCacheFound := false

	for _, req := range resolutionRequests.Items {
		if req.Namespace == namespace && req.Status.Data != "" && req.Status.Annotations != nil {
			switch req.Status.Annotations[cacheResolverTypeKey] {
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
func TestResolverCacheComprehensive(t *testing.T) {
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
		{"bundle-always", "bundle", "always", true, "Bundle resolver should cache with always"},
		{"bundle-never", "bundle", "never", false, "Bundle resolver should not cache with never"},
		{"bundle-auto", "bundle", "auto", true, "Bundle resolver should cache with auto (has digest)"},
		{"bundle-default", "bundle", "", true, "Bundle resolver should cache with default (auto with digest)"},

		// Git resolver tests
		{"git-always", "git", "always", true, "Git resolver should cache with always"},
		{"git-never", "git", "never", false, "Git resolver should not cache with never"},
		{"git-auto-commit", "git", "auto", true, "Git resolver should cache with auto and commit hash"},
		{"git-auto-branch", "git", "auto", false, "Git resolver should not cache with auto and branch"},

		// Cluster resolver tests
		{"cluster-always", "cluster", "always", true, "Cluster resolver should cache with always"},
		{"cluster-never", "cluster", "never", false, "Cluster resolver should not cache with never"},
		{"cluster-auto", "cluster", "auto", false, "Cluster resolver should not cache with auto"},
		{"cluster-default", "cluster", "", false, "Cluster resolver should not cache with default"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var tr *v1.TaskRun

			switch tc.resolver {
			case "bundle":
				tr = createBundleTaskRun(t, namespace, "config-test-"+tc.name, tc.cacheMode)
			case "git":
				// Use commit hash for auto mode, branch for others
				revision := "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e"
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
func TestResolverCacheErrorHandling(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with invalid cache mode (should fail with error due to centralized validation)
	tr1 := createBundleTaskRun(t, namespace, "error-test-invalid", "invalid")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun with invalid cache mode: %s", err)
	}

	// Should fail due to invalid cache parameter validation
	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunFailed(tr1.Name), "TaskRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to fail: %s", err)
	}

	// Verify it failed due to invalid cache mode
	resolutionRequest1 := getResolutionRequest(ctx, t, c, namespace, tr1.Name)
	if resolutionRequest1.Status.Conditions[0].Status != "False" {
		t.Error("TaskRun with invalid cache mode should fail resolution")
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
func TestResolverCacheTTL(t *testing.T) {
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
func TestResolverCacheStress(t *testing.T) {
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

// TestResolverCacheInvalidParams validates centralized cache parameter validation
func TestResolverCacheInvalidParams(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Set up local bundle registry
	taskName := helpers.ObjectNameForTest(t)
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/cachetest-invalid-" + helpers.ObjectNameForTest(t)

	// Create a task for the test
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: mirror.gcr.io/alpine
    script: 'echo Hello from invalid cache param test'
`, taskName, namespace))

	_, err := c.V1beta1TaskClient.Create(ctx, task, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	setupBundle(ctx, t, c, namespace, repo, task, nil)

	// Test with malformed cache parameter (should fail due to centralized validation)
	tr := createBundleTaskRunLocal(t, namespace, "invalid-params-test", "malformed-cache-value", repo, taskName)
	_, err = c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun with malformed cache parameter: %s", err)
	}

	// Should fail due to invalid cache parameter validation
	if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunFailed(tr.Name), "TaskRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to fail: %s", err)
	}

	// Verify it failed due to invalid cache mode
	resolutionRequest := getResolutionRequest(ctx, t, c, namespace, tr.Name)
	if resolutionRequest.Status.Conditions[0].Status != "False" {
		t.Error("TaskRun with malformed cache parameter should fail resolution")
	}

	t.Logf("Cache invalid parameters test completed successfully")
}

// getResolutionRequest gets the ResolutionRequest for a TaskRun
func getResolutionRequest(ctx context.Context, t *testing.T, c *clients, namespace, taskRunName string) *v1beta1.ResolutionRequest {
	t.Helper()

	// List all ResolutionRequests in the namespace
	resolutionRequests, err := c.V1beta1ResolutionRequestclient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list ResolutionRequests: %v", err)
	}

	// Find the ResolutionRequest that has this TaskRun as an owner
	var mostRecent *v1beta1.ResolutionRequest
	for _, rr := range resolutionRequests.Items {
		// Check if this ResolutionRequest is owned by our TaskRun
		for _, ownerRef := range rr.OwnerReferences {
			if ownerRef.Kind == "TaskRun" && ownerRef.Name == taskRunName {
				if mostRecent == nil || rr.CreationTimestamp.After(mostRecent.CreationTimestamp.Time) {
					mostRecent = &rr
				}
			}
		}
	}

	if mostRecent == nil {
		// No ResolutionRequest found - this might be expected for cache: never
		return nil
	}

	return mostRecent
}

func hasCacheAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	cached, exists := annotations[cacheAnnotationKey]
	return exists && cached == cacheValueTrue
}

// getImageDigest gets the digest of an image from the local registry
func getImageDigest(ctx context.Context, t *testing.T, c *clients, namespace, imageRef string) string {
	t.Helper()

	// Create a pod to run skopeo inspect
	podName := "get-digest-" + helpers.ObjectNameForTest(t)
	po, err := c.KubeClient.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "skopeo",
				Image:   "ghcr.io/tektoncd/catalog/upstream/tasks/skopeo-copy:latest",
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"skopeo inspect --tls-verify=false docker://" + imageRef + " | jq -r '.Digest'"},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create digest pod: %v", err)
	}

	// Wait for pod to complete
	if err := WaitForPodState(ctx, c, po.Name, namespace, func(pod *corev1.Pod) (bool, error) {
		return pod.Status.Phase == "Succeeded", nil
	}, "PodContainersTerminated"); err != nil {
		req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(po.GetName(), &corev1.PodLogOptions{Container: "skopeo"})
		logs, err := req.DoRaw(ctx)
		if err != nil {
			t.Fatalf("Error getting pod logs: %v", err)
		}
		t.Fatalf("Failed to get digest. Pod logs: \n%s", string(logs))
	}

	// Get the digest from pod logs
	req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(po.GetName(), &corev1.PodLogOptions{Container: "skopeo"})
	logs, err := req.DoRaw(ctx)
	if err != nil {
		t.Fatalf("Error getting pod logs: %v", err)
	}

	digest := strings.TrimSpace(string(logs))
	if digest == "" {
		t.Fatalf("Empty digest returned")
	}

	return digest
}

// getGitCommitHash gets the commit hash for a branch in the catalog repository
func getGitCommitHash(ctx context.Context, t *testing.T, c *clients, namespace, branch string) string {
	t.Helper()

	// Create a pod to run git ls-remote
	podName := "get-commit-" + helpers.ObjectNameForTest(t)
	po, err := c.KubeClient.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "git",
				Image:   "alpine/git:latest",
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"git ls-remote https://github.com/tektoncd/catalog.git " + branch + " | cut -f1"},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create git pod: %v", err)
	}

	// Wait for pod to complete
	if err := WaitForPodState(ctx, c, po.Name, namespace, func(pod *corev1.Pod) (bool, error) {
		return pod.Status.Phase == "Succeeded", nil
	}, "PodContainersTerminated"); err != nil {
		req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(po.GetName(), &corev1.PodLogOptions{Container: "git"})
		logs, err := req.DoRaw(ctx)
		if err != nil {
			t.Fatalf("Error getting pod logs: %v", err)
		}
		t.Fatalf("Failed to get commit hash. Pod logs: \n%s", string(logs))
	}

	// Get the commit hash from pod logs
	req := c.KubeClient.CoreV1().Pods(namespace).GetLogs(po.GetName(), &corev1.PodLogOptions{Container: "git"})
	logs, err := req.DoRaw(ctx)
	if err != nil {
		t.Fatalf("Error getting pod logs: %v", err)
	}

	commitHash := strings.TrimSpace(string(logs))
	if commitHash == "" {
		t.Fatalf("Empty commit hash returned")
	}

	return commitHash
}
