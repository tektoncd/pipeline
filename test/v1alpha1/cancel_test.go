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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
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
		t.Run(fmt.Sprintf("retries=%d", numRetries), func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)
			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			pipelineRunName := "cancel-me"
			pipelineRun := &v1alpha1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: pipelineRunName, Namespace: namespace},
				Spec: v1alpha1.PipelineRunSpec{
					PipelineSpec: &v1alpha1.PipelineSpec{
						Tasks: []v1alpha1.PipelineTask{{
							Name:    "task",
							Retries: numRetries,
							TaskSpec: &v1alpha1.TaskSpec{TaskSpec: v1beta1.TaskSpec{
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
			matchKinds := map[string][]string{"PipelineRun": {pipelineRunName}, "TaskRun": trName}
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
