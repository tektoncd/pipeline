//go:build e2e

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
	"testing"

	"sync"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"

	"github.com/tektoncd/pipeline/pkg/remoteresolution/resolver/bundle"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	cacheAnnotationKey     = "resolution.tekton.dev/cached"
	cacheResolverTypeKey   = "resolution.tekton.dev/cache-resolver-type"
	cacheTimestampKey      = "resolution.tekton.dev/cache-timestamp"
	cacheOperationKey      = "resolution.tekton.dev/cache-operation"
	cacheValueTrue         = "true"
	cacheOperationStore    = "store"
	cacheOperationRetrieve = "retrieve"
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
// @test:execution=parallel
func TestBundleResolverCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	task := newHelloWorldTask(t, helpers.ObjectNameForTest(t), namespace)
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/test-" + task.Name
	taskRun1 := newBundleTaskRun(t, namespace, "cache-task-run-1", "always", repo, task.Name)
	taskRun2 := newBundleTaskRun(t, namespace, "cache-task-run-2", "always", repo, task.Name)
	expectedStoreAnnotations := newCacheAnnotationMap(cacheOperationStore)
	expectedRetrieveAnnotations := newCacheAnnotationMap(cacheOperationRetrieve)
	setupBundle(ctx, t, c, namespace, repo, task, nil)

	// WHEN
	// First request should have cache annotations (it stores in cache with "always" mode)
	// Second request with same parameters should be cached
	createTaskRunAndWait(ctx, t, c, taskRun1)
	createTaskRunAndWait(ctx, t, c, taskRun2)
	actualTr1 := fetchTaskRunByName(ctx, t, c, taskRun1.Name)
	actualTr2 := fetchTaskRunByName(ctx, t, c, taskRun2.Name)

	// THEN
	assertCacheAnnotations(t, actualTr1.GetAnnotations(), expectedStoreAnnotations)
	assertCacheAnnotations(t, actualTr2.GetAnnotations(), expectedRetrieveAnnotations)
}

func TestBundleResolverCacheSemiParallel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	task := newHelloWorldTask(t, helpers.ObjectNameForTest(t), namespace)
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/test-" + task.Name
	firstTaskRun := newBundleTaskRun(t, namespace, "first-sq-tr", "always", repo, task.Name)
	parallelTaskRuns := newBundleTaskRuns(t, namespace, repo, task.Name, 20, "parallel")
	expectedStoreAnnotations := newCacheAnnotationMap(cacheOperationStore)
	expectedRetrieveAnnotations := newCacheAnnotationMap(cacheOperationRetrieve)
	setupBundle(ctx, t, c, namespace, repo, task, nil)

	// WHEN
	createTaskRunAndWait(ctx, t, c, firstTaskRun)
	actualTr1 := fetchTaskRunByName(ctx, t, c, firstTaskRun.Name)

	createTaskRunsInParallelAndWait(ctx, t, c, parallelTaskRuns)
	actualParallelTrs := fetchTaskRunsByName(ctx, t, c, parallelTaskRuns)

	// THEN
	assertCacheAnnotations(t, actualTr1.GetAnnotations(), expectedStoreAnnotations)
	for _, actualParallelTr := range actualParallelTrs {
		assertCacheAnnotations(t, actualParallelTr.GetAnnotations(), expectedRetrieveAnnotations)
	}
}

func TestBundleResolverCacheAllParallel(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	storeCount := 0
	retrieveCount := 0
	task := newHelloWorldTask(t, helpers.ObjectNameForTest(t), namespace)
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/test-" + task.Name
	taskRunCount := 10
	parallelTaskRuns := newBundleTaskRuns(t, namespace, repo, task.Name, taskRunCount, "parallel")
	expectedAnnotations := map[string]string{
		cacheAnnotationKey:   cacheValueTrue,
		cacheResolverTypeKey: bundle.LabelValueBundleResolverType,
	}
	setupBundle(ctx, t, c, namespace, repo, task, nil)

	// WHEN
	createTaskRunsInParallelAndWait(ctx, t, c, parallelTaskRuns)
	actualParallelTrs := fetchTaskRunsByName(ctx, t, c, parallelTaskRuns)

	// THEN
	for _, actualParallelTr := range actualParallelTrs {
		annotations := actualParallelTr.GetAnnotations()
		if annotations[cacheOperationKey] == cacheOperationStore {
			storeCount++
		}

		if annotations[cacheOperationKey] == cacheOperationRetrieve {
			retrieveCount++
		}

		assertCacheAnnotations(t, actualParallelTr.GetAnnotations(), expectedAnnotations)
	}
	t.Logf("For %d TaskRuns cache store=%d and cache retrieve=%d", taskRunCount, storeCount, retrieveCount)

	if storeCount >= retrieveCount {
		t.Fatalf("Expected retrieve > store operations, got store=%d, retrieve=%d", storeCount, retrieveCount)
	}
}

func newHelloWorldTask(t *testing.T, taskName string, namespace string) *v1beta1.Task {
	result := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: hello
    image: mirror.gcr.io/alpine
    script: 'echo "Hello World!"'
`, taskName, namespace))
	return result
}

func newBundleTaskRun(t *testing.T, namespace, taskRunName, cacheMode, repo, taskName string) *v1.TaskRun {
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
`, taskRunName, namespace, repo, taskName, cacheMode))
}

func newCacheAnnotationMap(operation string) map[string]string {
	result := map[string]string{
		cacheAnnotationKey:   cacheValueTrue,
		cacheResolverTypeKey: bundle.LabelValueBundleResolverType,
		cacheOperationKey:    operation,
	}
	return result
}

func createTaskRunAndWait(ctx context.Context, t *testing.T, c *clients, tr *v1.TaskRun) {
	t.Helper()

	_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun %s: %s", tr.Name, err)
	}

	if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", tr.Name, err)
	}
}

func fetchTaskRunByName(ctx context.Context, t *testing.T, c *clients, taskRunName string) *v1.TaskRun {
	t.Helper()

	result, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Could not fetch actual TaskRun %s: %s", taskRunName, err)
	}

	return result
}

func assertCacheAnnotations(t *testing.T, actual, expected map[string]string) {
	t.Helper()

	assertAnnotationsMatch(t, expected, actual)

	// in e2e tests timestamp can only be tested for existence
	if _, exist := actual[cacheTimestampKey]; !exist {
		t.Errorf("Expected cache timestamp annotation, got: %v", actual[cacheTimestampKey])
	}
}

func newBundleTaskRuns(t *testing.T, namespace string, repo string, taskName string, count int, prefix string) []*v1.TaskRun {
	t.Helper()

	result := make([]*v1.TaskRun, count)
	for i := 0; i < len(result); i++ {
		result[i] = newBundleTaskRun(t, namespace, fmt.Sprintf("%s-%d", prefix, i), "always", repo, taskName)
	}
	return result
}

func createTaskRunsInParallelAndWait(ctx context.Context, t *testing.T, c *clients, taskRuns []*v1.TaskRun) {
	t.Helper()

	var wg sync.WaitGroup
	startSignal := make(chan struct{})
	for _, tr := range taskRuns {
		wg.Add(1)

		go func(tr *v1.TaskRun, begin <-chan struct{}) {
			defer wg.Done()

			<-begin
			createTaskRunAndWait(ctx, t, c, tr)
		}(tr, startSignal)
	}
	close(startSignal)
	wg.Wait()
}

func fetchTaskRunsByName(ctx context.Context, t *testing.T, c *clients, taskRuns []*v1.TaskRun) []*v1.TaskRun {
	t.Helper()

	var result []*v1.TaskRun
	for _, parallelTaskRun := range taskRuns {
		actual := fetchTaskRunByName(ctx, t, c, parallelTaskRun.Name)
		result = append(result, actual)
	}
	return result
}

func hasCacheAnnotation(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}
	cached, exists := annotations[cacheAnnotationKey]
	return exists && cached == cacheValueTrue
}

func newGitCloneBundleTaskRun(t *testing.T, namespace, name, cacheMode string) *v1.TaskRun {
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

// TestCacheIsolationBetweenResolvers validates that cache keys are unique between resolvers
// @test:execution=parallel
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
	tr1 := newGitCloneBundleTaskRun(t, namespace, "isolation-bundle-1", "always")
	createTaskRunAndWait(ctx, t, c, tr1)

	// Test git resolver cache
	tr2 := createGitTaskRunWithCache(t, namespace, "isolation-git-1", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "always")
	createTaskRunAndWait(ctx, t, c, tr2)

	// Test cluster resolver cache
	tr3 := createClusterTaskRun(t, namespace, "isolation-cluster-1", taskName, "always")
	createTaskRunAndWait(ctx, t, c, tr3)

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
// @test:execution=parallel
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
				tr = newGitCloneBundleTaskRun(t, namespace, "config-test-"+tc.name, tc.cacheMode)
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
// @test:execution=parallel
func TestResolverCacheErrorHandling(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test with invalid cache mode (should fail with error due to centralized validation)
	tr1 := newGitCloneBundleTaskRun(t, namespace, "error-test-invalid", "invalid")
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
	tr2 := newGitCloneBundleTaskRun(t, namespace, "error-test-empty", "")
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
// @test:execution=parallel
func TestResolverCacheTTL(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// First request to populate cache
	tr1 := newGitCloneBundleTaskRun(t, namespace, "ttl-test-1", "always")
	_, err := c.V1TaskRunClient.Create(ctx, tr1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create first TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, tr1.Name, TaskRunSucceed(tr1.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for first TaskRun to finish: %s", err)
	}

	// Second request should hit cache
	tr2 := newGitCloneBundleTaskRun(t, namespace, "ttl-test-2", "always")
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
// @test:execution=parallel
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

			tr := newGitCloneBundleTaskRun(t, namespace, fmt.Sprintf("stress-test-%d", index), "always")
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
// @test:execution=parallel
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
	tr := newBundleTaskRun(t, namespace, "invalid-params-test", "malformed-cache-value", repo, taskName)
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
func getResolutionRequest(ctx context.Context, t *testing.T, c *clients, namespace, taskRunName string) *resolutionv1beta1.ResolutionRequest {
	t.Helper()

	// List all ResolutionRequests in the namespace
	resolutionRequests, err := c.V1beta1ResolutionRequestclient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list ResolutionRequests: %v", err)
	}

	// Find the ResolutionRequest that has this TaskRun as an owner
	var mostRecent *resolutionv1beta1.ResolutionRequest
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
