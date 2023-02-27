//go:build e2e
// +build e2e

/*
Copyright 2019 The Tekton Authors

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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
	"sigs.k8s.io/yaml"
)

var (
	provenanceFeatureFlags = requireAllGates(map[string]string{
		"enable-provenance-in-status": "true",
	})
	ignoreFeatureFlags = cmpopts.IgnoreFields(v1beta1.Provenance{}, "FeatureFlags")
)

// TestTaskRunPipelineRunStatus is an integration test that will
// verify a very simple "hello world" TaskRun and PipelineRun failure
// execution lead to the correct TaskRun status.
func TestTaskRunPipelineRunStatus(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	t.Logf("Creating Task and TaskRun in namespace %s", namespace)
	task := parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  steps:
  - name: foo
    image: busybox
    command: ['ls', '-la']`, helpers.ObjectNameForTest(t)))
	if _, err := c.V1beta1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}
	taskRun := parse.MustParseV1beta1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  taskRef:
    name: %s
  serviceAccountName: inexistent`, helpers.ObjectNameForTest(t), task.Name))
	if _, err := c.V1beta1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun in namespace %s to fail", namespace)
	if err := WaitForTaskRunState(ctx, c, taskRun.Name, TaskRunFailed(taskRun.Name), "BuildValidationFailed", v1beta1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}

	pipeline := parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  tasks:
  - name: foo
    taskRef:
      name: %s`, helpers.ObjectNameForTest(t), task.Name))
	if _, err := c.V1beta1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipeline.Name, err)
	}
	pipelineRun := parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
  serviceAccountName: inexistent`, helpers.ObjectNameForTest(t), pipeline.Name))
	if _, err := c.V1beta1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun in namespace %s to fail", namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "BuildValidationFailed", v1beta1Version); err != nil {
		t.Errorf("Error waiting for TaskRun to finish: %s", err)
	}
}

// TestProvenanceFieldInPipelineRunTaskRunStatus is an integration test that will
// verify if the provenance field in TaskRun and PipelineRun Status is populated
// with correct data.
// [Setup]: PipelineRun refers to a remote/in-cluster pipeline that will be resolved
// by cluster resolver, and the in-cluster pipeline uses a remote/in-cluster task that
// will also be resolved by cluster resolver.
// [Expectation]: PipelineRun status should contain the provenance about the remote pipeline
// i.e. configsource info, and the child TaskRun status should contain the provnenace
// about the remote task i.e. configsource info .
func TestProvenanceFieldInPipelineRunTaskRunStatus(t *testing.T) {
	ctx := context.Background()
	c, namespace := setupProvenance(ctx, t, clusterFeatureFlags, provenanceFeatureFlags)
	knativetest.CleanupOnInterrupt(func() { unsetProvenanceFlags(ctx, t, c) }, t.Logf)
	defer unsetProvenanceFlags(ctx, t, c)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// example task
	taskName := helpers.ObjectNameForTest(t)
	exampleTask, err := c.V1beta1TaskClient.Create(ctx, getExampleTask(t, taskName, namespace), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}
	taskSpec, err := yaml.Marshal(exampleTask.Spec)
	if err != nil {
		t.Fatalf("couldn't marshal task spec: %v", err)
	}
	expectedTaskRunProvenance := &v1beta1.Provenance{
		ConfigSource: &v1beta1.ConfigSource{
			URI:    fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s@%s", v1beta1.SchemeGroupVersion.String(), namespace, "task", exampleTask.Name, exampleTask.UID),
			Digest: map[string]string{"sha256": sha256CheckSum(taskSpec)},
		},
		FeatureFlags: &config.FeatureFlags{
			EnableProvenanceInStatus: true,
		},
	}

	// example pipeline
	pipelineName := helpers.ObjectNameForTest(t)
	examplePipeline, err := c.V1beta1PipelineClient.Create(ctx, getExamplePipeline(t, pipelineName, taskName, namespace), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}
	pipelineSpec, err := yaml.Marshal(examplePipeline.Spec)
	if err != nil {
		t.Fatalf("couldn't marshal pipeline spec: %v", err)
	}
	expectedPipelineRunProvenance := &v1beta1.Provenance{
		ConfigSource: &v1beta1.ConfigSource{
			URI:    fmt.Sprintf("/apis/%s/namespaces/%s/%s/%s@%s", v1beta1.SchemeGroupVersion.String(), namespace, "pipeline", examplePipeline.Name, examplePipeline.UID),
			Digest: map[string]string{"sha256": sha256CheckSum(pipelineSpec)},
		},
		FeatureFlags: &config.FeatureFlags{
			EnableAPIFields: config.DefaultEnableAPIFields,
		},
	}

	// pipelinerun
	prName := helpers.ObjectNameForTest(t)
	if _, err := c.V1beta1PipelineRunClient.Create(ctx, getExamplePipelineRun(t, prName, pipelineName, namespace), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1beta1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}

	// Get the updated status of the PipelineRun.
	pr, err := c.V1beta1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get PipelineRun %q after it completed: %v", prName, err)
	}

	// check the provenance field in the PipelineRun status
	if d := cmp.Diff(expectedPipelineRunProvenance, pr.Status.Provenance, ignoreFeatureFlags); d != "" {
		t.Errorf("provenance did not match: %s", diff.PrintWantGot(d))
	}
	// ensure that FeatureFlags is not nil
	if pr.Status.Provenance.FeatureFlags == nil {
		t.Error("Expected to get featureflags but got nil")
	}

	// Get the TaskRun name.
	var taskRunName string

	for _, cr := range pr.Status.ChildReferences {
		if cr.Kind == "TaskRun" {
			taskRunName = cr.Name
		}
	}
	if taskRunName == "" {
		t.Fatal("PipelineRun does not have expected TaskRun in .status.childReferences")
	}

	t.Logf("Waiting for TaskRun %s in namespace %s to complete", taskRunName, namespace)
	if err := WaitForTaskRunState(ctx, c, taskRunName, TaskRunSucceed(taskRunName), "TaskRunSuccess", v1beta1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun %s to finish: %s", taskRunName, err)
	}
	// Get the TaskRun.
	taskRun, err := c.V1beta1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get TaskRun %q: %v", taskRunName, err)
	}

	// check the provenance field in the PipelineRun status
	if d := cmp.Diff(expectedTaskRunProvenance, taskRun.Status.Provenance, ignoreFeatureFlags); d != "" {
		t.Errorf("provenance did not match: %s", diff.PrintWantGot(d))
	}
	// ensure that FeatureFlags is not nil
	if taskRun.Status.Provenance.FeatureFlags == nil {
		t.Error("Expected to get featureflags but got nil")
	}
}

func getExampleTask(t *testing.T, taskName, namespace string) *v1beta1.Task {
	t.Helper()
	return parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: HELLO
    default: "hello world!"
  steps:
  - image: ubuntu
    script: |
      #!/usr/bin/env bash
      echo "$(params.HELLO)"
`, taskName, namespace))
}

func getExamplePipeline(t *testing.T, pipelineName, taskName, namespace string) *v1beta1.Pipeline {
	t.Helper()
	return parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1beta1
kind: Pipeline
metadata:
  name: %s
  namespace: %s
spec:
 tasks:
 - name: task1
   taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: %s
    - name: namespace
      value: %s
`, pipelineName, namespace, taskName, namespace))
}

func getExamplePipelineRun(t *testing.T, prName, pipelineName, namespace string) *v1beta1.PipelineRun {
	t.Helper()
	return parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: %s
    - name: namespace
      value: %s
`, prName, namespace, pipelineName, namespace))
}

func sha256CheckSum(input []byte) string {
	h := sha256.New()
	h.Write(input)
	return hex.EncodeToString(h.Sum(nil))
}

func setupProvenance(ctx context.Context, t *testing.T, fn ...func(context.Context, *testing.T, *clients, string)) (*clients, string) {
	t.Helper()
	c, ns := setup(ctx, t)
	configMapData := map[string]string{
		"enable-provenance-in-status": "true",
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
	return c, ns
}

func unsetProvenanceFlags(ctx context.Context, t *testing.T, c *clients) {
	t.Helper()
	configMapData := map[string]string{
		"enable-provenance-in-status": "false",
	}

	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), configMapData); err != nil {
		t.Fatal(err)
	}
}
