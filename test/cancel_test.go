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
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
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
			var pipelineTask = tb.PipelineTask("foo", "banana")
			if tdd.retries {
				pipelineTask = tb.PipelineTask("foo", "banana", tb.Retries(1))
			}

			c, namespace := setup(t)
			t.Parallel()

			knativetest.CleanupOnInterrupt(func() { tearDown(t, c, namespace) }, t.Logf)
			defer tearDown(t, c, namespace)

			t.Logf("Creating Task in namespace %s", namespace)
			task := tb.Task("banana", namespace, tb.TaskSpec(
				tb.Step("ubuntu", tb.StepCommand("/bin/bash"), tb.StepArgs("-c", "sleep 5000")),
			))
			if _, err := c.TaskClient.Create(task); err != nil {
				t.Fatalf("Failed to create Task `banana`: %s", err)
			}

			t.Logf("Creating Pipeline in namespace %s", namespace)
			pipeline := tb.Pipeline("tomatoes", namespace,
				tb.PipelineSpec(pipelineTask),
			)
			if _, err := c.PipelineClient.Create(pipeline); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", "tomatoes", err)
			}

			pipelineRun := tb.PipelineRun("pear", namespace, tb.PipelineRunSpec(pipeline.Name))

			t.Logf("Creating PipelineRun in namespace %s", namespace)
			if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", "pear", err)
			}

			t.Logf("Waiting for Pipelinerun %s in namespace %s to be started", "pear", namespace)
			if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
				c := pr.Status.GetCondition(apis.ConditionSucceeded)
				if c != nil {
					if c.Status == corev1.ConditionTrue || c.Status == corev1.ConditionFalse {
						return true, errors.New(`pipelineRun "pear" already finished`)
					} else if c.Status == corev1.ConditionUnknown && (c.Reason == "Running" || c.Reason == "Pending") {
						return true, nil
					}
				}
				return false, nil
			}, "PipelineRunRunning"); err != nil {
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
					err := WaitForTaskRunState(c, name, func(tr *v1alpha1.TaskRun) (bool, error) {
						if c := tr.Status.GetCondition(apis.ConditionSucceeded); c != nil {
							if c.IsTrue() || c.IsFalse() {
								return true, fmt.Errorf("taskRun %q already finished", name)
							} else if c.IsUnknown() && (c.Reason == "Running" || c.Reason == "Pending") {
								return true, nil
							}
						}
						return false, nil
					}, "TaskRunRunning")
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

			pr.Spec.Status = v1alpha1.PipelineRunSpecStatusCancelled
			if _, err := c.PipelineRunClient.Update(pr); err != nil {
				t.Fatalf("Failed to cancel PipelineRun `%s`: %s", "pear", err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to be cancelled", "pear", namespace)
			if err := WaitForPipelineRunState(c, "pear", pipelineRunTimeout, func(pr *v1alpha1.PipelineRun) (bool, error) {
				if c := pr.Status.GetCondition(apis.ConditionSucceeded); c != nil {
					if c.IsFalse() {
						if c.Reason == "PipelineRunCancelled" {
							return true, nil
						}
						return true, fmt.Errorf(`pipelineRun "pear" completed with the wrong reason: %s`, c.Reason)
					} else if c.IsTrue() {
						return true, errors.New(`pipelineRun "pear" completed successfully, should have been cancelled`)
					}
				}
				return false, nil
			}, "PipelineRunCancelled"); err != nil {
				t.Errorf("Error waiting for PipelineRun `pear` to finished: %s", err)
			}

			t.Logf("Waiting for TaskRuns in PipelineRun %s in namespace %s to be cancelled", "pear", namespace)
			for _, taskrunItem := range taskrunList.Items {
				wg.Add(1)
				go func(name string) {
					defer wg.Done()
					err := WaitForTaskRunState(c, name, func(tr *v1alpha1.TaskRun) (bool, error) {
						if c := tr.Status.GetCondition(apis.ConditionSucceeded); c != nil {
							if c.IsFalse() {
								if c.Reason == "TaskRunCancelled" {
									return true, nil
								}
								return true, fmt.Errorf("taskRun %q completed with the wrong reason: %s", name, c.Reason)
							} else if c.IsTrue() {
								return true, fmt.Errorf("taskRun %q completed successfully, should have been cancelled", name)
							}
						}
						return false, nil
					}, "TaskRunCancelled")
					if err != nil {
						t.Errorf("Error waiting for TaskRun %s to be finished: %v", name, err)
					}
				}(taskrunItem.Name)
			}
			wg.Wait()

			matchKinds := map[string][]string{"PipelineRun": {"pear"}, "TaskRun": trName}
			expectedNumberOfEvents := 1 + len(trName)
			t.Logf("Making sure %d events were created from pipelinerun with kinds %v", expectedNumberOfEvents, matchKinds)
			events, err := collectMatchingEvents(c.KubeClient, namespace, matchKinds, "Failed")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) != expectedNumberOfEvents {
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of receieved events : %#v", expectedNumberOfEvents, len(events), events)
			}
		})
	}
}
