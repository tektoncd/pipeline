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

	"github.com/tektoncd/pipeline/test/parse"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	knativetest "knative.dev/pkg/test"
)

// TestTaskRunPipelineRunCancel cancels a PipelineRun and verifies TaskRun statuses and Pod deletions.
func TestTaskRunPipelineRunCancel(t *testing.T) {
	// We run the test twice, once with a PipelineTask configured to retry
	// on failure, to ensure that cancelling the PipelineRun doesn't cause
	// the retrying TaskRun to retry.
	for _, numRetries := range []int{0, 1} {
		numRetries := numRetries // capture range variable
		t.Run(fmt.Sprintf("retries=%d", numRetries), func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)
			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			pipelineRunName := "cancel-me"
			pipelineRun := parse.MustParseAlphaPipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineSpec:
    tasks:
    - name: task
      retries: %d
      taskSpec:
        steps:
        - image: busybox
          script: 'sleep 5000'
`, pipelineRunName, numRetries))

			t.Logf("Creating PipelineRun in namespace %s", namespace)
			if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRunName, err)
			}

			t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRunName, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRunName, pipelineRunTimeout, Running(pipelineRunName), "PipelineRunRunning"); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRunName, err)
			}

			taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRunName, err)
			}

			var wg sync.WaitGroup
			t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", pipelineRunName, namespace)
			for _, taskrunItem := range taskrunList.Items {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(ctx, c, name, Running(name), "TaskRunRunning")
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be running: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			pr, err := c.PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRunName, err)
			}

			patches := []jsonpatch.JsonPatchOperation{{
				Operation: "add",
				Path:      "/spec/status",
				Value:     v1alpha1.PipelineRunSpecStatusCancelled,
			}}
			patchBytes, err := json.Marshal(patches)
			if err != nil {
				t.Fatalf("failed to marshal patch bytes in order to cancel")
			}
			if _, err := c.PipelineRunClient.Patch(ctx, pr.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
				t.Fatalf("Failed to patch PipelineRun `%s` with cancellation: %s", pipelineRunName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to be cancelled", pipelineRunName, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRunName, pipelineRunTimeout, FailedWithReason("PipelineRunCancelled", pipelineRunName), "PipelineRunCancelled"); err != nil {
				t.Errorf("Error waiting for PipelineRun %q to finished: %s", pipelineRunName, err)
			}

			t.Logf("Waiting for TaskRuns in PipelineRun %s in namespace %s to be cancelled", pipelineRunName, namespace)
			for _, taskrunItem := range taskrunList.Items {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(ctx, c, name, FailedWithReason("TaskRunCancelled", name), "TaskRunCancelled")
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be finished: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			var trName []string
			taskrunList, err = c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRunName})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRunName, err)
			}
			for _, taskrunItem := range taskrunList.Items {
				trName = append(trName, taskrunItem.Name)
			}
			matchKinds := map[string][]string{"PipelineRun": {pipelineRunName}}
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
