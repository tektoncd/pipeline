/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either extress or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resources

import (
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetTask is a function used to retrieve Tasks.
type GetTask func(string) (v1alpha1.TaskInterface, error)
type GetTaskRun func(string) (*v1alpha1.TaskRun, error)

// GetClusterTask is a function that will retrieve the Task from name and namespace.
type GetClusterTask func(name string) (v1alpha1.TaskInterface, error)

// GetTaskData will retrieve the Task metadata and Spec associated with the
// provided TaskRun. This can come from a reference Task or from the TaskRun's
// metadata and embedded TaskSpec.
func GetTaskData(taskRun *v1alpha1.TaskRun, getTask GetTask) (*metav1.ObjectMeta, *v1alpha1.TaskSpec, error) {
	taskMeta := metav1.ObjectMeta{}
	taskSpec := v1alpha1.TaskSpec{}
	switch {
	case taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "":
		// Get related task for taskrun
		t, err := getTask(taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, nil, fmt.Errorf("error when listing tasks for taskRun %s %v", taskRun.Name, err)
		}
		taskMeta = t.TaskMetadata()
		taskSpec = t.TaskSpec()
	case taskRun.Spec.TaskSpec != nil:
		taskMeta = taskRun.ObjectMeta
		taskSpec = *taskRun.Spec.TaskSpec
	default:
		return nil, nil, fmt.Errorf("TaskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}
	return &taskMeta, &taskSpec, nil
}
