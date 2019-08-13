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

	"github.com/jonboulle/clockwork"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/helper/pods/fake"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
	"github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/reconciler/v1alpha1/pipelinerun/resources"
	pipelinetest "github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8stest "k8s.io/client-go/testing"
	"knative.dev/pkg/apis"
)

func TestLog_no_pipelinerun_argument(t *testing.T) {
	c := Command(&test.Params{})

	_, err := test.ExecuteCommand(c, "logs", "-n", "ns")

	if err == nil {
		t.Error("Expecting an error but it's empty")
	}
}

func TestLog_run_not_found(t *testing.T) {
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
	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: pr})
	p := &test.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	c := Command(p)
	_, err := test.ExecuteCommand(c, "logs", "output-pipeline-2", "-n", "ns")
	expected := "pipelineruns.tekton.dev \"output-pipeline-2\" not found"
	test.AssertOutput(t, expected, err.Error())
}

func TestPipelinerunLogs(t *testing.T) {
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
		tr1InitStep2 = "place-tools"

		task2Name    = "read-task"
		tr2Name      = "read-task-1"
		tr2StartTime = prstart.Now().Add(2 * time.Minute)
		tr2Pod       = "read-task-pod-123456"
		tr2Step1Name = "readfile-step"

		nopStep = "nop"
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(tr1Name, ns,
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task1Name),
			),
			tb.TaskRunStatus(
				tb.PodName(tr1Pod),
				tb.TaskRunStartTime(tr1StartTime),
				tb.StatusCondition(apis.Condition{
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
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task1Name),
			),
		),
		tb.TaskRun(tr2Name, ns,
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task2Name),
			),
			tb.TaskRunStatus(
				tb.PodName(tr2Pod),
				tb.TaskRunStartTime(tr2StartTime),
				tb.StatusCondition(apis.Condition{
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
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task2Name),
			),
		),
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonSucceeded,
				}),
				tb.PipelineRunTaskRunsStatus(tr1Name, &v1alpha1.PipelineRunTaskRunStatus{
					PipelineTaskName: task1Name,
					Status:           &trs[0].Status,
				}),
				tb.PipelineRunTaskRunsStatus(tr2Name, &v1alpha1.PipelineRunTaskRunStatus{
					PipelineTaskName: task2Name,
					Status:           &trs[1].Status,
				}),
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

	p := []*corev1.Pod{
		tb.Pod(tr1Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodInitContainer(tr1InitStep1, "override-with-creds:latest"),
				tb.PodInitContainer(tr1InitStep2, "override-with-tools:latest"),
				tb.PodContainer(tr1Step1Name, tr1Step1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodInitContainerStatus(tr1InitStep1, "override-with-creds:latest"),
				cb.PodInitContainerStatus(tr1InitStep2, "override-with-tools:latest"),
			),
		),
		tb.Pod(tr2Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodContainer(tr2Step1Name, tr1Step1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
		),
	}

	fakeLogs := fake.Logs(
		fake.Task(tr1Pod,
			fake.Step(tr1InitStep1, "initialized the credentials"),
			fake.Step(tr1InitStep2, "place tools log"),
			fake.Step(tr1Step1Name, "written a file"),
			fake.Step(nopStep, "Build successful"),
		),
		fake.Task(tr2Pod,
			fake.Step(tr2Step1Name, "able to read a file"),
			fake.Step(nopStep, "Build successful"),
		),
	)

	scenarios := []struct {
		name         string
		allSteps     bool
		tasks        []string
		expectedLogs []string
	}{
		{
			name:     "for all tasks",
			allSteps: false,
			expectedLogs: []string{
				"[output-task : writefile-step] written a file\n",
				"[output-task : nop] Build successful\n",
				"[read-task : readfile-step] able to read a file\n",
				"[read-task : nop] Build successful\n",
			},
		}, {
			name:     "for task1 only",
			allSteps: false,
			tasks:    []string{task1Name},
			expectedLogs: []string{
				"[output-task : writefile-step] written a file\n",
				"[output-task : nop] Build successful\n",
			},
		}, {
			name:     "including init steps",
			allSteps: true,
			expectedLogs: []string{
				"[output-task : credential-initializer-mdzbr] initialized the credentials\n",
				"[output-task : place-tools] place tools log\n",
				"[output-task : writefile-step] written a file\n",
				"[output-task : nop] Build successful\n",
				"[read-task : readfile-step] able to read a file\n",
				"[read-task : nop] Build successful\n",
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: pps, TaskRuns: trs, Pods: p})

			prlo := logOpts(prName, ns, cs, fake.Streamer(fakeLogs), s.allSteps, false, s.tasks...)
			output, _ := fetchLogs(prlo)

			expected := strings.Join(s.expectedLogs, "\n") + "\n"

			test.AssertOutput(t, expected, output)
		})
	}
}

// scenario, print logs for 1 completed taskruns out of 4 pipeline tasks
func TestPipelinerunLog_completed_taskrun_only(t *testing.T) {
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
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task1Name),
			),
			tb.TaskRunStatus(
				tb.PodName(tr1Pod),
				tb.TaskRunStartTime(tr1StartTime),
				tb.StatusCondition(apis.Condition{
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
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task1Name),
			),
		),
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonRunning,
				}),
				tb.PipelineRunTaskRunsStatus(tr1Name, &v1alpha1.PipelineRunTaskRunStatus{
					PipelineTaskName: task1Name,
					Status:           &trs[0].Status,
				}),
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

	p := []*corev1.Pod{
		tb.Pod(tr1Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodContainer(tr1Step1Name, tr1Step1Name+":latest"),
				tb.PodContainer("nop", "override-with-nop:latest"),
			),
		),
	}

	fakeLogStream := fake.Logs(
		fake.Task(tr1Pod,
			fake.Step(tr1Step1Name, "wrote a file"),
			fake.Step("nop", "Build successful"),
		),
	)

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: pps, TaskRuns: trs, Pods: p})
	prlo := logOpts(prName, ns, cs, fake.Streamer(fakeLogStream), false, false)
	output, _ := fetchLogs(prlo)

	expectedLogs := []string{
		"[output-task : writefile-step] wrote a file\n",
		"[output-task : nop] Build successful\n",
	}
	expected := strings.Join(expectedLogs, "\n") + "\n"

	test.AssertOutput(t, expected, output)
}

func TestPipelinerunLog_follow_mode(t *testing.T) {
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
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(tr1Name, ns,
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task1Name),
			),
			tb.TaskRunStatus(
				tb.PodName(tr1Pod),
				tb.TaskRunStartTime(tr1StartTime),
				tb.StatusCondition(apis.Condition{
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
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(task1Name),
			),
		),
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status: corev1.ConditionTrue,
					Reason: resources.ReasonRunning,
				}),
				tb.PipelineRunTaskRunsStatus(tr1Name, &v1alpha1.PipelineRunTaskRunStatus{
					PipelineTaskName: task1Name,
					Status:           &trs[0].Status,
				}),
			),
		),
	}

	pps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(task1Name, task1Name),
			),
		),
	}

	p := []*corev1.Pod{
		tb.Pod(tr1Pod, ns,
			tb.PodLabel("tekton.dev/task", pipelineName),
			tb.PodSpec(
				tb.PodContainer(tr1Step1Name, tr1Step1Name+":latest"),
				tb.PodContainer("nop", "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodPhase(corev1.PodSucceeded),
			),
		),
	}

	fakeLogStream := fake.Logs(
		fake.Task(tr1Pod,
			fake.Step(tr1Step1Name,
				"wrote a file1",
				"wrote a file2",
				"wrote a file3",
				"wrote a file4",
			),
			fake.Step("nop", "Build successful"),
		),
	)

	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: pps, TaskRuns: trs, Pods: p})
	prlo := logOpts(prName, ns, cs, fake.Streamer(fakeLogStream), false, true)
	output, _ := fetchLogs(prlo)

	expectedLogs := []string{
		"[output-task : writefile-step] wrote a file1",
		"[output-task : writefile-step] wrote a file2",
		"[output-task : writefile-step] wrote a file3",
		"[output-task : writefile-step] wrote a file4\n",
		"[output-task : nop] Build successful\n",
	}
	expected := strings.Join(expectedLogs, "\n") + "\n"

	test.AssertOutput(t, expected, output)
}

func TestLogs_error_log(t *testing.T) {
	var (
		pipelineName = "errlogs-pipeline"
		prName       = "errlogs-run"
		ns           = "namespace"
		taskName     = "errlogs-task"
		errMsg       = "Pipeline tektoncd/errlog-pipeline can't be Run; it contains Tasks that don't exist: Couldn't retrieve Task errlog-tasks: task.tekton.dev errlog-tasks not found"
	)

	ts := []*v1alpha1.Task{
		tb.Task(taskName, ns,
			tb.TaskSpec()),
	}

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunSpec(pipelineName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status:  corev1.ConditionFalse,
					Message: errMsg,
				}),
			),
		),
	}

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(taskName, taskName),
			),
		),
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: ps, Tasks: ts})
	prlo := logOpts(prName, ns, cs, fake.Streamer([]fake.Log{}), false, false)
	output, err := fetchLogs(prlo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	test.AssertOutput(t, errMsg+"\n", output)
}

func TestLogs_nologs(t *testing.T) {
	var (
		pipelineName = "nologs-pipeline"
		prName       = "nologs-run"
		ns           = "namespace"
		taskName     = "nologs-task"
	)

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunSpec(pipelineName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Status:  corev1.ConditionUnknown,
					Message: "Running",
				}),
			),
		),
	}

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(taskName, taskName),
			),
		),
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: ps})
	prlo := logOpts(prName, ns, cs, fake.Streamer([]fake.Log{}), false, false)
	output, err := fetchLogs(prlo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	test.AssertOutput(t, "No logs found\n", output)
}

func TestLog_run_failed_with_and_without_follow(t *testing.T) {
	var (
		pipelineName = "fail-pipeline"
		prName       = "fail-run"
		ns           = "namespace"
		taskName     = "fail-task"
		failMessage  = "Failed because I wanted"
	)

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunSpec(pipelineName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionFalse,
					Message: failMessage,
				}),
			),
		),
	}

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(taskName, taskName),
			),
		),
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: ps})

	// follow mode disabled
	prlo := logOpts(prName, ns, cs, fake.Streamer([]fake.Log{}), false, false)
	output, err := fetchLogs(prlo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	test.AssertOutput(t, failMessage+"\n", output)

	// follow mode enabled
	prlo = logOpts(prName, ns, cs, fake.Streamer([]fake.Log{}), false, true)
	output, err = fetchLogs(prlo)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	test.AssertOutput(t, failMessage+"\n", output)
}

func TestLog_pipeline_still_running(t *testing.T) {
	var (
		pipelineName = "inprogress-pipeline"
		prName       = "inprogress-run"
		ns           = "namespace"
		taskName     = "inprogress-task"
	)

	initialPRs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunSpec(pipelineName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Message: "Running",
				}),
			),
		),
	}

	finalPRs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Message: "Running",
				}),
			),
		),

		tb.PipelineRun(prName, ns,
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionTrue,
					Message: "Running",
				}),
			),
		),
	}

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(taskName, taskName),
			),
		),
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: initialPRs, Pipelines: ps})
	watcher := watch.NewFake()
	cs.Pipeline.PrependWatchReactor("pipelineruns", k8stest.DefaultWatchReactor(watcher, nil))
	prlo := logOpts(prName, ns, cs, fake.Streamer([]fake.Log{}), false, false)

	updatePR(finalPRs, watcher)

	output, err := fetchLogs(prlo)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	test.AssertOutput(t, "Pipeline still running ..."+"\n", output)
}

func TestLog_pipeline_status_done(t *testing.T) {
	var (
		pipelineName = "done-pipeline"
		prName       = "done-run"
		ns           = "namespace"
		taskName     = "done-task"
	)

	prs := []*v1alpha1.PipelineRun{
		tb.PipelineRun(prName, ns,
			tb.PipelineRunLabel("tekton.dev/pipeline", prName),
			tb.PipelineRunSpec(pipelineName),
			tb.PipelineRunStatus(
				tb.PipelineRunStatusCondition(apis.Condition{
					Type:    apis.ConditionSucceeded,
					Status:  corev1.ConditionUnknown,
					Message: "Running",
				}),
			),
		),
	}

	ps := []*v1alpha1.Pipeline{
		tb.Pipeline(pipelineName, ns,
			tb.PipelineSpec(
				tb.PipelineTask(taskName, taskName),
			),
		),
	}
	cs, _ := test.SeedTestData(t, pipelinetest.Data{PipelineRuns: prs, Pipelines: ps})
	watcher := watch.NewFake()
	cs.Pipeline.PrependWatchReactor("pipelineruns", k8stest.DefaultWatchReactor(watcher, nil))
	prlo := logOpts(prName, ns, cs, fake.Streamer([]fake.Log{}), false, false)

	go func() {
		time.Sleep(time.Second * 1)
		for _, pr := range prs {
			pr.Status.Conditions[0].Status = corev1.ConditionTrue
			pr.Status.Conditions[0].Message = "completed"
			watcher.Modify(pr)
		}
	}()

	start := time.Now()
	output, err := fetchLogs(prlo)
	elapsed := time.Since(start).Seconds()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if elapsed > 10 {
		t.Errorf("Timed out")
	}
	test.AssertOutput(t, "", output)
}

func updatePR(finalRuns []*v1alpha1.PipelineRun, watcher *watch.FakeWatcher) {
	go func() {
		for _, pr := range finalRuns {
			time.Sleep(time.Second * 1)
			watcher.Modify(pr)
		}
	}()
}

func logOpts(name string, ns string, cs pipelinetest.Clients, streamer stream.NewStreamerFunc, allSteps bool, follow bool, onlyTasks ...string) *LogOptions {
	p := test.Params{
		Kube:   cs.Kube,
		Tekton: cs.Pipeline,
	}
	p.SetNamespace(ns)

	logOptions := LogOptions{
		PipelineRunName: name,
		Tasks:           onlyTasks,
		AllSteps:        allSteps,
		Follow:          follow,
		Params:          &p,
		Streamer:        streamer,
	}

	return &logOptions
}

func fetchLogs(lo *LogOptions) (string, error) {
	out := new(bytes.Buffer)
	lo.Stream = &cli.Stream{Out: out, Err: out}

	err := lo.Run()

	return out.String(), err
}
