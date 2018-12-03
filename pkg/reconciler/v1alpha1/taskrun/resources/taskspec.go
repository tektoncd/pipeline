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

	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
)

// GetTask is a function used to retrieve Tasks.
type GetTask func(string) (*v1alpha1.Task, error)

// GetTaskSpec will retrieve the Task Spec associated with the provieded TaskRun. This can come from a
// reference Task or from an embeded Task spec.
func GetTaskSpec(taskRunSpec *v1alpha1.TaskRunSpec, taskRunName string, getTask GetTask) (*v1alpha1.TaskSpec, string, error) {
	taskSpec := &v1alpha1.TaskSpec{}
	taskName := ""
	if taskRunSpec.TaskRef != nil && taskRunSpec.TaskRef.Name != "" {
		// Get related task for taskrun
		t, err := getTask(taskRunSpec.TaskRef.Name)
		if err != nil {
			return nil, taskName, fmt.Errorf("error when listing tasks for taskRun %s %v", taskRunName, err)
		}
		taskSpec = &t.Spec
		taskName = t.Name
	} else if taskRunSpec.TaskSpec != nil {
		taskSpec = taskRunSpec.TaskSpec
		taskName = taskRunName
	} else {
		return taskSpec, taskName, fmt.Errorf("TaskRun %s not providing TaskRef or TaskSpec", taskRunName)
	}
	return taskSpec, taskName, nil
}
