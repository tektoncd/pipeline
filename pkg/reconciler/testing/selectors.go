/*
Copyright 2026 The Tekton Authors

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

package testing

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ValidateTaskRunsCount ensures that there are `expectedCount` TaskRuns.
// It will fatal the test if the number of TaskRuns is not `expectedCount`.
func ValidateTaskRunsCount(t *testing.T, taskRuns map[string]*v1.TaskRun, expectedCount int) {
	t.Helper()

	actualCount := len(taskRuns)
	if actualCount != expectedCount {
		t.Fatalf("Expected %d taskruns but it has %d", expectedCount, actualCount)
	}
}

// GetTaskRunByName retrieves the TaskRun with the specified name from the given TaskRuns.
// It will fatal the test if the name does not exist.
func GetTaskRunByName(t *testing.T, taskRuns map[string]*v1.TaskRun, expectedName string) *v1.TaskRun {
	t.Helper()

	tr, exist := taskRuns[expectedName]
	if !exist {
		t.Fatalf("Expected taskrun %s does not exist", expectedName)
	}

	return tr
}

// GetTaskRunsForPipelineRun returns the set of TaskRuns associated with the input PipelineRun.
// It will fatal the test if an error occurred.
func GetTaskRunsForPipelineRun(ctx context.Context, t *testing.T, clients clientset.Interface, namespace string, prName string) map[string]*v1.TaskRun {
	t.Helper()
	labelSelector := pipeline.PipelineRunLabelKey + "=" + prName
	return getTaskRuns(ctx, t, clients, namespace, labelSelector)
}

// GetTaskRunsForPipelineTask returns the set of TaskRuns associated with the input PipelineRun and PipelineTask.
// It will fatal the test if an error occurred.
func GetTaskRunsForPipelineTask(ctx context.Context, t *testing.T, clients clientset.Interface, namespace string, prName string, ptLabel string) map[string]*v1.TaskRun {
	t.Helper()
	labelSelector := pipeline.PipelineRunLabelKey + "=" + prName + "," + pipeline.PipelineTaskLabelKey + "=" + ptLabel
	return getTaskRuns(ctx, t, clients, namespace, labelSelector)
}

// getTaskRuns returns the set of TaskRuns matching the label selector.
// It will fatal the test if an error occurred.
func getTaskRuns(ctx context.Context, t *testing.T, clients clientset.Interface, namespace string, labelSelector string) map[string]*v1.TaskRun {
	t.Helper()

	opt := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	taskRuns, err := clients.TektonV1().TaskRuns(namespace).List(ctx, opt)
	if err != nil {
		t.Fatalf("failed to list taskruns, %s", err)
	}

	outputs := make(map[string]*v1.TaskRun)
	for _, item := range taskRuns.Items {
		tr := item
		outputs[item.Name] = &tr
	}

	return outputs
}

// GetChildPipelineRunsForPipelineRun returns the set of child PipelineRuns associated with the input parent PipelineRun.
// It will fatal the test if an error occurred.
func GetChildPipelineRunsForPipelineRun(
	ctx context.Context,
	t *testing.T,
	clients clientset.Interface,
	namespace, parentPipelineRunName string,
) map[string]*v1.PipelineRun {
	t.Helper()

	opt := metav1.ListOptions{
		LabelSelector: pipeline.PipelineRunLabelKey + "=" + parentPipelineRunName,
	}

	pipelineRunList, err := clients.
		TektonV1().
		PipelineRuns(namespace).
		List(ctx, opt)
	if err != nil {
		t.Fatalf("failed to list child PipelineRuns: %v", err)
	}

	result := make(map[string]*v1.PipelineRun)
	for _, pipelineRun := range pipelineRunList.Items {
		result[pipelineRun.Name] = &pipelineRun
	}

	return result
}

// ValidateChildPipelineRunCount ensures that there are `expectedCount` child PipelineRuns.
// It will fatal the test if the number of child PipelineRuns is not `expectedCount`.
func ValidateChildPipelineRunCount(t *testing.T, pipelineRuns map[string]*v1.PipelineRun, expectedCount int) {
	t.Helper()

	actualCount := len(pipelineRuns)
	if actualCount != expectedCount {
		t.Fatalf("Expected %d child PipelineRuns, got %d", expectedCount, actualCount)
	}
}

// GetChildPipelineRunByName retrieves the PipelineRun with the specified name from the given PipelineRuns.
// It will fatal the test if the name does not exist.
func GetChildPipelineRunByName(t *testing.T, pipelineRuns map[string]*v1.PipelineRun, expectedName string) *v1.PipelineRun {
	t.Helper()

	pr, exist := pipelineRuns[expectedName]
	if !exist {
		t.Fatalf("Expected pipelinerun %s does not exist", expectedName)
	}

	return pr
}
