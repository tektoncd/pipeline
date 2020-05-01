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
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	jsonpatch "gomodules.xyz/jsonpatch/v2"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	knativetest "knative.dev/pkg/test"
)

// TestTaskRunPipelineRunCancel is an integration test that will
// verify that pipelinerun cancel lead to the the correct TaskRun statuses
// and pod deletions.
func TestTaskRunPipelineRunCancel(t *testing.T) {
	type tests struct {
		name    string
		retries bool
	}

	tds := []tests{
		{
			name:    "With retries",
			retries: true,
		}, {
			name:    "No retries",
			retries: false,
		},
	}

	t.Parallel()

	for _, tdd := range tds {
		t.Run(tdd.name, func(t *testing.T) {
			tdd := tdd
			pipelineTask := v1beta1.PipelineTask{
				Name:    "foo",
				TaskRef: &v1beta1.TaskRef{Name: "banana"},
			}
			if tdd.retries {
				pipelineTask.Retries = 1
			}

			c, namespace := setup(t)
			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
			defer tearDown(t, c, namespace)

			t.Logf("Creating Task in namespace %s", namespace)
			task := &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: "banana", Namespace: namespace},
				Spec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image:   "ubuntu",
						Command: []string{"/bin/bash"},
						Args:    []string{"-c", "sleep 5000"},
					}}},
				},
			}
			if _, err := c.TaskClient.Create(task); err != nil {
				t.Fatalf("Failed to create Task `banana`: %s", err)
			}

			t.Logf("Creating Pipeline in namespace %s", namespace)
			pipeline := &v1beta1.Pipeline{
				ObjectMeta: metav1.ObjectMeta{Name: "tomatoes", Namespace: namespace},
				Spec: v1beta1.PipelineSpec{
					Tasks: []v1beta1.PipelineTask{pipelineTask},
				},
			}
			if _, err := c.PipelineClient.Create(pipeline); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
			}

			pipelineRun := &v1beta1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{Name: "pear", Namespace: namespace},
				Spec: v1beta1.PipelineRunSpec{
					PipelineRef: &v1beta1.PipelineRef{Name: pipeline.Name},
				},
			}

			t.Logf("Creating PipelineRun in namespace %s", namespace)
			if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
			}

			t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", "pear", namespace)
			if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, Running("pear"), "PipelineRunRunning"); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to be running: %s", "pear", err)
			}

			taskrunList, err := c.TaskRunClient.List(metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=pear"})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", "pear", err)
			}

			var wg sync.WaitGroup
			var trName []string
			t.Logf("Waiting for TaskRuns from PipelineRun %s in namespace %s to be running", "pear", namespace)
			for _, taskrunItem := range taskrunList.Items {
				trName = append(trName, taskrunItem.Name)
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(c, name, Running(name), "TaskRunRunning")
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be running: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			pr, err := c.PipelineRunClient.Get("pear", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("Failed to get PipelineRun `%s`: %s", "pear", err)
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
			if _, err := c.PipelineRunClient.Patch(pr.Name, types.JSONPatchType, patchBytes, ""); err != nil {
				t.Fatalf("Failed to patch PipelineRun `%s` with cancellation: %s", "pear", err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to be cancelled", "pear", namespace)
			if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, FailedWithReason("PipelineRunCancelled", "pear"), "PipelineRunCancelled"); err != nil {
				t.Errorf("Error waiting for PipelineRun `pear` to finished: %s", err)
			}

			t.Logf("Waiting for TaskRuns in PipelineRun %s in namespace %s to be cancelled", "pear", namespace)
			for _, taskrunItem := range taskrunList.Items {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(c, name, FailedWithReason("TaskRunCancelled", name), "TaskRunCancelled")
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be finished: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			matchKinds := map[string][]string{"PipelineRun": {"pear"}, "TaskRun": trName}
			// Expected failure events: 1 for the pipelinerun cancel, 1 for each TaskRun
			expectedNumberOfEvents := 1 + len(trName)
			t.Logf("Making sure %d events were created from pipelinerun with kinds %v", expectedNumberOfEvents, matchKinds)
			events, err := collectMatchingEvents(c.KubeClient, namespace, matchKinds, "Failed")
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
