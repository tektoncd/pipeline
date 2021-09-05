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
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	tbv1alpha1 "github.com/tektoncd/pipeline/internal/builder/v1alpha1"
	tb "github.com/tektoncd/pipeline/internal/builder/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	k8sres "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis"
	knativetest "knative.dev/pkg/test"
)

var (
	pipelineName    = "pipeline"
	pipelineRunName = "pipelinerun"
	secretName      = "secret"
	saName          = "service-account"
	taskName        = "task"
	task1Name       = "task1"
	cond1Name       = "cond-1"
)

func TestPipelineRun(t *testing.T) {
	t.Parallel()
	type tests struct {
		name                   string
		testSetup              func(ctx context.Context, t *testing.T, c *clients, namespace string, index int)
		expectedTaskRuns       []string
		expectedNumberOfEvents int
		pipelineRunFunc        func(int, string) *v1beta1.PipelineRun
	}

	tds := []tests{{
		name: "fan-in and fan-out",
		testSetup: func(ctx context.Context, t *testing.T, c *clients, namespace string, index int) {
			t.Helper()
			for _, task := range getFanInFanOutTasks(namespace) {
				if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
					t.Fatalf("Failed to create Task `%s`: %s", task.Name, err)
				}
			}

			for _, res := range getFanInFanOutGitResources() {
				if _, err := c.PipelineResourceClient.Create(ctx, res, metav1.CreateOptions{}); err != nil {
					t.Fatalf("Failed to create Pipeline Resource `%s`: %s", kanikoGitResourceName, err)
				}
			}

			if _, err := c.PipelineClient.Create(ctx, getFanInFanOutPipeline(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(pipelineName, index), err)
			}
		},
		pipelineRunFunc:  getFanInFanOutPipelineRun,
		expectedTaskRuns: []string{"create-file-kritis", "create-fan-out-1", "create-fan-out-2", "check-fan-in"},
		// 1 from PipelineRun and 4 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 5,
	}, {
		name: "service account propagation and pipeline param",
		testSetup: func(ctx context.Context, t *testing.T, c *clients, namespace string, index int) {
			t.Helper()
			t.Skip("build-crd-testing project got removed, the secret-sauce doesn't exist anymore, skipping")
			if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, getPipelineRunSecret(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create secret `%s`: %s", getName(secretName, index), err)
			}

			if _, err := c.KubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, getPipelineRunServiceAccount(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create SA `%s`: %s", getName(saName, index), err)
			}

			task := &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: getName(taskName, index), Namespace: namespace},
				Spec: v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "the.path", Type: v1beta1.ParamTypeString,
					}, {
						Name: "dest", Type: v1beta1.ParamTypeString,
					}},
					Steps: []v1beta1.Step{{
						Container: corev1.Container{
							Name:    "config-docker",
							Image:   "gcr.io/tekton-releases/dogfooding/skopeo:latest",
							Command: []string{"skopeo"},
							Args:    []string{"copy", `$(params["the.path"])`, "$(params.dest)"},
						}},
					},
				},
			}
			if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", getName(taskName, index), err)
			}

			if _, err := c.PipelineClient.Create(ctx, getHelloWorldPipelineWithSingularTask(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(pipelineName, index), err)
			}
		},
		expectedTaskRuns: []string{task1Name},
		// 1 from PipelineRun and 1 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 2,
		pipelineRunFunc:        getHelloWorldPipelineRun,
	}, {
		name: "pipeline succeeds when task skipped due to failed condition",
		testSetup: func(ctx context.Context, t *testing.T, c *clients, namespace string, index int) {
			t.Helper()
			cond := getFailingCondition()
			if _, err := c.ConditionClient.Create(ctx, cond, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Condition `%s`: %s", cond1Name, err)
			}

			task := &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: getName(taskName, index), Namespace: namespace},
				Spec: v1beta1.TaskSpec{
					Steps: []v1beta1.Step{{Container: corev1.Container{
						Image:   "ubuntu",
						Command: []string{"/bin/bash"},
						Args:    []string{"-c", "echo hello, world"},
					}}},
				},
			}
			if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", getName(taskName, index), err)
			}
			if _, err := c.PipelineClient.Create(ctx, getPipelineWithFailingCondition(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(pipelineName, index), err)
			}
		},
		expectedTaskRuns: []string{},
		// 1 from PipelineRun; 0 from taskrun since it should not be executed due to condition failing
		expectedNumberOfEvents: 1,
		pipelineRunFunc:        getConditionalPipelineRun,
	}, {
		name: "pipelinerun succeeds with LimitRange minimum in namespace",
		testSetup: func(ctx context.Context, t *testing.T, c *clients, namespace string, index int) {
			t.Helper()
			t.Skip("build-crd-testing project got removed, the secret-sauce doesn't exist anymore, skipping")
			if _, err := c.KubeClient.CoreV1().LimitRanges(namespace).Create(ctx, getLimitRange("prlimitrange", namespace, "100m", "99Mi", "100m"), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create LimitRange `%s`: %s", "prlimitrange", err)
			}

			if _, err := c.KubeClient.CoreV1().Secrets(namespace).Create(ctx, getPipelineRunSecret(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create secret `%s`: %s", getName(secretName, index), err)
			}

			if _, err := c.KubeClient.CoreV1().ServiceAccounts(namespace).Create(ctx, getPipelineRunServiceAccount(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create SA `%s`: %s", getName(saName, index), err)
			}

			task := &v1beta1.Task{
				ObjectMeta: metav1.ObjectMeta{Name: getName(taskName, index), Namespace: namespace},
				Spec: v1beta1.TaskSpec{
					Params: []v1beta1.ParamSpec{{
						Name: "the.path", Type: v1beta1.ParamTypeString,
					}, {
						Name: "dest", Type: v1beta1.ParamTypeString,
					}},
					Steps: []v1beta1.Step{{
						Container: corev1.Container{
							Name:    "config-docker",
							Image:   "gcr.io/tekton-releases/dogfooding/skopeo:latest",
							Command: []string{"skopeo"},
							Args:    []string{"copy", `$(params["the.path"])`, `$(params["dest"])`},
						}},
					},
				},
			}
			if _, err := c.TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Task `%s`: %s", fmt.Sprint("task", index), err)
			}
			if _, err := c.PipelineClient.Create(ctx, getHelloWorldPipelineWithSingularTask(index, namespace), metav1.CreateOptions{}); err != nil {
				t.Fatalf("Failed to create Pipeline `%s`: %s", getName(pipelineName, index), err)
			}
		},
		expectedTaskRuns: []string{task1Name},
		// 1 from PipelineRun and 1 from Tasks defined in pipelinerun
		expectedNumberOfEvents: 2,
		pipelineRunFunc:        getHelloWorldPipelineRun,
	}}

	for i, td := range tds {
		i := i   // capture range variable
		td := td // capture range variable
		t.Run(td.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			c, namespace := setup(ctx, t)

			knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
			defer tearDown(ctx, t, c, namespace)

			t.Logf("Setting up test resources for %q test in namespace %s", td.name, namespace)
			td.testSetup(ctx, t, c, namespace, i)

			prName := fmt.Sprintf("%s%d", pipelineRunName, i)
			pipelineRun, err := c.PipelineRunClient.Create(ctx, td.pipelineRunFunc(i, namespace), metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
			}

			t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
			if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess"); err != nil {
				t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
			}

			t.Logf("Making sure the expected TaskRuns %s were created", td.expectedTaskRuns)
			actualTaskrunList, err := c.TaskRunClient.List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("tekton.dev/pipelineRun=%s", prName)})
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
				r, err := c.TaskRunClient.Get(ctx, taskRunName, metav1.GetOptions{})
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
				t.Fatalf("Expected %d number of successful events from pipelinerun and taskrun but got %d; list of receieved events : %#v", td.expectedNumberOfEvents, len(events), collectedEvents)
			}

			// Wait for up to 10 minutes and restart every second to check if
			// the PersistentVolumeClaims has the DeletionTimestamp
			if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				// Check to make sure the PipelineRun's artifact storage PVC has been "deleted" at the end of the run.
				pvc, errWait := c.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, artifacts.GetPVCName(pipelineRun), metav1.GetOptions{})
				if errWait != nil && !errors.IsNotFound(errWait) {
					return true, fmt.Errorf("error looking up PVC %s for PipelineRun %s: %s", artifacts.GetPVCName(pipelineRun), prName, errWait)
				}
				// If we are not found then we are okay since it got cleaned up
				if errors.IsNotFound(errWait) {
					return true, nil
				}
				return pvc.DeletionTimestamp != nil, nil
			}); err != nil {
				t.Fatalf("Error while waiting for the PVC to be set as deleted: %s: %s: %s", artifacts.GetPVCName(pipelineRun), err, prName)
			}
			t.Logf("Successfully finished test %q", td.name)
		})
	}
}

func getHelloWorldPipelineWithSingularTask(suffix int, namespace string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: getName(pipelineName, suffix), Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Params: []v1beta1.ParamSpec{{
				Name: "the.path", Type: v1beta1.ParamTypeString,
			}, {
				Name: "dest", Type: v1beta1.ParamTypeString,
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    task1Name,
				TaskRef: &v1beta1.TaskRef{Name: getName(taskName, suffix)},
				Params: []v1beta1.Param{{
					Name: "the.path", Value: *v1beta1.NewArrayOrString(`$(params["the.path"])`),
				}, {
					Name: "dest", Value: *v1beta1.NewArrayOrString("$(params.dest)"),
				}},
			}},
		},
	}
}

// TestPipelineRunRefDeleted tests that a running PipelineRun doesn't fail when the Pipeline
// it references is deleted.
func TestPipelineRunRefDeleted(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := "pipelinerun-referencing-deleted"
	t.Logf("Creating Pipeline, and PipelineRun %s in namespace %s", prName, namespace)

	pipeline := mustParsePipeline(t, `
metadata:
  name: pipeline-to-be-deleted
spec:
  tasks:
  - name: step1
    taskSpec:
      steps:
      - name: echo
        image: ubuntu
        script: |
          #!/usr/bin/env bash
          # Sleep for 10s
          sleep 10
  - name: step2
    runAfter: [step1]
    taskSpec:
      steps:
      - name: echo
        image: ubuntu
        script: |
          #!/usr/bin/env bash
          # Sleep for another 10s
          sleep 10
`)
	if _, err := c.PipelineClient.Create(ctx, pipeline, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", prName, err)
	}

	pipelinerun := mustParsePipelineRun(t, `
metadata:
  name: pipelinerun-referencing-deleted
spec:
  pipelineRef:
    name: pipeline-to-be-deleted
`)
	_, err := c.PipelineRunClient.Create(ctx, pipelinerun, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, Running(prName), "PipelineRunRunning"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}

	if err := c.PipelineClient.Delete(ctx, pipeline.Name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete Pipeline `%s`: %s", pipeline.Name, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}

}

// TestPipelineRunPending tests that a Pending PipelineRun is not run until the pending
// status is cleared. This is separate from the TestPipelineRun suite because it has to
// transition PipelineRun states during the test, which the TestPipelineRun suite does not
// support.
func TestPipelineRunPending(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c, namespace := setup(ctx, t)

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := "pending-pipelinerun-test"

	t.Logf("Creating Task, Pipeline, and Pending PipelineRun %s in namespace %s", prName, namespace)

	if _, err := c.TaskClient.Create(ctx, &v1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{Name: prName, Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo hello, world"},
			}}},
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task `%s`: %s", prName, err)
	}

	if _, err := c.PipelineClient.Create(ctx, &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: prName, Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    "task",
				TaskRef: &v1beta1.TaskRef{Name: prName},
			}},
		},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Pipeline `%s`: %s", prName, err)
	}

	pipelineRun, err := c.PipelineRunClient.Create(ctx, &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: prName, Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: prName},
			Status:      v1beta1.PipelineRunSpecStatusPending,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create PipelineRun `%s`: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to be marked pending", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunPending(prName), "PipelineRunPending"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to be marked pending: %s", prName, err)
	}

	t.Logf("Clearing pending status on PipelineRun %s", prName)

	pipelineRun, err = c.PipelineRunClient.Get(ctx, prName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error getting PipelineRun %s: %s", prName, err)
	}

	if pipelineRun.Status.StartTime != nil {
		t.Fatalf("Error start time must be nil, not: %s", pipelineRun.Status.StartTime)
	}

	pipelineRun.Spec.Status = ""

	if _, err := c.PipelineRunClient.Update(ctx, pipelineRun, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Error clearing pending status on PipelineRun %s: %s", prName, err)
	}

	t.Logf("Waiting for PipelineRun %s in namespace %s to complete", prName, namespace)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout, PipelineRunSucceed(prName), "PipelineRunSuccess"); err != nil {
		t.Fatalf("Error waiting for PipelineRun %s to finish: %s", prName, err)
	}
}

func getFanInFanOutTasks(namespace string) []*v1beta1.Task {
	workspaceResource := v1beta1.TaskResource{ResourceDeclaration: v1beta1.ResourceDeclaration{
		Name: "workspace",
		Type: resource.PipelineResourceTypeGit,
	}}
	return []*v1beta1.Task{{
		ObjectMeta: metav1.ObjectMeta{Name: "create-file", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name:       "workspace",
					Type:       resource.PipelineResourceTypeGit,
					TargetPath: "brandnewspace",
				}}},
				Outputs: []v1beta1.TaskResource{workspaceResource},
			},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "write-data-task-0-step-0",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo stuff > $(resources.outputs.workspace.path)/stuff"},
			}}, {Container: corev1.Container{
				Name:    "write-data-task-0-step-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo other > $(resources.outputs.workspace.path)/other"},
			}}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "check-create-files-exists", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs:  []v1beta1.TaskResource{workspaceResource},
				Outputs: []v1beta1.TaskResource{workspaceResource},
			},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "read-from-task-0",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "[[ stuff == $(cat $(inputs.resources.workspace.path)/stuff) ]]"},
			}}, {Container: corev1.Container{
				Name:    "write-data-task-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo something > $(outputs.resources.workspace.path)/something"},
			}}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "check-create-files-exists-2", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs:  []v1beta1.TaskResource{workspaceResource},
				Outputs: []v1beta1.TaskResource{workspaceResource},
			},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "read-from-task-0",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "[[ other == $(cat $(inputs.resources.workspace.path)/other) ]]"},
			}}, {Container: corev1.Container{
				Name:    "write-data-task-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "echo else > $(outputs.resources.workspace.path)/else"},
			}}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Name: "read-files", Namespace: namespace},
		Spec: v1beta1.TaskSpec{
			Resources: &v1beta1.TaskResources{
				Inputs: []v1beta1.TaskResource{{ResourceDeclaration: v1beta1.ResourceDeclaration{
					Name:       "workspace",
					Type:       resource.PipelineResourceTypeGit,
					TargetPath: "readingspace",
				}}},
			},
			Steps: []v1beta1.Step{{Container: corev1.Container{
				Name:    "read-from-task-0",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "[[ something == $(cat $(inputs.resources.workspace.path)/something) ]]"},
			}}, {Container: corev1.Container{
				Name:    "read-from-task-1",
				Image:   "ubuntu",
				Command: []string{"/bin/bash"},
				Args:    []string{"-c", "[[ else == $(cat $(inputs.resources.workspace.path)/else) ]]"},
			}}},
		},
	}}
}

func getFanInFanOutPipeline(suffix int, namespace string) *v1beta1.Pipeline {
	outGitResource := v1beta1.PipelineTaskOutputResource{
		Name:     "workspace",
		Resource: "git-repo",
	}
	return &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: getName(pipelineName, suffix), Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Resources: []v1beta1.PipelineDeclaredResource{{
				Name: "git-repo", Type: resource.PipelineResourceTypeGit,
			}},
			Tasks: []v1beta1.PipelineTask{{
				Name:    "create-file-kritis",
				TaskRef: &v1beta1.TaskRef{Name: "create-file"},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "workspace", Resource: "git-repo",
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{outGitResource},
				},
			}, {
				Name:    "create-fan-out-1",
				TaskRef: &v1beta1.TaskRef{Name: "check-create-files-exists"},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "workspace", Resource: "git-repo", From: []string{"create-file-kritis"},
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{outGitResource},
				},
			}, {
				Name:    "create-fan-out-2",
				TaskRef: &v1beta1.TaskRef{Name: "check-create-files-exists-2"},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "workspace", Resource: "git-repo", From: []string{"create-file-kritis"},
					}},
					Outputs: []v1beta1.PipelineTaskOutputResource{outGitResource},
				},
			}, {
				Name:    "check-fan-in",
				TaskRef: &v1beta1.TaskRef{Name: "read-files"},
				Resources: &v1beta1.PipelineTaskResources{
					Inputs: []v1beta1.PipelineTaskInputResource{{
						Name: "workspace", Resource: "git-repo", From: []string{"create-fan-out-2", "create-fan-out-1"},
					}},
				},
			}},
		},
	}
}

func getFanInFanOutGitResources() []*v1alpha1.PipelineResource {
	return []*v1alpha1.PipelineResource{
		tb.PipelineResource("kritis-resource-git", tb.PipelineResourceSpec(
			v1alpha1.PipelineResourceTypeGit,
			tb.PipelineResourceSpecParam("Url", "https://github.com/grafeas/kritis"),
			tb.PipelineResourceSpecParam("Revision", "master"),
		)),
	}
}

func getPipelineRunServiceAccount(suffix int, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(saName, suffix),
		},
		Secrets: []corev1.ObjectReference{{
			Name: getName(secretName, suffix),
		}},
	}
}
func getFanInFanOutPipelineRun(suffix int, namespace string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{Name: getName(pipelineRunName, suffix), Namespace: namespace},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: getName(pipelineName, suffix)},
			Resources: []v1beta1.PipelineResourceBinding{{
				Name:        "git-repo",
				ResourceRef: &v1beta1.PipelineResourceRef{Name: "kritis-resource-git"},
			}},
		},
	}
}

func getPipelineRunSecret(suffix int, namespace string) *corev1.Secret {
	// Generated by:
	//   cat /tmp/key.json | base64 -w 0
	// This service account is JUST a storage reader on gcr.io/build-crd-testing
	encoedDockercred := "ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIsCiAgInByb2plY3RfaWQiOiAiYnVpbGQtY3JkLXRlc3RpbmciLAogICJwcml2YXRlX2tleV9pZCI6ICIwNTAyYTQxYTgxMmZiNjRjZTU2YTY4ZWM1ODMyYWIwYmExMWMxMWU2IiwKICAicHJpdmF0ZV9rZXkiOiAiLS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tXG5NSUlFdlFJQkFEQU5CZ2txaGtpRzl3MEJBUUVGQUFTQ0JLY3dnZ1NqQWdFQUFvSUJBUUM5WDRFWU9BUmJ4UU04XG5EMnhYY2FaVGsrZ1k4ZWp1OTh0THFDUXFUckdNVzlSZVQyeE9ZNUF5Z2FsUFArcDd5WEVja3dCRC9IaE0wZ2xJXG43TVRMZGVlS1dyK3JBMUx3SFp5V0ZXN0gwT25mN3duWUhFSExXVW1jM0JDT1JFRHRIUlo3WnJQQmYxSFRBQS8zXG5Nblc1bFpIU045b2p6U1NGdzZBVnU2ajZheGJCSUlKNzU0THJnS2VBWXVyd2ZJUTJSTFR1MjAxazJJcUxZYmhiXG4zbVNWRzVSK3RiS3oxQ3ZNNTNuSENiN0NmdVZlV3NyQThrazd4SHJyTFFLTW1JOXYyc2dSdWd5TUF6d3ovNnpOXG5oNS9pTXh4Z2VxNVc4eGtWeDNKMm5ZOEpKZEhhZi9UNkFHc09ORW80M3B4ZWlRVmpuUmYvS24xMFRDYzJFc0lZXG5TNDlVc1o3QkFnTUJBQUVDZ2dFQUF1cGxkdWtDUVF1RDVVL2dhbUh0N0dnVzNBTVYxOGVxbkhuQ2EyamxhaCtTXG5BZVVHbmhnSmpOdkUrcE1GbFN2NXVmMnAySzRlZC9veEQ2K0NwOVpYRFJqZ3ZmdEl5cWpsemJ3dkZjZ3p3TnVEXG55Z1VrdXA3SGVjRHNEOFR0ZUFvYlQvVnB3cTZ6S01yQndDdk5rdnk2YlZsb0VqNXgzYlhzYXhlOTVETy95cHU2XG53MFc5N3p4d3dESlk2S1FjSVdNamhyR3h2d1g3bmlVQ2VNNGxlV0JEeUd0dzF6ZUpuNGhFYzZOM2FqUWFjWEtjXG4rNFFseGNpYW1ZcVFXYlBudHhXUWhoUXpjSFdMaTJsOWNGYlpENyt1SkxGNGlONnk4bVZOVTNLM0sxYlJZclNEXG5SVXAzYVVWQlhtRmcrWi8ycHVWTCttVTNqM0xMV1l5Qk9rZXZ1T21kZ1FLQmdRRGUzR0lRa3lXSVMxNFRkTU9TXG5CaUtCQ0R5OGg5NmVoTDBIa0RieU9rU3RQS2RGOXB1RXhaeGh5N29qSENJTTVGVnJwUk4yNXA0c0V6d0ZhYyt2XG5KSUZnRXZxN21YZm1YaVhJTmllUG9FUWFDbm54RHhXZ21yMEhVS0VtUzlvTWRnTGNHVStrQ1ZHTnN6N0FPdW0wXG5LcVkzczIyUTlsUTY3Rk95cWl1OFdGUTdRUUtCZ1FEWmlGaFRFWmtQRWNxWmpud0pwVEI1NlpXUDlLVHNsWlA3XG53VTRiemk2eSttZXlmM01KKzRMMlN5SGMzY3BTTWJqdE5PWkN0NDdiOTA4RlVtTFhVR05oY3d1WmpFUXhGZXkwXG5tNDFjUzVlNFA0OWI5bjZ5TEJqQnJCb3FzMldCYWwyZWdkaE5KU3NDV29pWlA4L1pUOGVnWHZoN2I5MWp6b0syXG5xMlBVbUE0RGdRS0JnQVdMMklqdkVJME95eDJTMTFjbi9lM1dKYVRQZ05QVEc5MDNVcGErcW56aE9JeCtNYXFoXG5QRjRXc3VBeTBBb2dHSndnTkpiTjhIdktVc0VUdkE1d3l5TjM5WE43dzBjaGFyRkwzN29zVStXT0F6RGpuamNzXG5BcTVPN0dQR21YdWI2RUJRQlBKaEpQMXd5NHYvSzFmSGcvRjQ3cTRmNDBMQUpPa2FZUkpENUh6QkFvR0JBTlVoXG5uSUJQSnFxNElNdlE2Y0M5ZzhCKzF4WURlYTkvWWsxdytTbVBHdndyRVh5M0dLeDRLN2xLcGJQejdtNFgzM3N4XG5zRVUvK1kyVlFtd1JhMXhRbS81M3JLN1YybDVKZi9ENDAwalJtNlpmU0FPdmdEVHJ0Wm5VR0pNcno5RTd1Tnc3XG5sZ1VIM0pyaXZ5Ri9meE1JOHFzelFid1hQMCt4bnlxQXhFQWdkdUtCQW9HQUlNK1BTTllXQ1pYeERwU0hJMThkXG5qS2tvQWJ3Mk1veXdRSWxrZXVBbjFkWEZhZDF6c1hRR2RUcm1YeXY3TlBQKzhHWEJrbkJMaTNjdnhUaWxKSVN5XG51Y05yQ01pcU5BU24vZHE3Y1dERlVBQmdqWDE2SkgyRE5GWi9sL1VWRjNOREFKalhDczFYN3lJSnlYQjZveC96XG5hU2xxbElNVjM1REJEN3F4Unl1S3Nnaz1cbi0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS1cbiIsCiAgImNsaWVudF9lbWFpbCI6ICJwdWxsLXNlY3JldC10ZXN0aW5nQGJ1aWxkLWNyZC10ZXN0aW5nLmlhbS5nc2VydmljZWFjY291bnQuY29tIiwKICAiY2xpZW50X2lkIjogIjEwNzkzNTg2MjAzMzAyNTI1MTM1MiIsCiAgImF1dGhfdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi9hdXRoIiwKICAidG9rZW5fdXJpIjogImh0dHBzOi8vYWNjb3VudHMuZ29vZ2xlLmNvbS9vL29hdXRoMi90b2tlbiIsCiAgImF1dGhfcHJvdmlkZXJfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9vYXV0aDIvdjEvY2VydHMiLAogICJjbGllbnRfeDUwOV9jZXJ0X3VybCI6ICJodHRwczovL3d3dy5nb29nbGVhcGlzLmNvbS9yb2JvdC92MS9tZXRhZGF0YS94NTA5L3B1bGwtc2VjcmV0LXRlc3RpbmclNDBidWlsZC1jcmQtdGVzdGluZy5pYW0uZ3NlcnZpY2VhY2NvdW50LmNvbSIKfQo="

	decoded, err := base64.StdEncoding.DecodeString(encoedDockercred)
	if err != nil {
		return nil
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      getName(secretName, suffix),
			Annotations: map[string]string{
				"tekton.dev/docker-0": "https://us.gcr.io",
				"tekton.dev/docker-1": "https://eu.gcr.io",
				"tekton.dev/docker-2": "https://asia.gcr.io",
				"tekton.dev/docker-3": "https://gcr.io",
			},
		},
		Type: "kubernetes.io/basic-auth",
		Data: map[string][]byte{
			"username": []byte("_json_key"),
			"password": decoded,
		},
	}
}

func getHelloWorldPipelineRun(suffix int, namespace string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: getName(pipelineRunName, suffix), Namespace: namespace,
			Labels: map[string]string{
				"hello-world-key": "hello-world-value",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: getName(pipelineName, suffix)},
			Params: []v1beta1.Param{{
				Name:  "the.path",
				Value: *v1beta1.NewArrayOrString("docker://gcr.io/build-crd-testing/secret-sauce"),
			}, {
				Name:  "dest",
				Value: *v1beta1.NewArrayOrString("dir:///tmp/"),
			}},
			ServiceAccountName: fmt.Sprintf("%s%d", saName, suffix),
		},
	}
}

func getName(namespace string, suffix int) string {
	return fmt.Sprintf("%s%d", namespace, suffix)
}

// collectMatchingEvents collects list of events under 5 seconds that match
// 1. matchKinds which is a map of Kind of Object with name of objects
// 2. reason which is the expected reason of event
func collectMatchingEvents(ctx context.Context, kubeClient kubernetes.Interface, namespace string, kinds map[string][]string, reason string) ([]*corev1.Event, error) {
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
func checkLabelPropagation(ctx context.Context, t *testing.T, c *clients, namespace string, pipelineRunName string, tr *v1beta1.TaskRun) {
	// Our controllers add 4 labels automatically. If custom labels are set on
	// the Pipeline, PipelineRun, or Task then the map will have to be resized.
	labels := make(map[string]string, 4)

	// Check label propagation to PipelineRuns.
	pr, err := c.PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", tr.Name, err)
	}
	p, err := c.PipelineClient.Get(ctx, pr.Spec.PipelineRef.Name, metav1.GetOptions{})
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
		task, err := c.TaskClient.Get(ctx, tr.Spec.TaskRef.Name, metav1.GetOptions{})
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

	// PodName is "" iff a retry happened and pod is deleted
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
func checkAnnotationPropagation(ctx context.Context, t *testing.T, c *clients, namespace string, pipelineRunName string, tr *v1beta1.TaskRun) {
	annotations := make(map[string]string)

	// Check annotation propagation to PipelineRuns.
	pr, err := c.PipelineRunClient.Get(ctx, pipelineRunName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Couldn't get expected PipelineRun for %s: %s", tr.Name, err)
	}
	p, err := c.PipelineClient.Get(ctx, pr.Spec.PipelineRef.Name, metav1.GetOptions{})
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
		task, err := c.TaskClient.Get(ctx, tr.Spec.TaskRef.Name, metav1.GetOptions{})
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

func getPodForTaskRun(ctx context.Context, t *testing.T, kubeClient kubernetes.Interface, namespace string, tr *v1beta1.TaskRun) *corev1.Pod {
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
	for key, expectedVal := range expectedLabels {
		if actualVal := actualLabels[key]; actualVal != expectedVal {
			t.Errorf("Expected labels containing %s=%s but labels were %v", key, expectedVal, actualLabels)
		}
	}
}

func assertAnnotationsMatch(t *testing.T, expectedAnnotations, actualAnnotations map[string]string) {
	for key, expectedVal := range expectedAnnotations {
		if actualVal := actualAnnotations[key]; actualVal != expectedVal {
			t.Errorf("Expected annotations containing %s=%s but annotations were %v", key, expectedVal, actualAnnotations)
		}
	}
}

func getPipelineWithFailingCondition(suffix int, namespace string) *v1beta1.Pipeline {
	return &v1beta1.Pipeline{
		ObjectMeta: metav1.ObjectMeta{Name: getName(pipelineName, suffix), Namespace: namespace},
		Spec: v1beta1.PipelineSpec{
			Tasks: []v1beta1.PipelineTask{{
				Name:    task1Name,
				TaskRef: &v1beta1.TaskRef{Name: getName(taskName, suffix)},
				Conditions: []v1beta1.PipelineTaskCondition{{
					ConditionRef: cond1Name,
				}},
			}, {
				Name:     "task2",
				TaskRef:  &v1beta1.TaskRef{Name: getName(taskName, suffix)},
				RunAfter: []string{task1Name},
			}},
		},
	}
}

func getFailingCondition() *v1alpha1.Condition {
	return tbv1alpha1.Condition(cond1Name, tbv1alpha1.ConditionSpec(tbv1alpha1.ConditionSpecCheck("", "ubuntu",
		tb.Command("/bin/bash"), tb.Args("exit 1"))))
}

func getConditionalPipelineRun(suffix int, namespace string) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: getName(pipelineRunName, suffix), Namespace: namespace,
			Labels: map[string]string{
				"hello-world-key": "hello-world-vaule",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			PipelineRef: &v1beta1.PipelineRef{Name: getName(pipelineName, suffix)},
		},
	}
}

func getLimitRange(name, namespace, resourceCPU, resourceMemory, resourceEphemeralStorage string) *corev1.LimitRange {
	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Min: corev1.ResourceList{
						corev1.ResourceCPU:              k8sres.MustParse(resourceCPU),
						corev1.ResourceMemory:           k8sres.MustParse(resourceMemory),
						corev1.ResourceEphemeralStorage: k8sres.MustParse(resourceEphemeralStorage),
					},
				},
			},
		},
	}
}
