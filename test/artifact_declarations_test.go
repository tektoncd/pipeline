//go:build e2e

/*
Copyright 2026 The Tekton Authors

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

// This file exercises the declarative TEP-0192 Artifact API
// (spec.artifacts.inputs/outputs, PipelineTask artifact bindings) end to
// end, including content transport through a real OCI registry deployed in
// the test namespace. It is distinct from artifacts_test.go, which covers
// the pre-existing TEP-0147 step-artifacts provenance API.
package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var requireEnableArtifactsGate = map[string]string{
	"enable-artifacts": "true",
}

// withArtifactStorage sets the config-artifact-storage ConfigMap to point at
// an in-cluster registry deployed via withRegistry, returning a cleanup
// function that restores the previous configuration. It also enables the
// enable-artifacts feature flag for the duration of the test.
func withArtifactStorage(ctx context.Context, t *testing.T, c *clients, namespace string) func() {
	t.Helper()

	withRegistry(ctx, t, c, namespace)

	registryHost := fmt.Sprintf("registry.%s.svc.cluster.local:5000", namespace)
	repo := registryHost + "/artifacts"

	// Only keys that were actually present before are restored; absent keys
	// are removed rather than reset to "", since an empty (but present)
	// "insecure" value fails strconv.ParseBool and previously could crash
	// the whole controller on config reload.
	previousStorage := map[string]string{}
	if cm, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetArtifactStorageConfigName(), metav1.GetOptions{}); err == nil {
		for _, k := range []string{"oci-repository", "insecure"} {
			if v, ok := cm.Data[k]; ok {
				previousStorage[k] = v
			}
		}
	}
	previousFlags := map[string]string{"enable-artifacts": "false"}
	if cm, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{}); err == nil {
		if v, ok := cm.Data["enable-artifacts"]; ok {
			previousFlags["enable-artifacts"] = v
		}
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetArtifactStorageConfigName(), map[string]string{
		"oci-repository": repo,
		"insecure":       "true",
	}); err != nil {
		t.Fatalf("Failed to set config-artifact-storage: %v", err)
	}
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), requireEnableArtifactsGate); err != nil {
		t.Fatalf("Failed to enable enable-artifacts: %v", err)
	}

	restore := func() {
		cm, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetArtifactStorageConfigName(), metav1.GetOptions{})
		if err == nil {
			for _, k := range []string{"oci-repository", "insecure"} {
				if v, ok := previousStorage[k]; ok {
					cm.Data[k] = v
				} else {
					delete(cm.Data, k)
				}
			}
			_, _ = c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Update(ctx, cm, metav1.UpdateOptions{})
		}
		_ = updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), previousFlags)
	}
	knativetest.CleanupOnInterrupt(restore, t.Logf)
	return restore
}

// TestArtifactContentTransport exercises a "content" artifact produced by
// one Task and consumed by another via a Pipeline-level artifact binding.
// It verifies the entrypoint uploads the artifact to the configured OCI
// registry, that a downstream init container fetches and verifies it, and
// that the consuming Task sees the exact bytes the producer wrote.
//
// @test:execution=serial
// @test:reason=modifies config-artifact-storage and feature-flags ConfigMaps
func TestArtifactContentTransport(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	restore := withArtifactStorage(ctx, t, c, namespace)
	defer restore()

	checkFlagsEnabled := requireAllGates(requireEnableArtifactsGate)
	checkFlagsEnabled(ctx, t, c, namespace)

	const content = "hello from a content artifact"

	produce := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  artifacts:
    outputs:
      - name: greeting
        type: content
  steps:
    - name: produce
      image: %s
      script: |
        mkdir -p $(outputs.greeting.path)
        printf '%s' > $(outputs.greeting.path)/greeting.txt
`, helpers.ObjectNameForTest(t), namespace, getTestImage(busyboxImage), content))
	if _, err := c.V1TaskClient.Create(ctx, produce, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create producer Task: %v", err)
	}

	consume := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  artifacts:
    inputs:
      - name: greeting
  results:
    - name: greeting
  steps:
    - name: consume
      image: %s
      script: |
        cat $(inputs.greeting.path)/greeting.txt | tee $(results.greeting.path)
`, helpers.ObjectNameForTest(t), namespace, getTestImage(busyboxImage)))
	if _, err := c.V1TaskClient.Create(ctx, consume, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create consumer Task: %v", err)
	}

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
    - name: produce
      taskRef:
        name: %s
    - name: consume
      taskRef:
        name: %s
      artifacts:
        inputs:
          - name: greeting
            from: tasks.produce.outputs.greeting
`, helpers.ObjectNameForTest(t), namespace, produce.Name, consume.Name))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline: %v", err)
	}

	prName := helpers.ObjectNameForTest(t)
	pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, prName, namespace, pipeline.Name))
	if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %v", err)
	}

	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSucceeded", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %v", err)
	}

	trs, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=consume", prName),
	})
	if err != nil {
		t.Fatalf("Failed to list consume TaskRuns: %v", err)
	}
	if len(trs.Items) != 1 {
		t.Fatalf("expected exactly one consume TaskRun, got %d", len(trs.Items))
	}
	consumeTR := trs.Items[0]

	var gotResult string
	for _, r := range consumeTR.Status.Results {
		if r.Name == "greeting" {
			gotResult = strings.TrimSpace(r.Value.StringVal)
		}
	}
	if d := cmp.Diff(content, gotResult); d != "" {
		t.Fatalf("consumer did not receive the expected content artifact bytes; diff (-want +got): %s", d)
	}

	// The producer's TaskRun status should record the output artifact with
	// a digest-addressed URI pointing at the configured registry.
	prodTRs, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s,tekton.dev/pipelineTask=produce", prName),
	})
	if err != nil {
		t.Fatalf("Failed to list produce TaskRuns: %v", err)
	}
	if len(prodTRs.Items) != 1 {
		t.Fatalf("expected exactly one produce TaskRun, got %d", len(prodTRs.Items))
	}
	if prodTRs.Items[0].Status.Artifacts == nil || len(prodTRs.Items[0].Status.Artifacts.Outputs) == 0 {
		t.Fatalf("expected the producer TaskRun to record an output artifact, got none")
	}
	gotURI := prodTRs.Items[0].Status.Artifacts.Outputs[0].Values[0].Uri
	if !strings.Contains(gotURI, "@sha256:") {
		t.Errorf("expected the recorded artifact URI to be digest-addressed, got %q", gotURI)
	}
}

// TestArtifactReferenceOutput exercises a "reference" artifact, where the
// step has already stored content elsewhere and only records a
// digest-pinned URI. It verifies Tekton performs no upload and records
// exactly what the step wrote.
//
// @test:execution=serial
// @test:reason=modifies feature-flags ConfigMap
func TestArtifactReferenceOutput(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), requireEnableArtifactsGate); err != nil {
		t.Fatalf("Failed to enable enable-artifacts: %v", err)
	}
	defer func() {
		_ = updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{"enable-artifacts": "false"})
	}()

	const wantURI = "registry.example.com/myapp@sha256:1111111111111111111111111111111111111111111111111111111111111111"

	taskRunName := helpers.ObjectNameForTest(t)
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskSpec:
    artifacts:
      outputs:
        - name: image
          type: reference
          subject: true
    steps:
      - name: build
        image: %s
        script: |
          echo -n %s > $(outputs.image.uri)
`, taskRunName, namespace, getTestImage(busyboxImage), wantURI))
	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %v", err)
	}

	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSucceed", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to finish: %v", err)
	}

	tr, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected TaskRun %s: %v", taskRunName, err)
	}
	if tr.Status.Artifacts == nil || len(tr.Status.Artifacts.Outputs) == 0 {
		t.Fatalf("expected the TaskRun to record a reference output artifact, got none")
	}
	if d := cmp.Diff([]v1.Artifact{{
		Name:   "image",
		Values: []v1.ArtifactValue{{Uri: wantURI}},
	}}, tr.Status.Artifacts.Outputs); d != "" {
		t.Fatalf("recorded reference artifact does not match what the step wrote; diff (-want +got): %s", d)
	}
}
