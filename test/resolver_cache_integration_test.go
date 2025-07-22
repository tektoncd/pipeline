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
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/resolution/v1alpha1"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestCacheAnnotationsIntegration verifies that cache annotations are properly added
// to resolved resources when they are served from cache
func TestCacheAnnotationsIntegration(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, withRegistry, requireAllGates(map[string]string{
		"enable-bundles-resolver": "true",
		"enable-api-fields":       "beta",
	}))

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create a TaskRun that will trigger bundle resolution
	tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: cache-test-taskrun
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
      value: always
`, namespace))

	_, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	// Wait for the TaskRun to complete
	if err := WaitForTaskRunState(ctx, c, tr.Name, TaskRunSucceed(tr.Name), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %s", err)
	}

	// Get the resolution request to check for cache annotations
	resolutionRequests, err := c.V1alpha1ResolutionRequestclient.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to list ResolutionRequests: %s", err)
	}

	// Find the resolution request for our TaskRun
	var foundRequest *v1alpha1.ResolutionRequest
	for _, req := range resolutionRequests.Items {
		if req.Namespace == namespace && req.Status.Data != "" {
			foundRequest = &req
			break
		}
	}

	if foundRequest == nil {
		t.Fatal("No ResolutionRequest found for TaskRun")
	}

	// Check for cache annotations
	annotations := foundRequest.Status.Annotations
	if annotations == nil {
		t.Fatal("ResolutionRequest has no annotations")
	}

	// Verify cache annotation is present
	if cached, exists := annotations["resolution.tekton.dev/cached"]; !exists || cached != "true" {
		t.Errorf("Expected cache annotation 'resolution.tekton.dev/cached=true', got: %v", annotations)
	}

	// Verify resolver type annotation is present
	if resolverType, exists := annotations["resolution.tekton.dev/cache-resolver-type"]; !exists || resolverType != "bundles" {
		t.Errorf("Expected resolver type annotation 'resolution.tekton.dev/cache-resolver-type=bundles', got: %v", annotations)
	}

	// Verify timestamp annotation is present
	if timestamp, exists := annotations["resolution.tekton.dev/cache-timestamp"]; !exists || timestamp == "" {
		t.Errorf("Expected cache timestamp annotation 'resolution.tekton.dev/cache-timestamp', got: %v", annotations)
	}

	t.Logf("Cache annotations verified successfully: %v", annotations)
}
