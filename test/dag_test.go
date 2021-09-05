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
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
)

// TestDAGPipelineRun creates a graph of arbitrary Tasks, then looks at the corresponding
// TaskRun start times to ensure they were run in the order intended, which is:
//                               |
//                        pipeline-task-1
//                       /               \
//   pipeline-task-2-parallel-1    pipeline-task-2-parallel-2
//                       \                /
//                        pipeline-task-3
//                               |
//                        pipeline-task-4
func TestDAGPipelineRun(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)
	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Create the Task that echoes text
	repoTaskResource := v1beta1.TaskResource{ResourceDeclaration: v1beta1.ResourceDeclaration{
		Name: "repo", Type: resource.PipelineResourceTypeGit,
	}}
	echoTask := &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: "echo-task", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs:  []v1beta1.TaskResource{repoTaskResource},
				Outputs: []v1beta1.TaskResource{repoTaskResource},
			},
			Params: []v1beta1.ParamSpec{{
				Name: "text", Type: v1beta1.ParamTypeString,
				Description: "The text that should be echoed",
			}},
			Steps: []v1beta1.Step{{
				Container: corev1.Container{Image: "busybox"},
				Script:    `echo $(params["text"])`,
			}, {
				Container: corev1.Container{Image: "busybox"},
				Script:    "ln -s $(resources.inputs.repo.path) $(resources.outputs.repo.path)",
			}},
		},
	}
	if _, err := c.TaskClient.Create(ctx, echoTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create echo Task: %s", err)
	}

	// Create the repo PipelineResource (doesn't really matter which repo we use)
	repoResource := &resource.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{Name: "repo"},
		Spec: resource.PipelineResourceSpec{
			Type: resource.PipelineResourceTypeGit,
			Params: []resource.ResourceParam{{
				Name:  "Url",
				Value: "https://github.com/githubtraining/example-basic",
			}},
		},
	}
	if _, err := c.PipelineResourceClient.Create(ctx, repoResource, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}

	// Intentionally declaring Tasks in a mixed up order to ensure the order
	// of execution isn't at all dependent on the order they are declared in
	pipeline := &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: "dag-pipeline", Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{{
				Name: "repo", Type: resource.PipelineResourceTypeGit,
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    "pipeline-task-3",
				TaskRef: &v1beta1.TaskRef{Name: "echo-task"},
				Params: []v1beta1.Param{{
					Name: "text", Value: *v1beta1.NewArrayOrString("wow"),
				}},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "repo", Resource: "repo",
						From: []string{"pipeline-task-2-parallel-1", "pipeline-task-2-parallel-2"},
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name: "repo", Resource: "repo",
					}},
				},
			}, {
				Name:    "pipeline-task-2-parallel-2",
				TaskRef: &v1beta1.TaskRef{Name: "echo-task"},
				Params: []v1beta1.Param{{
					Name: "text", Value: *v1beta1.NewArrayOrString("such parallel"),
				}},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "repo", Resource: "repo",
						From: []string{"pipeline-task-1"},
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name: "repo", Resource: "repo",
					}},
				},
			}, {
				Name:    "pipeline-task-4",
				TaskRef: &v1beta1.TaskRef{Name: "echo-task"},
				Params: []v1beta1.Param{{
					Name: "text", Value: *v1beta1.NewArrayOrString("very cloud native"),
				}},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "repo", Resource: "repo",
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name: "repo", Resource: "repo",
					}},
				},
				RunAfter: []string{"pipeline-task-3"},
			}, {
				Name:    "pipeline-task-2-parallel-1",
				TaskRef: &v1beta1.TaskRef{Name: "echo-task"},
				Params: []v1beta1.Param{{
					Name: "text", Value: *v1beta1.NewArrayOrString("much graph"),
				}},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "repo", Resource: "repo",
						From: []string{"pipeline-task-1"},
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name: "repo", Resource: "repo",
					}},
				},
			}, {
				Name:    "pipeline-task-1",
				TaskRef: &v1beta1.TaskRef{Name: "echo-task"},
				Params: []v1beta1.Param{{
					Name: "text", Value: *v1beta1.NewArrayOrString("how to ci/cd?"),
				}},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "repo", Resource: "repo",
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{{
						Name: "repo", Resource: "repo",
					}},
				},
			}},
		},
	}
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline: %s", err)
	}
	pipelineRun := &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: "dag-pipeline-run", Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: "dag-pipeline"},
			Resources: []v1beta1.PipelineResourceBinding{{
				Name:        "repo",
				ResourceRef: &v1beta1.PipelineResourceRef{Name: "repo"},
			}},
		},
	}
	if _, err := c.PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	t.Logf("Waiting for DAG pipeline to complete")
	if err := WaitForPipelineRunState(ctx, c, "dag-pipeline-run", timeout, PipelineRunSucceed("dag-pipeline-run"), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}

	verifyExpectedOrder(ctx, t, c.TaskRunClient)
}

func verifyExpectedOrder(ctx context.Context, t *testing.T, c clientset.TaskRunInterface) {
	t.Logf("Verifying order of execution")

	taskRunsResp, err := c.List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Couldn't get TaskRuns (so that we could check when they executed): %v", err)
	}
	taskRuns := taskRunsResp.Items
	if len(taskRuns) != 5 {
		t.Fatalf("Expected 5 TaskRuns to have executed but got start times for %d TaskRuns", len(taskRuns))
	}

	sort.Slice(taskRuns, func(i, j int) bool {
		it := taskRuns[i].Status.StartTime.Time
		jt := taskRuns[j].Status.StartTime.Time
		return it.Before(jt)
	})

	wantPrefixes := []string{
		"dag-pipeline-run-pipeline-task-1",
		// Could be task-2-parallel-1 or task-2-parallel-2
		"dag-pipeline-run-pipeline-task-2-parallel",
		"dag-pipeline-run-pipeline-task-2-parallel",
		"dag-pipeline-run-pipeline-task-3",
		"dag-pipeline-run-pipeline-task-4",
	}
	for i, wp := range wantPrefixes {
		if !strings.HasPrefix(taskRuns[i].Name, wp) {
			t.Errorf("Expected task %q to execute first, but %q was first", wp, taskRuns[0].Name)
		}
	}

	// Check that the two tasks that can run in parallel did
	s1 := taskRuns[1].Status.StartTime.Time
	s2 := taskRuns[2].Status.StartTime.Time
	absDiff := time.Duration(math.Abs(float64(s2.Sub(s1))))
	if absDiff > (time.Second * 5) {
		t.Errorf("Expected parallel tasks to execute more or less at the same time, but they were %v apart", absDiff)
	}
}
