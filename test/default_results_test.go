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

package test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/system"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestFinallyTaskUsesDefaultResultWhenTaskFails verifies that when a DAG task
// declares a result with a default value but fails before writing that
// result, a finally task consuming the result falls back to the declared
// default instead of leaving the PipelineRun unresolvable.
//
// @test:execution=serial
// @test:reason=modifies enable-default-results field in feature-flags ConfigMap
func TestFinallyTaskUsesDefaultResultWhenTaskFails(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	previous := setDefaultResultsFeatureFlag(ctx, t, c, "true")
	knativetest.CleanupOnInterrupt(func() { setDefaultResultsFeatureFlag(ctx, t, c, previous) }, t.Logf)
	defer setDefaultResultsFeatureFlag(ctx, t, c, previous)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: greeting
          default: "default-greeting"
        steps:
        - name: fail-before-writing-result
          image: mirror.gcr.io/busybox
          script: |
            echo "failing before the result is written"
            exit 1
    finally:
    - name: finaltask1
      params:
      - name: greeting
        value: $(tasks.task1.results.greeting)
      taskSpec:
        params:
        - name: greeting
        steps:
        - name: check-default-is-used
          image: mirror.gcr.io/busybox
          script: |
            echo "finaltask1 received: $(params.greeting)"
            if [ "$(params.greeting)" != "default-greeting" ]; then
              echo "expected default-greeting but got $(params.greeting)"
              exit 1
            fi
`, helpers.ObjectNameForTest(t)))

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to fail", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to fail: %s", err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}
	if len(taskrunList.Items) != 2 {
		t.Fatalf("The PipelineRun %q should have exactly 2 TaskRuns, one for task1 and one for finaltask1, instead it has %d", pipelineRun.Name, len(taskrunList.Items))
	}

	for _, taskrunItem := range taskrunList.Items {
		switch taskrunItem.Labels["tekton.dev/pipelineTask"] {
		case "task1":
			if !isFailed(t, taskrunItem.Name, taskrunItem.Status.Conditions) {
				t.Fatalf("task1 should have failed")
			}
			if len(taskrunItem.Status.Results) != 1 {
				t.Fatalf("task1 should have exactly one result populated from its default, got %d", len(taskrunItem.Status.Results))
			}
			if r := taskrunItem.Status.Results[0]; r.Name != "greeting" || r.Value.StringVal != "default-greeting" {
				t.Fatalf("task1 result should be \"greeting\"=\"default-greeting\", got %q=%q", r.Name, r.Value.StringVal)
			}
		case "finaltask1":
			if !isSuccessful(t, taskrunItem.Name, taskrunItem.Status.Conditions) {
				t.Fatalf("finaltask1 should have succeeded using the default result value")
			}
		default:
			t.Fatalf("Unexpected TaskRun %q for PipelineRun %q", taskrunItem.Name, pipelineRun.Name)
		}
	}
}

// TestFinallyTaskUsesProducedResultOverDefault verifies that when a DAG task
// declares a result with a default value and produces its own value for that
// result before later failing, a finally task consuming the result observes
// the produced value rather than the declared default.
//
// @test:execution=serial
// @test:reason=modifies enable-default-results field in feature-flags ConfigMap
func TestFinallyTaskUsesProducedResultOverDefault(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	previous := setDefaultResultsFeatureFlag(ctx, t, c, "true")
	knativetest.CleanupOnInterrupt(func() { setDefaultResultsFeatureFlag(ctx, t, c, previous) }, t.Logf)
	defer setDefaultResultsFeatureFlag(ctx, t, c, previous)

	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    tasks:
    - name: task1
      taskSpec:
        results:
        - name: greeting
          default: "default-greeting"
        steps:
        - name: produce-result-then-fail
          image: mirror.gcr.io/busybox
          script: |
            echo -n "actual-greeting" | tee $(results.greeting.path)
            exit 1
    finally:
    - name: finaltask1
      params:
      - name: greeting
        value: $(tasks.task1.results.greeting)
      taskSpec:
        params:
        - name: greeting
        steps:
        - name: check-produced-value-is-used
          image: mirror.gcr.io/busybox
          script: |
            echo "finaltask1 received: $(params.greeting)"
            if [ "$(params.greeting)" != "actual-greeting" ]; then
              echo "expected actual-greeting but got $(params.greeting)"
              exit 1
            fi
`, helpers.ObjectNameForTest(t)))

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to fail", pipelineRun.Name, namespace)
	if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, PipelineRunFailed(pipelineRun.Name), "PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to fail: %s", err)
	}

	taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
	if err != nil {
		t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
	}
	if len(taskrunList.Items) != 2 {
		t.Fatalf("The PipelineRun %q should have exactly 2 TaskRuns, one for task1 and one for finaltask1, instead it has %d", pipelineRun.Name, len(taskrunList.Items))
	}

	for _, taskrunItem := range taskrunList.Items {
		switch taskrunItem.Labels["tekton.dev/pipelineTask"] {
		case "task1":
			if !isFailed(t, taskrunItem.Name, taskrunItem.Status.Conditions) {
				t.Fatalf("task1 should have failed")
			}
			if len(taskrunItem.Status.Results) != 1 {
				t.Fatalf("task1 should have exactly one result populated from the step, got %d", len(taskrunItem.Status.Results))
			}
			if r := taskrunItem.Status.Results[0]; r.Name != "greeting" || r.Value.StringVal != "actual-greeting" {
				t.Fatalf("task1 result should be \"greeting\"=\"actual-greeting\", got %q=%q", r.Name, r.Value.StringVal)
			}
		case "finaltask1":
			if !isSuccessful(t, taskrunItem.Name, taskrunItem.Status.Conditions) {
				t.Fatalf("finaltask1 should have succeeded using the produced result value")
			}
		default:
			t.Fatalf("Unexpected TaskRun %q for PipelineRun %q", taskrunItem.Name, pipelineRun.Name)
		}
	}
}

// setDefaultResultsFeatureFlag sets the "enable-default-results" field in the
// feature-flags ConfigMap to value, returning the previous value so callers
// can restore it.
func setDefaultResultsFeatureFlag(ctx context.Context, t *testing.T, c *clients, value string) string {
	t.Helper()
	featureFlagsCM, err := c.KubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(ctx, config.GetFeatureFlagsConfigName(), metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap `%s`: %s", config.GetFeatureFlagsConfigName(), err)
	}
	previous, ok := featureFlagsCM.Data[config.EnableDefaultResults]
	if !ok {
		previous = "false"
	}
	if err := updateConfigMap(ctx, c.KubeClient, system.Namespace(), config.GetFeatureFlagsConfigName(), map[string]string{
		config.EnableDefaultResults: value,
	}); err != nil {
		t.Fatalf("Failed to set %q on the feature-flags ConfigMap: %s", config.EnableDefaultResults, err)
	}
	return previous
}
