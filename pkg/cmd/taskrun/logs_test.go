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

package taskrun

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/helper/pods/fake"
	"github.com/tektoncd/cli/pkg/helper/pods/stream"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8stest "k8s.io/client-go/testing"
)

func TestLog_no_taskrun_arg(t *testing.T) {
	c := Command(&tu.Params{})

	_, err := tu.ExecuteCommand(c, "logs", "-n", "ns")
	if err == nil {
		t.Error("Expecting an error but it's empty")
	}
}

func TestLog_missing_taskrun(t *testing.T) {
	tr := []*v1alpha1.TaskRun{
		tb.TaskRun("output-taskrun-1", "ns"),
	}
	cs, _ := test.SeedTestData(test.Data{TaskRuns: tr})
	watcher := watch.NewFake()
	cs.Kube.PrependWatchReactor("pods", k8stest.DefaultWatchReactor(watcher, nil))
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	c := Command(p)
	got, _ := tu.ExecuteCommand(c, "logs", "output-taskrun-2", "-n", "ns")
	expected := "Error: " + msgTRNotFoundErr + " : taskruns.tekton.dev \"output-taskrun-2\" not found\n"

	tu.AssertOutput(t, expected, got)
}

func TestLog_taskrun_logs(t *testing.T) {
	var (
		ns          = "namespace"
		taskName    = "output-task"
		trName      = "output-task-1"
		trStartTime = clockwork.NewFakeClock().Now().Add(20 * time.Second)
		trPod       = "output-task-pod-123456"
		trStep1Name = "writefile-step"
		nopStep     = "nop"
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(trName, ns,
			tb.TaskRunStatus(
				tb.PodName(trPod),
				tb.TaskRunStartTime(trStartTime),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName(trStep1Name),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName(nopStep),
					tb.StateTerminated(0),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(taskName),
			),
		),
	}
	ps := []*corev1.Pod{
		tb.Pod(trPod, ns,
			tb.PodSpec(
				tb.PodContainer(trStep1Name, trStep1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodPhase(corev1.PodSucceeded),
			),
		),
	}

	logs := fake.Logs(
		fake.Task(trPod,
			fake.Step(trStep1Name, "wrote a file"),
			fake.Step(nopStep, "Build successful"),
		),
	)

	cs, _ := test.SeedTestData(test.Data{TaskRuns: trs, Pods: ps})
	trlo := logOpts(trName, ns, cs, fake.Streamer(logs), false, false)
	output, _ := fetchLogs(trlo)

	expectedLogs := []string{
		"[writefile-step] wrote a file\n",
		"[nop] Build successful\n",
	}
	expected := strings.Join(expectedLogs, "\n") + "\n"

	tu.AssertOutput(t, expected, output)
}

func TestLog_taskrun_all_steps(t *testing.T) {
	var (
		prstart  = clockwork.NewFakeClock()
		ns       = "namespace"
		taskName = "output-task"

		trName      = "output-task-run"
		trStartTime = prstart.Now().Add(20 * time.Second)
		trPod       = "output-task-pod-123456"
		trInitStep1 = "credential-initializer-mdzbr"
		trInitStep2 = "place-tools"
		trStep1Name = "writefile-step"
		nopStep     = "nop"
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(trName, ns,
			tb.TaskRunStatus(
				tb.PodName(trPod),
				tb.TaskRunStartTime(trStartTime),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName(trStep1Name),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName(nopStep),
					tb.StateTerminated(0),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(taskName),
			),
		),
	}

	p := []*corev1.Pod{
		tb.Pod(trPod, ns,
			tb.PodSpec(
				tb.PodInitContainer(trInitStep1, "override-with-creds:latest"),
				tb.PodInitContainer(trInitStep2, "override-with-tools:latest"),
				tb.PodContainer(trStep1Name, trStep1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodPhase(corev1.PodSucceeded),
				cb.PodInitContainerStatus(trInitStep1, "override-with-creds:latest"),
				cb.PodInitContainerStatus(trInitStep2, "override-with-tools:latest"),
			),
		),
	}

	logs := fake.Logs(
		fake.Task(trPod,
			fake.Step(trInitStep1, "initialized the credentials"),
			fake.Step(trInitStep2, "place tools log"),
			fake.Step(trStep1Name, "written a file"),
			fake.Step(nopStep, "Build successful"),
		),
	)

	cs, _ := test.SeedTestData(test.Data{TaskRuns: trs, Pods: p})

	trl := logOpts(trName, ns, cs, fake.Streamer(logs), true, false)
	output, _ := fetchLogs(trl)

	expectedLogs := []string{
		"[credential-initializer-mdzbr] initialized the credentials\n",
		"[place-tools] place tools log\n",
		"[writefile-step] written a file\n",
		"[nop] Build successful\n",
	}
	expected := strings.Join(expectedLogs, "\n") + "\n"

	tu.AssertOutput(t, expected, output)
}

func TestLog_taskrun_follow_mode(t *testing.T) {
	var (
		prstart     = clockwork.NewFakeClock()
		ns          = "namespace"
		taskName    = "output-task"
		trName      = "output-task-run"
		trStartTime = prstart.Now().Add(20 * time.Second)
		trPod       = "output-task-pod-123456"
		trStep1Name = "writefile-step"
		trInitStep1 = "credential-initializer-mdzbr"
		trInitStep2 = "place-tools"
		nopStep     = "nop"
	)

	trs := []*v1alpha1.TaskRun{
		tb.TaskRun(trName, ns,
			tb.TaskRunStatus(
				tb.PodName(trPod),
				tb.TaskRunStartTime(trStartTime),
				tb.Condition(apis.Condition{
					Type:   apis.ConditionSucceeded,
					Status: corev1.ConditionTrue,
				}),
				tb.StepState(
					cb.StepName(trStep1Name),
					tb.StateTerminated(0),
				),
				tb.StepState(
					cb.StepName(nopStep),
					tb.StateTerminated(0),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(taskName),
			),
		),
	}

	p := []*corev1.Pod{
		tb.Pod(trPod, ns,
			tb.PodSpec(
				tb.PodInitContainer(trInitStep1, "override-with-creds:latest"),
				tb.PodInitContainer(trInitStep2, "override-with-tools:latest"),
				tb.PodContainer(trStep1Name, trStep1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodPhase(corev1.PodSucceeded),
				cb.PodInitContainerStatus(trInitStep1, "override-with-creds:latest"),
				cb.PodInitContainerStatus(trInitStep2, "override-with-tools:latest"),
			),
		),
	}

	logs := fake.Logs(
		fake.Task(trPod,
			fake.Step(trInitStep1, "initialized the credentials"),
			fake.Step(trInitStep2, "place tools log"),
			fake.Step(trStep1Name, "wrote a file"),
			fake.Step(nopStep, "Build successful"),
		),
	)

	cs, _ := test.SeedTestData(test.Data{TaskRuns: trs, Pods: p})

	trlo := logOpts(trName, ns, cs, fake.Streamer(logs), false, true)
	output, _ := fetchLogs(trlo)

	expectedLogs := []string{
		"[writefile-step] wrote a file\n",
		"[nop] Build successful\n",
	}
	expected := strings.Join(expectedLogs, "\n") + "\n"

	tu.AssertOutput(t, expected, output)
}

func logOpts(run, ns string, cs test.Clients, streamer stream.NewStreamerFunc, allSteps bool, follow bool) *LogOptions {
	p := tu.Params{
		Kube:   cs.Kube,
		Tekton: cs.Pipeline,
	}
	p.SetNamespace(ns)

	return &LogOptions{
		taskrunName: run,
		allSteps:    allSteps,
		follow:      follow,
		params:      &p,
		streamer:    streamer,
	}
}

func fetchLogs(lo *LogOptions) (string, error) {
	out := new(bytes.Buffer)
	lo.stream = &cli.Stream{Out: out, Err: out}

	err := lo.run()

	return out.String(), err
}
