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
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/cli/pkg/cmd/taskrun"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/logs"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
)

func Test_command_has_pipelinerun_arg(t *testing.T) {
	c := Command(&tu.Params{})

	_, err := tu.ExecuteCommand(c, "logs", "-n", "ns")

	if err == nil {
		t.Error("Expecting an error but it's empty")
	}
}

func Test_pipelinerun_not_found(t *testing.T) {
	pr := []*v1alpha1.PipelineRun{
		tb.PipelineRun("output-pipeline-1", "ns",
			tb.PipelineRunLabel("tekton.dev/pipeline", "output-pipeline-1"),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionFalse,
					Reason: resources.ReasonFailed,
				}),
			),
		),
	}
	cs, _ := test.SeedTestData(test.Data{PipelineRuns: pr})
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	c := Command(p)
	got, _ := tu.ExecuteCommand(c, "logs", "output-pipeline-2", "-n", "ns")
	expected := msgPRNotFoundErr + " : pipelineruns.tekton.dev \"output-pipeline-2\" not found \n"

	if d := cmp.Diff(expected, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func Test_print_pipelinerun_logs(t *testing.T) {
	var (
		pipelineName = "output-pipeline"
		prName       = "output-pipeline-1"
		prstart      = clockwork.NewFakeClock()
		ns           = "namespace"

		task1Name    = "output-task"
		tr1Name      = "output-task-1"
		tr1StartTime = prstart.Now().Add(20 * time.Second)
		tr1Pod       = "output-task-pod-123456"
		tr1Step1Name = "writefile-step"
		tr1InitStep1 = "credential-initializer-mdzbr"
		pr2InitStep2 = "place-tools"

		task2Name    = "read-task"
		tr2Name      = "read-task-1"
		tr2StartTime = prstart.Now().Add(2 * time.Minute)
		tr2Pod       = "read-task-pod-123456"
		tr2Step1Name = "readfile-step"

		nopStep = "nop"
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(tr1Name, ns,
			tb.TaskRunStatus(
				tb.PodName(tr1Pod),
				tb.TaskRunStartTime(tr1StartTime),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName(tr1Step1Name),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName(nopStep),
					tb.StateTerminated(0),
				),
			),
		),
		tb.TaskRun(tr2Name, ns,
			tb.TaskRunStatus(
				tb.PodName(tr2Pod),
				tb.TaskRunStartTime(tr2StartTime),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName(tr2Step1Name),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName(nopStep),
					tb.StateTerminated(0),
				),
			),
		),
	}

	prtrs := map[string]*v1alpha1.PipelineRunTaskRunStatus{
		tr1Name: {
			PipelineTaskName: task1Name,
			Status:           &trs[0].Status,
		},
		tr2Name: {
			PipelineTaskName: task2Name,
			Status:           &trs[1].Status,
		},
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
				tb.PipelineRunTaskRunsStatus(
					prtrs,
				),
			),
		),
	}

	pps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(task1Name, task1Name),
				tb.PipelineTask(task2Name, task2Name),
			),
		),
	}

	pods := []*corev1.Pod{
		tb.Pod(tr1Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodInitContainer(taskrun.ContainerNameForStep(tr1InitStep1), "override-with-creds:latest"),
				tb.PodInitContainer(taskrun.ContainerNameForStep(pr2InitStep2), "override-with-tools:latest"),
				tb.PodContainer(taskrun.ContainerNameForStep(tr1Step1Name), tr1Step1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodInitContainerStatus(taskrun.ContainerNameForStep(tr1InitStep1), "override-with-creds:latest"),
				cb.PodInitContainerStatus(taskrun.ContainerNameForStep(pr2InitStep2), "override-with-tools:latest"),
			),
		),
		tb.Pod(tr2Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodContainer(taskrun.ContainerNameForStep(tr2Step1Name), tr1Step1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
		),
	}

	fakeLogStream := logs.Pipeline(
		logs.Task(task1Name, tr1Pod,
			logs.Step(tr1InitStep1, taskrun.ContainerNameForStep(tr1InitStep1),
				"initialized the credentials",
			),
			logs.Step(pr2InitStep2, taskrun.ContainerNameForStep(pr2InitStep2),
				"place tools log",
			),
			logs.Step(tr1Step1Name, taskrun.ContainerNameForStep(tr1Step1Name),
				"written a file",
			),
			logs.Step(nopStep, nopStep,
				"Build successful",
			),
		),
		logs.Task(task2Name, tr2Pod,
			logs.Step(tr2Step1Name, taskrun.ContainerNameForStep(tr2Step1Name),
				"able to read a file",
			),
			logs.Step(nopStep, nopStep,
				"Build successful",
			),
		),
	)

	scenarios := []struct {
		name         string
		allSteps     bool
		tasks        []string
		expectedLogs []string
	}{
		{
			name:     "all task logs",
			allSteps: false,
			expectedLogs: []string{
				"[output-task : writefile-step] written a file",
				"[output-task : nop] Build successful",
				"[read-task : readfile-step] able to read a file",
				"[read-task : nop] Build successful",
			},
		}, {
			name:     "task1 logs only",
			allSteps: false,
			tasks:    []string{task1Name},
			expectedLogs: []string{
				"[output-task : writefile-step] written a file",
				"[output-task : nop] Build successful",
			},
		}, {
			name:     "all steps logs",
			allSteps: true,
			expectedLogs: []string{
				"[output-task : credential-initializer-mdzbr] initialized the credentials",
				"[output-task : place-tools] place tools log",
				"[output-task : writefile-step] written a file",
				"[output-task : nop] Build successful",
				"[read-task : readfile-step] able to read a file",
				"[read-task : nop] Build successful",
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			cs, _ := test.SeedTestData(test.Data{PipelineRuns: prs, Pipelines: pps, TaskRuns: trs, Pods: pods})

			plr := fakePipelineRunLogs(prName, ns, cs)
			output := fetchLogs(plr, LogOpts(s.allSteps, s.tasks...), logs.FakeLogFetcher(cs.Kube, fakeLogStream))

			expected := strings.Join(s.expectedLogs, "\n") + "\n"

			if d := cmp.Diff(output, expected); d != "" {
				t.Errorf("Unexpected output mismatch: \n%s\n", d)
			}
		})
	}
}

// scenario, print logs for 1 completed taskruns out of 4 pipeline tasks
func Test_print_valid_taskrun_logs(t *testing.T) {
	var (
		pipelineName = "output-pipeline"
		prName       = "output-pipeline-1"
		prstart      = clockwork.NewFakeClock()
		ns           = "namespace"

		task1Name    = "output-task"
		tr1Name      = "output-task-1"
		tr1StartTime = prstart.Now().Add(20 * time.Second)
		tr1Pod       = "output-task-pod-123456"
		tr1Step1Name = "writefile-step"

		// these are pipeline tasks for which pipeline has not
		// scheduled any taskrun
		task2Name = "read-task"

		task3Name = "notify"

		task4Name = "teardown"
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(tr1Name, ns,
			tb.TaskRunStatus(
				tb.PodName(tr1Pod),
				tb.TaskRunStartTime(tr1StartTime),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName(tr1Step1Name),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName("nop"),
					tb.StateTerminated(0),
				),
			),
		),
	}

	prtrs := map[string]*v1alpha1.PipelineRunTaskRunStatus{
		tr1Name: {
			PipelineTaskName: task1Name,
			Status:           &trs[0].Status,
		},
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonRunning,
				}),
				tb.PipelineRunTaskRunsStatus(
					prtrs,
				),
			),
		),
	}

	pps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(task1Name, task1Name),
				tb.PipelineTask(task2Name, task2Name),
				tb.PipelineTask(task3Name, task3Name),
				tb.PipelineTask(task4Name, task4Name),
			),
		),
	}

	pods := []*corev1.Pod{
		tb.Pod(tr1Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodContainer(taskrun.ContainerNameForStep(tr1Step1Name), tr1Step1Name+":latest"),
				tb.PodContainer("nop", "override-with-nop:latest"),
			),
		),
	}

	fakeLogStream := logs.Pipeline(
		logs.Task(task1Name, tr1Pod,
			logs.Step(tr1Step1Name, taskrun.ContainerNameForStep(tr1Step1Name),
				"written a file",
			),
			logs.Step("nop", "nop",
				"Build successful",
			),
		),
	)

	cs, _ := test.SeedTestData(test.Data{PipelineRuns: prs, Pipelines: pps, TaskRuns: trs, Pods: pods})
	plr := fakePipelineRunLogs(prName, ns, cs)

	output := fetchLogs(plr, LogOpts(false), logs.FakeLogFetcher(cs.Kube, fakeLogStream))
	expectedLogs := []string{
		"[output-task : writefile-step] written a file",
		"[output-task : nop] Build successful",
	}

	expected := strings.Join(expectedLogs, "\n") + "\n"

	if d := cmp.Diff(output, expected); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}

}

func LogOpts(allSteps bool, tasks ...string) LogOptions {
	return LogOptions{
		LogOptions: taskrun.LogOptions{
			AllSteps: allSteps,
		},
		Tasks: tasks,
	}
}

func fakePipelineRunLogs(name string, ns string, cs test.Clients) *PipelineRunLogs {
	return NewPipelineRunLogs(name, ns,
		&cli.Clients{
			Tekton: cs.Pipeline,
			Kube:   cs.Kube,
		})
}

func fetchLogs(plr *PipelineRunLogs, opt LogOptions, fetcher *logs.LogFetcher) string {
	out := new(bytes.Buffer)

	plr.Fetch(logs.Streams{Out: out, Err: out}, opt, fetcher)

	return out.String()
}
