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
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/knative/build-pipeline/test/builder"
	knativetest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDAGPipelineRun creates a graph of Tasks which echo the time they
// were run at into a file (named after the Pipeline Task) in an arbitrary resource,
// then looks at the contents of the files and ensures they were run in the order
// intended, which is:
//                               |
//                        pipeline-task-1
//                       /               \
//   pipeline-task-2-parallel-1    pipeline-task-2-parallel-2
//                       \                /
//                        pipeline-task-3
//                               |
//                  pipeline-task-4-validate-results
func TestDAGPipelineRun(t *testing.T) {
	logger := logging.GetContextLogger(t.Name())
	c, namespace := setup(t, logger)

	knativetest.CleanupOnInterrupt(func() { tearDown(t, logger, c, namespace) }, logger)
	defer tearDown(t, logger, c, namespace)

	// Create the Task that echoes the current time to an output resource
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

	// Create the Task that reads from an input resource
	readTask := tb.Task("folder-read", namespace, tb.TaskSpec(
		tb.TaskInputs(
			tb.InputsResource("repo", v1alpha1.PipelineResourceTypeGit),
		),
		tb.Step("read-all", "busybox", tb.Command("/bin/sh", "-c"),
			tb.Args("cd /workspace/repo/folder && tail -n +1 -- *"),
		),
		// TODO(#107 or #224) Use a reliable mechanism to get the logs intead of a `sleep 5` hack
		tb.Step("make-sure-logs-available", "busybox", tb.Command("/bin/sh", "-c"),
			tb.Args("sleep 5"),
		),
	))
	if _, err := c.TaskClient.Create(readTask); err != nil {
		t.Fatalf("Failed to create folder reader Task: %s", err)
	}

	// Create the repo PipelineResource (doesn't really matter which repo we use)
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
			tb.PipelineTaskParam("filename", "pipeline-task-2-parallel-2"),
			tb.PipelineTaskParam("sleep-sec", "5"),
		),
		tb.PipelineTask("pipeline-task-2-parallel-1", "time-echo-task",
			tb.PipelineTaskInputResource("repo", "repo", tb.From("pipeline-task-1")),
			tb.PipelineTaskOutputResource("repo", "repo"),
			tb.PipelineTaskParam("filename", "pipeline-task-2-parallel-1"),
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

	logger.Info("Reading logs from last Task")
	r, err := c.PipelineRunClient.Get("dag-pipeline-run", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't retrieve info about finished piplinerun: %v", err)
	}

	// TODO(christiewilson): Nader is randomizing these names :O
	output, err := readLogsFromPod(logger, c, namespace, r.Status.TaskRuns["dag-pipeline-run-pipeline-task-4-validate-results"].PodName, "build-step-read-all")
	if err != nil {
		t.Fatalf("Unable to get log output from volume: %s", err)
	}

	logger.Info("Verifying order of execution")
	verifyOrderViaTimesInFiles(t, output)
}

func verifyOrderViaTimesInFiles(t *testing.T, output string) {
	// TODO(#107 or #224) Use a reliable mechanism to get the logs intead of a `sleep 5` hack
	if strings.HasPrefix("Unable to retrieve container logs for docker://", output) {
		t.Fatalf("Since init container logs are unreliable, this test is sleeping for 5 seconds in the last task to try to keep the logs around. It didn't work this time, so we should revisit this test (see PR #473)")
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
}

func readLogsFromPod(logger *logging.BaseLogger, c *clients, namespace, podName, containerName string) (string, error) {
	req := c.KubeClient.Kube.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})
	readCloser, err := req.Stream()
	if err != nil {
		return "", fmt.Errorf("couldn't open stream to read logs: %v", err)
	}
	defer readCloser.Close()
	var buf bytes.Buffer
	out := bufio.NewWriter(&buf)
	_, err = io.Copy(out, readCloser)
	return buf.String(), nil
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
	r, err := regexp.Compile("==> (.*) <==")
	if err != nil {
		return times, fmt.Errorf("couldn't compile filename regex: %v", err)
	}

	lines := strings.Split(output, "\n")
	for i := 0; i < len(lines); i += 3 {
		// First line is the name of the file, e.g.: `==> pipeline-task-3 <==`
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
