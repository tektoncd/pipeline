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

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/knative/pkg/apis"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/logs"
	tu "github.com/tektoncd/cli/pkg/test"
	cb "github.com/tektoncd/cli/pkg/test/builder"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
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
	p := &tu.Params{Tekton: cs.Pipeline, Kube: cs.Kube}

	c := Command(p)
	got, _ := tu.ExecuteCommand(c, "logs", "output-taskrun-2", "-n", "ns")
	expected := msgTRNotFoundErr + " : taskruns.tekton.dev \"output-taskrun-2\" not found \n"

	if d := cmp.Diff(expected, got); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func TestLog_valid_taskrun_logs(t *testing.T) {
	var (
		ns          = "namespace"
		taskName    = "output-task"
		trName      = "output-task-1"
		trStartTime = clockwork.NewFakeClock().Now().Add(20 * time.Second)
		trPod       = "output-task-pod-123456"
		trStep1Name = "writefile-step"
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
					cb.StepName("nop"),
					tb.StateTerminated(0),
				),
			),
			tb.TaskRunSpec(
				tb.TaskRunTaskRef(taskName),
			),
		),
	}
	pods := []*corev1.Pod{
		tb.Pod(trPod, ns,
			tb.PodSpec(
				tb.PodContainer(ContainerNameForStep(trStep1Name), trStep1Name+":latest"),
				tb.PodContainer("nop", "override-with-nop:latest"),
			),
		),
	}
	fakeLogStream := logs.TaskRun(
		logs.Task(taskName, trPod,
			logs.Step(trStep1Name, ContainerNameForStep(trStep1Name),
				"written a file",
			),
			logs.Step("nop", "nop",
				"Build successful",
			),
		),
	)

	cs, _ := test.SeedTestData(test.Data{TaskRuns: trs, Pods: pods})
	tr := fakeLogs(trName, ns, cs)

	output := fetchLogs(LogOptions{}, tr, logs.FakeLogFetcher(cs.Kube, fakeLogStream))
	expectedLogs := []string{
		"[output-task : writefile-step] written a file",
		"[output-task : nop] Build successful",
	}

	expected := strings.Join(expectedLogs, "\n") + "\n"

	if d := cmp.Diff(output, expected); d != "" {
		t.Errorf("Unexpected output mismatch: \n%s\n", d)
	}
}

func TestLog_taskrun_logs(t *testing.T) {
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

	pods := []*corev1.Pod{
		tb.Pod(trPod, ns,
			tb.PodSpec(
				tb.PodInitContainer(ContainerNameForStep(trInitStep1), "override-with-creds:latest"),
				tb.PodInitContainer(ContainerNameForStep(trInitStep2), "override-with-tools:latest"),
				tb.PodContainer(ContainerNameForStep(trStep1Name), trStep1Name+":latest"),
				tb.PodContainer(nopStep, "override-with-nop:latest"),
			),
			cb.PodStatus(
				cb.PodInitContainerStatus(ContainerNameForStep(trInitStep1), "override-with-creds:latest"),
				cb.PodInitContainerStatus(ContainerNameForStep(trInitStep2), "override-with-tools:latest"),
			),
		),
	}

	fakeLogStream := logs.TaskRun(
		logs.Task(taskName, trPod,
			logs.Step(trInitStep1, ContainerNameForStep(trInitStep1),
				"initialized the credentials",
			),
			logs.Step(trInitStep2, ContainerNameForStep(trInitStep2),
				"place tools log",
			),
			logs.Step(trStep1Name, ContainerNameForStep(trStep1Name),
				"written a file",
			),
			logs.Step(nopStep, nopStep,
				"Build successful",
			),
		),
	)

	scenarios := []struct {
		name         string
		allSteps     bool
		expectedLogs []string
	}{
		{
			name:     "task logs only",
			allSteps: false,
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
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			cs, _ := test.SeedTestData(test.Data{TaskRuns: trs, Pods: pods})

			trl := fakeLogs(trName, ns, cs)
			opts := LogOptions{AllSteps: s.allSteps}
			output := fetchLogs(opts, trl, logs.FakeLogFetcher(cs.Kube, fakeLogStream))
			expected := strings.Join(s.expectedLogs, "\n") + "\n"

			if d := cmp.Diff(output, expected); d != "" {
				t.Errorf("Unexpected output mismatch: \n%s\n", d)
			}
		})
	}
}

func fakeLogs(run, ns string, cs test.Clients) *Logs {
	return &Logs{
		Run: run,
		Ns:  ns,
		Clients: &cli.Clients{
			Tekton: cs.Pipeline,
			Kube:   cs.Kube,
		},
	}
}

func fetchLogs(opt LogOptions, trl *Logs, fetcher *logs.LogFetcher) string {
	out := new(bytes.Buffer)
	trl.Fetch(opt, logs.Streams{Out: out, Err: out}, fetcher)
	return out.String()
}
