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
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

// TestTaskRunPipelineRunCancel cancels a PipelineRun and verifies TaskRun statuses and Pod deletions.
func TestTaskRunPipelineRunCancel(t *testing.T) {
	// We run the test twice, once with a PipelineTask configured to retry
	// on failure, to ensure that cancelling the PipelineRun doesn't cause
	// the retrying TaskRun to retry.
	for _, numRetries := range []int{0, 1} {
		numRetries := numRetries // capture range variable
		specStatus := v1.PipelineRunSpecStatusCancelled
		t.Run(fmt.Sprintf("retries=%d,status=%s", numRetries, specStatus), func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			requirements := []func(context.Context, *testing.T, *clients, string){}
			c, namespace := setup(ctx, t, requirements...)
			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    tasks:
    - name: task
      retries: %d
      taskSpec:
        steps:
        - image: busybox
          script: 'sleep 5000'
`, helpers.ObjectNameForTest(t), namespace, numRetries))

			t.Logf("Creating PipelineRun in namespace %s", namespace)
			if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
			}

			t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, Running(pipelineRun.Name), "PipelineRunRunning", v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRun.Name, err)
			}

			taskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
			}

			var wg sync.WaitGroup
			t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", pipelineRun.Name, namespace)
			for _, taskrunItem := range taskrunList.Items {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(ctx, c, name, Running(name), "TaskRunRunning", v1Version)
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be running: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
			}

			patches := []jsonpatch.JsonPatchOperation{{
				Operation: "add",
				Path:      "/spec/status",
				Value:     specStatus,
			}}
			patchBytes, err := json.Marshal(patches)
			if err != nil {
				t.Fatalf("failed to marshal patch bytes in order to cancel")
			}
			if _, err := c.V1PipelineRunClient.Patch(ctx, pr.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
				t.Fatalf("Failed to patch PipelineRun `%s` with cancellation: %s", pipelineRun.Name, err)
			}

			expectedReason := v1.PipelineRunReasonCancelled.String()
			expectedCondition := FailedWithReason(expectedReason, pipelineRun.Name)
			t.Logf("Waiting for PipelineRun %s in namespace %s to be cancelled", pipelineRun.Name, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, timeout, expectedCondition, expectedReason, v1Version); err != nil {
				t.Errorf("Error waiting for PipelineRun %q to finished: %s", pipelineRun.Name, err)
			}

			t.Logf("Waiting for TaskRuns in PipelineRun %s in namespace %s to be cancelled", pipelineRun.Name, namespace)
			for _, taskrunItem := range taskrunList.Items {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(ctx, c, name, FailedWithReason("TaskRunCancelled", name), "TaskRunCancelled", v1Version)
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be finished: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			var trName []string
			taskrunList, err = c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
			}
			for _, taskrunItem := range taskrunList.Items {
				trName = append(trName, taskrunItem.Name)
				if taskrunItem.Spec.Status != v1.TaskRunSpecStatusCancelled {
					t.Fatalf("Status is %s while it should have been %s", taskrunItem.Spec.Status, v1.TaskRunSpecStatusCancelled)
				}
				if taskrunItem.Spec.StatusMessage != v1.TaskRunCancelledByPipelineMsg {
					t.Fatalf("Status message is set to %s while it should be %s.", taskrunItem.Spec.StatusMessage, v1.TaskRunCancelledByPipelineMsg)
				}
			}

			matchKinds := map[string][]string{"PipelineRun": {pipelineRun.Name}}
			// Expected failure events: 1 for the pipelinerun cancel
			expectedNumberOfEvents := 1
			t.Logf("Making sure %d events were created from pipelinerun with kinds %v", expectedNumberOfEvents, matchKinds)
			events, err := collectMatchingEvents(ctx, c.KubeClient, namespace, matchKinds, "Failed")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) < expectedNumberOfEvents {
				collectedEvents := make([]string, 0, len(events))
				for _, event := range events {
					collectedEvents = append(collectedEvents, fmt.Sprintf("%#v", event))
				}
				t.Fatalf("Expected %d number of failed events from pipelinerun but got %d; list of received events : %s", expectedNumberOfEvents, len(events), strings.Join(collectedEvents, ", "))
			}
			matchKinds = map[string][]string{"TaskRun": trName}
			// Expected failure events: 1 for each TaskRun
			expectedNumberOfEvents = len(trName)
			t.Logf("Making sure %d events were created from taskruns with kinds %v", expectedNumberOfEvents, matchKinds)
			events, err = collectMatchingEvents(ctx, c.KubeClient, namespace, matchKinds, "Failed")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) < expectedNumberOfEvents {
				collectedEvents := make([]string, 0, len(events))
				for _, event := range events {
					collectedEvents = append(collectedEvents, fmt.Sprintf("%#v", event))
				}
				t.Fatalf("Expected %d number of failed events from taskrun but got %d; list of received events : %s", expectedNumberOfEvents, len(events), strings.Join(collectedEvents, ", "))
			}
		})
	}
}
