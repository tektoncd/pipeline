// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipelinerun

import (
	"testing"
	"time"

	trh "github.com/tektoncd/cli/pkg/helper/taskrun"
	"github.com/tektoncd/cli/pkg/test"
	clitest "github.com/tektoncd/cli/pkg/test"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8stest "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
)

func TestTracker_pipelinerun_complete(t *testing.T) {
	var (
		pipelineName = "output-pipeline"
		prName       = "output-pipeline-1"
		ns           = "namespace"

		task1Name = "output-task-1"
		tr1Name   = "output-task-1"
		tr1Pod    = "output-task-1-pod-123456"

		task2Name = "output-task-2"
		tr2Name   = "output-task-2"
		tr2Pod    = "output-task-2-pod-123456"

		allTasks  = []string{}
		onlyTask1 = []string{task1Name}
	)

	scenarios := []struct {
		name     string
		tasks    []string
		expected []trh.Run
	}{
		{
			name:  "for all tasks",
			tasks: allTasks,
			expected: []trh.Run{
				{
					Name: tr1Name,
					Task: task1Name,
				}, {
					Name: tr2Name,
					Task: task2Name,
				},
			},
		},
		{
			name:  "for one task",
			tasks: onlyTask1,
			expected: []trh.Run{
				{
					Name: tr1Name,
					Task: task1Name,
				},
			},
		},
	}

	for _, s := range scenarios {
		taskruns := []*v1alpha1.TaskRun{
			tb.TaskRun(tr1Name, ns,
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(task1Name),
				),
				tb.TaskRunStatus(
					tb.PodName(tr1Pod),
				),
			),
			tb.TaskRun(tr2Name, ns,
				tb.TaskRunSpec(
					tb.TaskRunTaskRef(task2Name),
				),
				tb.TaskRunStatus(
					tb.PodName(tr2Pod),
				),
			),
		}

		initialPR := []*v1alpha1.PipelineRun{
			tb.PipelineRun(prName, ns,
				tb.PipelineRunLabel("tekton.dev/pipeline", prName),
				tb.PipelineRunStatus(
					tb.PipelineRunStatusCondition(apis.Condition{
						Status: corev1.ConditionUnknown,
						Reason: resources.ReasonRunning,
					}),
					tb.PipelineRunTaskRunsStatus(tr1Name, &v1alpha1.PipelineRunTaskRunStatus{
						PipelineTaskName: task1Name,
						Status:           &taskruns[0].Status,
					}),
				),
			),
		}

		prStatusFn := tb.PipelineRunStatus(
			tb.PipelineRunStatusCondition(apis.Condition{
				Status: corev1.ConditionTrue,
				Reason: resources.ReasonSucceeded,
			}),
			tb.PipelineRunTaskRunsStatus(tr1Name, &v1alpha1.PipelineRunTaskRunStatus{
				PipelineTaskName: task1Name,
				Status:           &taskruns[0].Status,
			}),
			tb.PipelineRunTaskRunsStatus(tr2Name, &v1alpha1.PipelineRunTaskRunStatus{
				PipelineTaskName: task2Name,
				Status:           &taskruns[1].Status,
			}),
		)
		pr := &v1alpha1.PipelineRun{}
		prStatusFn(pr)

		tc := startPipelineRun(t, pipelinetest.Data{PipelineRuns: initialPR, TaskRuns: taskruns}, pr.Status)
		tracker := NewTracker(pipelineName, ns, tc)
		output := taskRunsFor(s.tasks, tracker)

		clitest.AssertOutput(t, s.expected, output)
	}
}

func taskRunsFor(onlyTasks []string, tracker *Tracker) []trh.Run {
	output := []trh.Run{}
	for ts := range tracker.Monitor(onlyTasks) {
		output = append(output, ts...)
	}
	return output
}

func startPipelineRun(t *testing.T, data pipelinetest.Data, prStatus ...v1alpha1.PipelineRunStatus) versioned.Interface {
	cs, _ := test.SeedTestData(t, data)

	// to keep pushing the taskrun over the period(simulate watch)
	watcher := watch.NewFake()
	cs.Pipeline.PrependWatchReactor("pipelineruns", k8stest.DefaultWatchReactor(watcher, nil))

	go func() {
		for _, status := range prStatus {
			time.Sleep(time.Second * 2)
			data.PipelineRuns[0].Status = status
			watcher.Modify(data.PipelineRuns[0])
		}
	}()

	return cs.Pipeline
}
