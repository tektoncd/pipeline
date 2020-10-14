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
	"sync"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
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
		t.Run(fmt.Sprintf("retries=%d", numRetries), func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)
			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: helpers.ObjectNameForTest(t), Namespace: namespace},
				Spec: v1beta1.PipelineRunSpec{
					PipelineSpec: &v1beta1.PipelineSpec{
						Tasks: []v1beta1.PipelineTask{{
							Name:    "task",
							Retries: numRetries,
							TaskSpec: &v1beta1.EmbeddedTask{TaskSpec: v1beta1.TaskSpec{
								Steps: []v1beta1.Step{{
									Container: corev1.Container{
										Image: "busybox",
									},
									Script: "sleep 5000",
								}},
							}},
						}},
					},
				},
			}

			t.Logf("Creating PipelineRun in namespace %s", namespace)
			if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", pipelineRun.Name, err)
			}

			t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", pipelineRun.Name, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, pipelineRunTimeout, Running(pipelineRun.Name), "PipelineRunRunning"); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to be running: %s", pipelineRun.Name, err)
			}

			taskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
			}

			var wg sync.WaitGroup
			t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", pipelineRun.Name, namespace)
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

			pr, err := c.PipelineRunClient.Get(ctx, pipelineRun.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun `%s`: %s", pipelineRun.Name, err)
			}

			patches := []jsonpatch.JsonPatchOperation{{
				Operation: "add",
				Path:      "/spec/status",
				Value:     v1beta1.PipelineRunSpecStatusCancelled,
			}}
			patchBytes, err := json.Marshal(patches)
			if err != nil {
				t.Fatalf("failed to marshal patch bytes in order to cancel")
			}
			if _, err := c.PipelineRunClient.Patch(ctx, pr.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
				t.Fatalf("Failed to patch PipelineRun `%s` with cancellation: %s", pipelineRun.Name, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to be cancelled", pipelineRun.Name, namespace)
			if err := WaitForPipelineRunState(ctx, c, pipelineRun.Name, pipelineRunTimeout, FailedWithReason("PipelineRunCancelled", pipelineRun.Name), "PipelineRunCancelled"); err != nil {
				t.Errorf("Error waiting for PipelineRun %q to finished: %s", pipelineRun.Name, err)
			}

			t.Logf("Waiting for TaskRuns in PipelineRun %s in namespace %s to be cancelled", pipelineRun.Name, namespace)
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
			taskrunList, err = c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + pipelineRun.Name})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", pipelineRun.Name, err)
			}
			for _, taskrunItem := range taskrunList.Items {
				trName = append(trName, taskrunItem.Name)
			}

			matchKinds := map[string][]string{"PipelineRun": {pipelineRun.Name}, "TaskRun": trName}
			// Expected failure events: 1 for the pipelinerun cancel, 1 for each TaskRun
			expectedNumberOfEvents := 1 + len(trName)
			t.Logf("Making sure %d events were created from pipelinerun with kinds %v", expectedNumberOfEvents, matchKinds)
			events, err := collectMatchingEvents(ctx, c.KubeClient, namespace, matchKinds, "Failed")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) != expectedNumberOfEvents {
				collectedEvents := ""
				for i, event := range events {
					collectedEvents += fmt.Sprintf("%#v", event)
					if i < (len(events) - 1) {
						collectedEvents += ", "
					}
				}
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of received events : %#v", expectedNumberOfEvents, len(events), collectedEvents)
			}
		})
	}
}
