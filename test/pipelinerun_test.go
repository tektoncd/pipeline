//go:build e2e
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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

var (
	secretName = "secret"
	saName     = "service-account"
	task1Name  = "task1"
)

func TestPipelineRunStatusSpec(t *testing.T) {
	t.Parallel()
	type tests struct {
		name                   string
		testSetup              func(ctx context.Context, t *testing.T, c *clients, namespace string, index int) *v1.Pipeline
		expectedTaskRuns       []string
		expectedNumberOfEvents int
		pipelineRunFunc        func(*testing.T, int, string, string) *v1.PipelineRun
	}

	tds := []tests{{
		name: "pipeline status spec updated",
		testSetup: func(ctx context.Context, t *testing.T, c *clients, namespace string, _ int) *v1.Pipeline {
			t.Helper()
			task := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: pipeline-status-spec-updated
  namespace: %s
spec:
  params:
  - name: HELLO
    default: "Hi!"
  steps:
  - image: mirror.gcr.io/ubuntu
    script: |
      #!/usr/bin/env bash
      echo "$(params.HELLO)"
`, namespace))
			if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
			}

			p := getUpdatedStatusSpecPipeline(t, namespace, task.Name)
			if _, err := c.V1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", p.Name, err)
			}

			return p
		},
		expectedTaskRuns: []string{"task1"},
		// 1 from PipelineRun; 0 from taskrun since it should not be executed due to condition failing
		expectedNumberOfEvents: 2,
		pipelineRunFunc:        getUpdatedStatusSpecPipelineRun,
	}}

	for i, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			p := td.testSetup(ctx, t, c, namespace, i)

			pipelineRun := td.pipelineRunFunc(t, i, namespace, p.Name)
			prName := pipelineRun.Name
			_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}
			t.Logf("Making sure the expected TaskRuns %s were created", td.expectedTaskRuns)
			actualTaskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + prName})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", prName, err)
			}
			expectedTaskRunNames := []string{}
			for _, runName := range td.expectedTaskRuns {
				taskRunName := strings.Join([]string{prName, runName}, "-")
				// check the actual task name starting with prName+runName with a random suffix
				for _, actualTaskRunItem := range actualTaskrunList.Items {
					if strings.HasPrefix(actualTaskRunItem.Name, taskRunName) {
						taskRunName = actualTaskRunItem.Name
					}
				}
				expectedTaskRunNames = append(expectedTaskRunNames, taskRunName)
				r, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
				}
				if !r.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
					t.Fatalf("Expected TaskRun %s to have succeeded but Status is %v", taskRunName, r.Status)
				}
			}

			matchKinds := map[string][]string{"PipelineRun": {prName}, "TaskRun": expectedTaskRunNames}

			events, err := collectMatchingEvents(ctx, c.KubeClient, namespace, matchKinds, "Succeeded")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) != td.expectedNumberOfEvents {
				collectedEvents := ""
				for i, event := range events {
					collectedEvents += fmt.Sprintf("%#v", event)
					if i < (len(events) - 1) {
						collectedEvents += ", "
					}
				}
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of receieved events : %#v", td.expectedNumberOfEvents, len(events), collectedEvents)
			}
			t.Log("Checking if parameter replacements have been updated in the spec.")
			cl, _ := c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
			if cl.Status.PipelineSpec.Tasks[0].Params[0].Value.StringVal != "Hello World!" {
				t.Fatalf(`Expected replaced parameter value %s but found %s`, "Hello World!", cl.Status.PipelineSpec.Tasks[0].Params[0].Value.StringVal)
			}
			tl, _ := c.V1TaskRunClient.Get(ctx, "pipeline-task-update-task1", metav1.GetOptions{})
			if !strings.Contains(tl.Status.TaskSpec.Steps[0].Script, "Hello World!") {
				t.Fatalf(`Expected replaced parameter value : %s in Script: %s But not found`, "Hello World!", tl.Status.TaskSpec.Steps[0].Script)
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func TestPipelineRun(t *testing.T) {
	t.Parallel()
	type tests struct {
		name                   string
		testSetup              func(ctx context.Context, t *testing.T, c *clients, namespace string, index int) *v1.Pipeline
		expectedTaskRuns       []string
		expectedNumberOfEvents int
		pipelineRunFunc        func(*testing.T, int, string, string) *v1.PipelineRun
	}

	tds := []tests{{
		name: "fan-in and fan-out",
		testSetup: func(ctx context.Context, t *testing.T, c *clients, namespace string, _ int) *v1.Pipeline {
			t.Helper()
			tasks := getFanInFanOutTasks(t, namespace)
			for _, task := range tasks {
				if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
					t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
				}
			}

			p := getFanInFanOutPipeline(t, namespace, tasks)
			if _, err := c.V1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", p.Name, err)
			}

			return p
		},
		pipelineRunFunc:  getFanInFanOutPipelineRun,
		expectedTaskRuns: []string{"create-file-kritis", "create-fan-out-1", "create-fan-out-2", "check-fan-in"},
		// 1 from PipelineRun and 4 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 5,
	}}

	for i, td := range tds {
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			p := td.testSetup(ctx, t, c, namespace, i)

			pipelineRun := td.pipelineRunFunc(t, i, namespace, p.Name)
			prName := pipelineRun.Name
			_, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}
			t.Logf("Making sure the expected TaskRuns %s were created", td.expectedTaskRuns)
			actualTaskrunList, err := c.V1TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: "tekton.dev/pipelineRun=" + prName})
			if err != nil {
				t.Fatalf("Error listing TaskRuns for PipelineRun %s: %s", prName, err)
			}
			expectedTaskRunNames := []string{}
			for _, runName := range td.expectedTaskRuns {
				taskRunName := strings.Join([]string{prName, runName}, "-")
				// check the actual task name starting with prName+runName with a random suffix
				for _, actualTaskRunItem := range actualTaskrunList.Items {
					if strings.HasPrefix(actualTaskRunItem.Name, taskRunName) {
						taskRunName = actualTaskRunItem.Name
					}
				}
				expectedTaskRunNames = append(expectedTaskRunNames, taskRunName)
				r, err := c.V1TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Couldn't get expected TaskRun %s: %s", taskRunName, err)
				}
				if !r.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
					t.Fatalf("Expected TaskRun %s to have succeeded but Status is %v", taskRunName, r.Status)
				}

				t.Logf("Checking that labels were propagated correctly for TaskRun %s", r.Name)
				checkLabelPropagation(ctx, t, c, namespace, prName, r)
				t.Logf("Checking that annotations were propagated correctly for TaskRun %s", r.Name)
				checkAnnotationPropagation(ctx, t, c, namespace, prName, r)
			}

			matchKinds := map[string][]string{"PipelineRun": {prName}, "TaskRun": expectedTaskRunNames}

			t.Logf("Making sure %d events were created from taskrun and pipelinerun with kinds %v", td.expectedNumberOfEvents, matchKinds)

			events, err := collectMatchingEvents(ctx, c.KubeClient, namespace, matchKinds, "Succeeded")
			if err != nil {
				t.Fatalf("Failed to collect matching events: %q", err)
			}
			if len(events) != td.expectedNumberOfEvents {
				collectedEvents := ""
				for i, event := range events {
					collectedEvents += fmt.Sprintf("%#v", event)
					if i < (len(events) - 1) {
						collectedEvents += ", "
					}
				}
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of received events : %#v", td.expectedNumberOfEvents, len(events), collectedEvents)
			}

			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getUpdatedStatusSpecPipeline(t *testing.T, namespace string, taskName string) *v1.Pipeline {
	t.Helper()
	return parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: pipeline-status-spec-updated
  namespace: %s
spec:
  params:
  - name: HELLO
    type: string
  tasks:
  - name: %s
    params:
    - name: HELLO
      value: "$(params.HELLO)"
    taskRef:
      name: %s
`, namespace, task1Name, taskName))
}

// TestPipelineRunRefDeleted tests that a running PipelineRun doesn't fail when the Pipeline
// it references is deleted.
func TestPipelineRunRefDeleted(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	pipelineName := helpers.ObjectNameForTest(t)
	prName := helpers.ObjectNameForTest(t)
	t.Logf("Creating Pipeline, and PipelineRun %s in namespace %s", prName, namespace)

	pipeline := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  tasks:
  - name: step1
    taskSpec:
      steps:
      - name: echo
        image: mirror.gcr.io/ubuntu
        script: |
          #!/usr/bin/env bash
          # Sleep for 10s
          sleep 10
  - name: step2
    runAfter: [step1]
    taskSpec:
      steps:
      - name: echo
        image: mirror.gcr.io/ubuntu
        script: |
          #!/usr/bin/env bash
          # Sleep for another 10s
          sleep 10
`, pipelineName))
	if _, err := c.V1PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}

	pipelinerun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
spec:
  pipelineRef:
    name: %s
`, prName, pipelineName))
	_, err := c.V1PipelineRunClient.Create(ctx, pipelinerun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, Running(prName), "PipelineRunRunning", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}

	if err := c.V1PipelineClient.Delete(ctx, pipeline.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete Pipeline `%s`: %s", pipeline.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

// TestPipelineRunPending tests that a Pending PipelineRun is not run until the pending
// status is cleared. This is separate from the TestPipelineRun suite because it has to
// transition PipelineRun states during the test, which the TestPipelineRun suite does not
// support.
func TestPipelineRunPending(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	prName := helpers.ObjectNameForTest(t)

	t.Logf("Creating Task, Pipeline, and Pending PipelineRun %s in namespace %s", prName, namespace)

	if _, err := c.V1TaskClient.Create(ctx, parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: mirror.gcr.io/ubuntu
    command: ['/bin/bash']
    args: ['-c', 'echo hello, world']
`, taskName, namespace)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	if _, err := c.V1PipelineClient.Create(ctx, parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task
    taskRef:
      name: %s
`, pipelineName, namespace, taskName)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}

	pipelineRun, err := c.V1PipelineRunClient.Create(ctx, parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
  status: PipelineRunPending
`, prName, namespace, pipelineName)), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be marked pending", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunPending(prName), "PipelineRunPending", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be marked pending: %s", prName, err)
	}

	t.Logf("Clearing pending status on PipelineRun %s", prName)

	pipelineRun, err = c.V1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting PipelineRun %s: %s", prName, err)
	}

	if pipelineRun.Status.StartTime != nil {
		t.Fatalf("Error start time must be nil, not: %s", pipelineRun.Status.StartTime)
	}

	pipelineRun.Spec.Status = ""

	if _, err := c.V1PipelineRunClient.Update(ctx, pipelineRun, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Error clearing pending status on PipelineRun %s: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

func getFanInFanOutTasks(t *testing.T, namespace string) map[string]*v1.Task {
	t.Helper()
	return map[string]*v1.Task{
		"create-file": parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: mirror.gcr.io/busybox
    name: write-data-task-0-step-0
    script: echo stuff | tee $(results.result-stuff.path)
  - image: mirror.gcr.io/busybox
    name: write-data-task-0-step-1
    script: echo other | tee $(results.result-other.path)
  results:
    - name: result-stuff
    - name: result-other
`, helpers.ObjectNameForTest(t), namespace)),
		"check-create-files-exists": parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: check-stuff
  steps:
  - image: mirror.gcr.io/busybox
    name: read-from-task-0
    script: echo $(params.check-stuff)
  - image: mirror.gcr.io/busybox
    name: write-data-task-1
    script: echo | tee $(results.result-something.path)
  results:
  - name: result-something
`, helpers.ObjectNameForTest(t), namespace)),
		"check-create-files-exists-2": parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: check-other
  steps:
  - script: echo $(params.check-other)
    image: mirror.gcr.io/busybox
    name: read-from-task-0
  - image: mirror.gcr.io/busybox
    name: write-data-task-1
    script: echo something | tee $(results.result-else.path)
  results:
    - name: result-else
`, helpers.ObjectNameForTest(t), namespace)),
		"read-files": parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  params:
  - name: workspacepath-something
    value: $(tasks.create-file.results.result-something)
  - name: workspacepath-else
    value: $(tasks.create-file.results.result-else)
  steps:
  - image: mirror.gcr.io/busybox
    script: echo params.workspacepath-something
`, helpers.ObjectNameForTest(t), namespace)),
	}
}

func getFanInFanOutPipeline(t *testing.T, namespace string, tasks map[string]*v1.Task) *v1.Pipeline {
	t.Helper()
	return parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: create-file-kritis
    taskRef:
      name: %s
  - name: create-fan-out-1
    taskRef:
      name: %s
    params:
    - name: check-stuff
      value: $(tasks.create-file-kritis.results.result-stuff)
  - name: create-fan-out-2
    taskRef:
      name: %s
    params:
    - name: check-other
      value: $(tasks.create-file-kritis.results.result-other)
  - name: check-fan-in
    taskRef:
      name: %s
    params:
    - name: workspacepath-something
      value: $(tasks.create-fan-out-1.results.result-something)
    - name: workspacepath-else
      value: $(tasks.create-fan-out-2.results.result-else)
`, helpers.ObjectNameForTest(t), namespace, tasks["create-file"].Name, tasks["check-create-files-exists"].Name,
		tasks["check-create-files-exists-2"].Name, tasks["read-files"].Name))
}

func getFanInFanOutPipelineRun(t *testing.T, _ int, namespace string, pipelineName string) *v1.PipelineRun {
	t.Helper()
	return parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, helpers.ObjectNameForTest(t), namespace, pipelineName))
}

func getUpdatedStatusSpecPipelineRun(t *testing.T, _ int, namespace string, pipelineName string) *v1.PipelineRun {
	t.Helper()
	return parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: "pipeline-task-update"
  namespace: %s
spec:
  params:
  - name: HELLO
    value: "Hello World!"
  pipelineRef:
    name: %s
`, namespace, pipelineName))
}

// collectMatchingEvents collects list of events under 5 seconds that match
// 1. matchKinds which is a map of Kind of Object with name of objects
// 2. reason which is the expected reason of event
func collectMatchingEvents(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	namespace string,
	kinds map[string][]string,
	reason string,
) ([]*corev1.Event, error) {
	var events []*corev1.Event

	watchEvents, err := kubeClient.CoreV1().Events(namespace).Watch(ctx, metav1.ListOptions{})
	// close watchEvents channel
	defer watchEvents.Stop()
	if err != nil {
		return events, err
	}

	// create timer to not wait for events longer than 5 seconds
	timer := time.NewTimer(5 * time.Second)

	for {
		select {
		case wevent := <-watchEvents.ResultChan():
			event := wevent.Object.(*corev1.Event)
			if val, ok := kinds[event.InvolvedObject.Kind]; ok {
				for _, expectedName := range val {
					if event.InvolvedObject.Name == expectedName && event.Reason == reason {
						events = append(events, event)
					}
				}
			}
		case <-timer.C:
			return events, nil
		}
	}
}

// checkLabelPropagation checks that labels are correctly propagating from
// Pipelines, PipelineRuns, and Tasks to TaskRuns and Pods.
func checkLabelPropagation(ctx context.Context, t *testing.T, c *clients, namespace string, pipelineRunName string, tr *v1.TaskRun) {
	t.Helper()
	// Our controllers add 4 labels automatically. If custom labels are set on
	// the Pipeline, PipelineRun, or Task then the map will have to be resized.
	labels := make(map[string]string, 4)

	// Check label propagation to PipelineRuns.
	pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", tr.Name, err)
	}
	p, err := c.V1PipelineClient.Get(ctx, pr.Spec.PipelineRef.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected Pipeline for %s: %s", pr.Name, err)
	}
	for key, val := range p.ObjectMeta.Labels {
		labels[key] = val
	}
	// This label is added to every PipelineRun by the PipelineRun controller
	labels[pipeline.PipelineLabelKey] = p.Name
	assertLabelsMatch(t, labels, pr.ObjectMeta.Labels)

	// Check label propagation to TaskRuns.
	for key, val := range pr.ObjectMeta.Labels {
		labels[key] = val
	}
	// This label is added to every TaskRun by the PipelineRun controller
	labels[pipeline.PipelineRunLabelKey] = pr.Name
	if tr.Spec.TaskRef != nil {
		task, err := c.V1TaskClient.Get(ctx, tr.Spec.TaskRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Couldn't get expected Task for %s: %s", tr.Name, err)
		}
		for key, val := range task.ObjectMeta.Labels {
			labels[key] = val
		}
		// This label is added to TaskRuns that reference a Task by the TaskRun controller
		labels[pipeline.TaskLabelKey] = task.Name
	}
	assertLabelsMatch(t, labels, tr.ObjectMeta.Labels)

	// PodName is "" if a retry happened and pod is deleted
	// This label is added to every Pod by the TaskRun controller
	if tr.Status.PodName != "" {
		// Check label propagation to Pods.
		pod := getPodForTaskRun(ctx, t, c.KubeClient, namespace, tr)
		// This label is added to every Pod by the TaskRun controller
		labels[pipeline.TaskRunLabelKey] = tr.Name
		assertLabelsMatch(t, labels, pod.ObjectMeta.Labels)
	}
}

// checkAnnotationPropagation checks that annotations are correctly propagating from
// Pipelines, PipelineRuns, and Tasks to TaskRuns and Pods.
func checkAnnotationPropagation(ctx context.Context, t *testing.T, c *clients, namespace string, pipelineRunName string, tr *v1.TaskRun) {
	t.Helper()
	annotations := make(map[string]string)

	// Check annotation propagation to PipelineRuns.
	pr, err := c.V1PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", tr.Name, err)
	}
	p, err := c.V1PipelineClient.Get(ctx, pr.Spec.PipelineRef.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected Pipeline for %s: %s", pr.Name, err)
	}
	for key, val := range p.ObjectMeta.Annotations {
		annotations[key] = val
	}
	assertAnnotationsMatch(t, annotations, pr.ObjectMeta.Annotations)

	// Check annotation propagation to TaskRuns.
	for key, val := range pr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	if tr.Spec.TaskRef != nil {
		task, err := c.V1TaskClient.Get(ctx, tr.Spec.TaskRef.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Couldn't get expected Task for %s: %s", tr.Name, err)
		}
		for key, val := range task.ObjectMeta.Annotations {
			annotations[key] = val
		}
	}
	assertAnnotationsMatch(t, annotations, tr.ObjectMeta.Annotations)

	// Check annotation propagation to Pods.
	pod := getPodForTaskRun(ctx, t, c.KubeClient, namespace, tr)
	assertAnnotationsMatch(t, annotations, pod.ObjectMeta.Annotations)
}

func getPodForTaskRun(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, namespace string, tr *v1.TaskRun) *corev1.Pod {
	t.Helper()
	// The Pod name has a random suffix, so we filter by label to find the one we care about.
	pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: pipeline.TaskRunLabelKey + " = " + tr.Name,
	})
	if err != nil {
		t.Fatalf("Couldn't get expected Pod for %s: %s", tr.Name, err)
	}
	if numPods := len(pods.Items); numPods != 1 {
		t.Fatalf("Expected 1 Pod for %s, but got %d Pods", tr.Name, numPods)
	}
	return &pods.Items[0]
}

func assertLabelsMatch(t *testing.T, expectedLabels, actualLabels map[string]string) {
	t.Helper()
	for key, expectedVal := range expectedLabels {
		if actualVal := actualLabels[key]; actualVal != expectedVal {
			t.Errorf("Expected labels containing %s=%s but labels were %v", key, expectedVal, actualLabels)
		}
	}
}

func assertAnnotationsMatch(t *testing.T, expectedAnnotations, actualAnnotations map[string]string) {
	t.Helper()
	for key, expectedVal := range expectedAnnotations {
		if actualVal := actualAnnotations[key]; actualVal != expectedVal {
			t.Errorf("Expected annotations containing %s=%s but annotations were %v", key, expectedVal, actualAnnotations)
		}
	}
}

func TestPipelineRunTaskFailed(t *testing.T) {
	ctx := t.Context()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	pipelineName := helpers.ObjectNameForTest(t)
	prName := helpers.ObjectNameForTest(t)

	t.Logf("Creating Task, Pipeline, and Pending PipelineRun %s in namespace %s", prName, namespace)

	if _, err := c.V1beta1TaskClient.Create(ctx, parse.MustParseV1beta1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - image: mirror.gcr.io/ubuntu
    command: ['/bin/bash']
    args: ['-c', 'echo hello, world']
`, taskName, namespace)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", taskName, err)
	}

	if _, err := c.V1beta1PipelineClient.Create(ctx, parse.MustParseV1beta1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task
    taskRef:
      name: %s
`, pipelineName, namespace, taskName)), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", pipelineName, err)
	}

	pipelineRun, err := c.V1beta1PipelineRunClient.Create(ctx, parse.MustParseV1beta1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    results:
      - name: abc
        value: "$(tasks.xxx.results.abc)"
    tasks:
    - name: xxx
      taskSpec:
        results:
          - name: abc
        steps:
        - name: update-sa
          image: mirror.gcr.io/bash
          script: |
            echo 'test' >  $(results.abc.path)
            exit 1
`, prName, namespace)), metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}
	// Wait for the PipelineRun to fail.
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunFailed(prName), "PipelineRunFailed", v1beta1Version); err != nil {
		t.Fatalf("Waiting for PipelineRun to fail: %v", err)
	}

	pipelineRun, err = c.V1beta1PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting PipelineRun %s: %s", prName, err)
	}

	if pipelineRun.Status.GetCondition(apis.ConditionSucceeded).IsTrue() {
		t.Errorf("Expected PipelineRun to fail but found condition: %s", pipelineRun.Status.GetCondition(apis.ConditionSucceeded))
	}
	expectedMessage := "Tasks Completed: 1 (Failed: 1, Cancelled 0), Skipped: 0"
	if pipelineRun.Status.GetCondition(apis.ConditionSucceeded).Message != expectedMessage {
		t.Errorf("Expected PipelineRun to fail with condition message: %s but got: %s", expectedMessage, pipelineRun.Status.GetCondition(apis.ConditionSucceeded).Message)
	}
}
