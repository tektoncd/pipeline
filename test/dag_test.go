// +build e2e

/*
Copyright 2018 Knative Authors LLC
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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
)

func TestDAGPipelineRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	echoTask := tb.Task("time-echo-task", namespace, tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("repo", v1alpha1.PipelineResourceTypeGit),
			tb.InputsParam("filename", tb.ParamDescription("Name of the file to echo the time into")),
			tb.InputsParam("sleep-sec", tb.ParamDescription("Number of seconds to sleep after echoing")),
		),
		tb.TaskOutputs(tb.OutputsResource("repo", v1alpha1.PipelineResourceTypeGit)),
		tb.Step("echo-time-into-file", "busybox", tb.Command("/bin/sh", "-c"),
			tb.Args("pwd && mkdir -p /workspace/repo/folder && date +%s > /workspace/repo/folder/${inputs.params.filename} && sleep ${inputs.params.sleep-sec}"),
		),
	))
	if _, err := c.TaskClient.Create(echoTask); err != nil {
		t.Fatalf("Failed to create time echo Task: %s", err)
	}
	readTask := tb.Task("folder-read", namespace, tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("repo", v1alpha1.PipelineResourceTypeGit),
		),
		tb.Step("read-all", "busybox", tb.Command("/bin/sh", "-c"),
			tb.Args("cd /workspace/repo/folder && tail -n +1 -- *"),
		),
	))
	if _, err := c.TaskClient.Create(readTask); err != nil {
		t.Fatalf("Failed to create folder reader Task: %s", err)
	}
	repoResource := tb.PipelineResource("repo", namespace, tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypeGit,
		tb.PipelineResourceSpecParam("Url", "https://github.com/githubtraining/example-basic"),
	))
	if _, err := c.PipelineResourceClient.Create(repoResource); err != nil {
		t.Fatalf("Failed to create simple repo PipelineResource: %s", err)
	}
	pipeline := tb.Pipeline("dag-pipeline", namespace, tb.PipelineSpec(
		tb.PipelineDeclaredResource("repo", "repo"),
		tb.PipelineTask("pipeline-task-3", "time-echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-2-parallel-1", "pipeline-task-2-parallel-2")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("filename", "pipeline-task-3"),
			tb.PipelineTaskParam("sleep-sec", "5"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-2", "time-echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")), tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("filename", "pipeline-task-2-paralell-2"),
			tb.PipelineTaskParam("sleep-sec", "5"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-1", "time-echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("filename", "pipeline-task-2-paralell-1"),
			tb.PipelineTaskParam("sleep-sec", "5"),
		),
		tb.PipelineTask("pipeline-task-1", "time-echo-task",
			tb.PipelineTaskInputResource("repo", "repo"),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("filename", "pipeline-task-1"),
			tb.PipelineTaskParam("sleep-sec", "5"),
		),
		tb.PipelineTask("pipeline-task-4-validate-results", "folder-read",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-3")),
		),
	))
	if _, err := c.PipelineClient.Create(pipeline); err != nil {
		t.Fatalf("Failed to create dag-pipeline Pipeline: %s", err)
	}
	pipelineRun := tb.PipelineRun("dag-pipeline-run", namespace, tb.PipelineRunSpec("dag-pipeline",
		tb.PipelineRunResourceBinding("repo", tb.PipelineResourceBindingRef("repo")),
	))
	if _, err := c.PipelineRunClient.Create(pipelineRun); err != nil {
		t.Fatalf("Failed to create dag-pipeline-run PipelineRun: %s", err)
	}
	logger.Infof("Waiting for DAG pipeline to complete")
	if err := WaitForPipelineRunState(c, "dag-pipeline-run", pipelineRunTimeout, PipelineRunSucceed("dag-pipeline-run"), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun to finish: %s", err)
	}
	// FIXME(vdemeester) do the rest :)
	/*
		logger.Infof("Getting logs from results validation task")
		// The volume created with the results will have the same name as the TaskRun
		validationTaskRunName := "dag-pipeline-run-pipeline-task-4-validate-results"
		output, err := getBuildOutputFromVolume(logger, c, namespace, validationTaskRunName, "dag-validation-pod")
		if err != nil {
			t.Fatalf("Unable to get build output for taskrun %s: %s", validationTaskRunName, err)
		}
		fmt.Println(output)

		// Check that the overall order is correct
		times, err := getTimes(output)
		if err != nil {
			t.Fatalf("Unable to parse output %q: %v", output, err)
		}
		sort.Sort(times)

		if times[0].name != "pipeline-task-1" {
			t.Errorf("Expected first task to execute first, but %q was first", times[0].name)
		}
		if !strings.HasPrefix(times[1].name, "pipeline-task-2") {
			t.Errorf("Expected parallel tasks to run second & third, but %q was second", times[1].name)
		}
		if !strings.HasPrefix(times[2].name, "pipeline-task-2") {
			t.Errorf("Expected parallel tasks to run second & third, but %q was third", times[2].name)
		}
		if times[3].name != "pipeline-task-3" {
			t.Errorf("Expected third task to execute third, but %q was third", times[3].name)
		}

		// Check that the two tasks that can run in parallel did
		parallelDiff := times[2].t.Sub(times[1].t)
		if parallelDiff > (time.Second * 5) {
			t.Errorf("Expected parallel tasks to execute more or less at the ame time, but they were %v apart", parallelDiff)
		}
	*/
}

type fileTime struct {
	name string
	t    time.Time
}

type fileTimes []fileTime

func (f fileTimes) Len() int {
	return len(f)
}

func (f fileTimes) Less(i, j int) bool {
	return f[i].t.Before(f[j].t)
}

func (f fileTimes) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func getTimes(output string) (fileTimes, error) {
	times := fileTimes{}
	// The output of tail doesn't include the filename when it only outputs one file
	if len(output) <= 1 {
		return times, fmt.Errorf("output %q is too short to parse, this implies not all tasks wrote their files", output)
	}
	// ==> pipeline-task-1 <==
	//1544055212
	//
	//==> pipeline-task-2-parallel-1 <==
	//1544055304
	//
	//==> pipeline-task-2-parallel-2 <==
	//1544055263
	//
	//==> pipeline-task-3 <==
	//1544055368
	r, err := regexp.Compile("==> (.*) <==")
	if err != nil {
		return times, fmt.Errorf("couldn't compile filename regex: %v", err)
	}

	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i += 3 {
		// First line is the name of the file
		m := r.FindStringSubmatch(lines[i])
		if len(m) != 2 {
			return times, fmt.Errorf("didn't find expected filename in output line %d: %q", i, lines[i])
		}

		// Next line is the date
		i, err := strconv.Atoi(lines[i+1])
		if err != nil {
			return times, fmt.Errorf("error converting date %q to int: %v", lines[i+1], err)
		}

		times = append(times, fileTime{
			name: m[1],
			t:    time.Unix(int64(i), 0),
		})
	}
	return times, nil
}
