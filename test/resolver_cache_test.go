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
	"errors"
	"fmt"
	"strings"
	"testing"

	"sync"

	resolverconfig "github.com/tektoncd/pipeline/pkg/apis/config/resolver"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resolutionv1beta1 "github.com/tektoncd/pipeline/pkg/apis/resolution/v1beta1"

	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"

	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

const (
	cacheAnnotationKey   = "resolution.tekton.dev/cached"
	cacheResolverTypeKey = "resolution.tekton.dev/cache-resolver-type"
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

// @test:execution=parallel
func TestBundleResolverCache(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	replicas := 1
	taskRunCount := 20
	expectedRequests := replicas
	task := newHelloWorldTask(t, helpers.ObjectNameForTest(t), namespace)
	repoName := "test-" + task.Name
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/" + repoName
	parallelTaskRuns := newBundleTaskRuns(t, namespace, repo, task.Name, taskRunCount, "parallel")
	setupBundle(ctx, t, c, namespace, repo, task, nil)

	// WHEN
	createTaskRunsInParallelAndWait(ctx, t, c, parallelTaskRuns)

	// THEN
	assertRegistryRequestCount(ctx, t, c, namespace, repoName, expectedRequests, taskRunCount, replicas)
}

// @test:execution=serial
// @test:reason=scales the shared resolver deployment to 4 replicas, causing concurrent cache tests to see 4 registry fetches instead of the expected 1
// @test:tags=resolver,cache,replicas
func TestBundleResolverCacheWithFourResolverReplicas(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, cacheResolverFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// GIVEN
	replicas := 4
	taskRunCount := 80
	expectedRequests := replicas
	task := newHelloWorldTask(t, helpers.ObjectNameForTest(t), namespace)
	repoName := "test-" + task.Name
	repo := getRegistryServiceIP(ctx, t, c, namespace) + ":5000/" + repoName
	parallelTaskRuns := newBundleTaskRuns(t, namespace, repo, task.Name, taskRunCount, "parallel")
	setupBundle(ctx, t, c, namespace, repo, task, nil)
	scaleResolverDeployment(ctx, t, c, int32(replicas))
	defer scaleResolverDeployment(ctx, t, c, 1)
	t.Logf("Scaled resolver deployment to %d replicas", replicas)

	// WHEN
	createTaskRunsInParallelAndWait(ctx, t, c, parallelTaskRuns)

	// THEN
	assertRegistryRequestCount(ctx, t, c, namespace, repoName, expectedRequests, taskRunCount, replicas)
}

func newHelloWorldTask(t *testing.T, taskName string, namespace string) *v1beta1.Task {
	t.Helper()

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

func createTaskRunAndWait(ctx context.Context, c *clients, tr *v1.TaskRun) error {
	if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create TaskRun: %w", err)
	}

	return WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version)
}

func newBundleTaskRuns(t *testing.T, namespace string, repo string, taskName string, count int, prefix string) []*v1.TaskRun {
	t.Helper()

	result := make([]*v1.TaskRun, count)
	for i := 0; i < len(result); i++ {
		result[i] = newBundleTaskRun(t, namespace, fmt.Sprintf("%s-%d", prefix, i), "always", repo, taskName)
	}
	return result
}

func scaleResolverDeployment(ctx context.Context, t *testing.T, c *clients, replicas int32) {
	t.Helper()

	resolverNS := resolverconfig.ResolversNamespace(system.Namespace())
	deploymentName := "tekton-pipelines-remote-resolvers"

	scale, err := c.KubeClient.AppsV1().Deployments(resolverNS).GetScale(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get scale for deployment %s: %v", deploymentName, err)
	}

	scale.Spec.Replicas = replicas
	if _, err := c.KubeClient.AppsV1().Deployments(resolverNS).UpdateScale(ctx, deploymentName, scale, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to scale deployment %s to %d replicas: %v", deploymentName, replicas, err)
	}

	if err := knativetest.WaitForDeploymentScale(ctx, c.KubeClient, deploymentName, resolverNS, int(replicas)); err != nil {
		t.Fatalf("Error waiting for deployment %s to reach %d ready replicas: %v", deploymentName, replicas, err)
	}
}

func createTaskRunsInParallelAndWait(ctx context.Context, t *testing.T, c *clients, taskRuns []*v1.TaskRun) {
	t.Helper()

	var wg sync.WaitGroup
	startSignal := make(chan struct{})
	errChan := make(chan error, len(taskRuns))
	for _, tr := range taskRuns {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-startSignal
			if err := createTaskRunAndWait(ctx, c, tr); err != nil {
				errChan <- err
			}
		}()
	}
	close(startSignal)
	wg.Wait()
	close(errChan)

	var allTaskRunErrs []error
	for taskRunErr := range errChan {
		allTaskRunErrs = append(allTaskRunErrs, taskRunErr)
	}
	if joinedErrs := errors.Join(allTaskRunErrs...); joinedErrs != nil {
		t.Fatal(joinedErrs)
	}
}

func assertRegistryRequestCount(ctx context.Context, t *testing.T, c *clients, namespace string, repoName string, expectedRequests int, taskRunCount int, replicas int) {
	t.Helper()

	actualRequestsFromLogs := countManifestGetRequestsInRegistryLogs(ctx, t, c, namespace, repoName)
	if expectedRequests != actualRequestsFromLogs {
		t.Errorf(
			"Caching not working as expected. Expected %d registry requests with %d resolver replicas, got %d",
			expectedRequests, replicas, actualRequestsFromLogs,
		)
	}

	actualRequestsFromMetrics, err := manifestGetRequestCountFromRegistryMetrics(ctx, t, c, namespace)
	if err != nil {
		t.Errorf("Error occurred during metrics gathering: %v", err)
	}

	if expectedRequests != actualRequestsFromMetrics {
		t.Errorf(
			"Caching not working as expected. Expected %d requests from registry metrics, got %d",
			expectedRequests, actualRequestsFromMetrics,
		)
	}

	t.Logf("%s summary: taskRuns=%d, resolverReplicas=%d\n"+
		"  Registry GETs from logs:    expected=%d, actual=%d\n"+
		"  Registry GETs from metrics: expected=%d, actual=%d",
		t.Name(),
		taskRunCount, replicas,
		expectedRequests, actualRequestsFromLogs,
		expectedRequests, actualRequestsFromMetrics)
}

func manifestGetRequestCountFromRegistryMetrics(ctx context.Context, t *testing.T, c *clients, namespace string) (int, error) {
	t.Helper()

	podName := getRegistryPodName(ctx, t, c, namespace)

	result := c.KubeClient.
		CoreV1().
		RESTClient().
		Get().
		Resource("pods").
		Name(podName + ":5001").
		Namespace(namespace).
		SubResource("proxy").
		Suffix("metrics").
		Do(ctx)

	body, err := result.Raw()
	if err != nil {
		return 0, fmt.Errorf("failed to get metrics from pod: %w", err)
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)
	metricFamilies, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse metrics: %w", err)
	}

	totalHttpRequestMetricFamily, ok := metricFamilies["registry_http_requests_total"]
	if !ok {
		return 0, errors.New("metric registry_http_requests_total not found")
	}

	for _, oneHttpRequestMetric := range totalHttpRequestMetricFamily.GetMetric() {
		hasGetManifest := false
		hasGet := false

		for _, label := range oneHttpRequestMetric.GetLabel() {
			if label.GetName() == "handler" && label.GetValue() == "manifest" {
				hasGetManifest = true
			}
			if label.GetName() == "method" && label.GetValue() == "get" {
				hasGet = true
			}
		}

		if hasGetManifest && hasGet {
			return int(oneHttpRequestMetric.GetCounter().GetValue()), nil
		}
	}

	return 0, errors.New("metric with handler=manifest and method=get not found")
}

func countManifestGetRequestsInRegistryLogs(ctx context.Context, t *testing.T, c *clients, namespace, repoName string) int {
	t.Helper()

	podName := getRegistryPodName(ctx, t, c, namespace)

	rawLogs, err := getContainerLogsFromPod(ctx, c.KubeClient, podName, "registry", namespace)
	if err != nil {
		t.Logf("Warning: failed to get registry logs: %v", err)
		return 0
	}

	pattern := fmt.Sprintf("GET /v2/%s/manifests/", repoName)
	return strings.Count(rawLogs, pattern)
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
	if err := createTaskRunAndWait(ctx, c, tr1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test git resolver cache
	tr2 := createGitTaskRunWithCache(t, namespace, "isolation-git-1", "dd7cc22f2965ff4c9d8855b7161c2ffe94b6153e", "always")
	if err := createTaskRunAndWait(ctx, c, tr2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test cluster resolver cache
	tr3 := createClusterTaskRun(t, namespace, "isolation-cluster-1", taskName, "always")
	if err := createTaskRunAndWait(ctx, c, tr3); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
			if resolutionRequest == nil {
				t.Fatal("Expected ResolutionRequest, got nil")
			}

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
	if resolutionRequest1 == nil {
		t.Fatal("Expected ResolutionRequest, got nil")
	}
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
	if resolutionRequest2 == nil {
		t.Fatal("Expected ResolutionRequest, got nil")
	}
	if !hasCacheAnnotation(resolutionRequest2.Status.Annotations) {
		t.Error("TaskRun with empty cache mode should still cache (defaults to auto)")
	}
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
	if resolutionRequest == nil {
		t.Fatal("Expected ResolutionRequest, got nil")
	}
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
