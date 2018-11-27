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

// ResolvedTaskRun contains all the data that is needed to execute
// the TaskRun: the TaskRun, it's Task and the PipelineResources it needs.
type ResolvedTaskRun struct {
	TaskName string
	TaskSpec *v1alpha1.TaskSpec
	// Inputs is a map from the name of the input required by the Task
	// to the actual Resource to use for it
	Inputs map[string]*v1alpha1.PipelineResource
	// Outputs is a map from the name of the output required by the Task
	// to the actual Resource to use for it
	Outputs map[string]*v1alpha1.PipelineResource
}

// GetResource is a function used to retrieve PipelineResources.
type GetResource func(string) (*v1alpha1.PipelineResource, error)

// GetTask is a function used to retrieve Tasks.
type GetTask func(string) (*v1alpha1.Task, error)

// ResolveTaskRun looks up CRDs referenced by the TaskRun and returns an instance
// of TaskRunResource with all of the relevant data populated. If referenced CRDs can't
// be found, an error is returned.
func ResolveTaskRun(tr *v1alpha1.TaskRun, getTask GetTask, gr GetResource) (*ResolvedTaskRun, error) {
	var err error
	rtr := ResolvedTaskRun{
		Inputs:  map[string]*v1alpha1.PipelineResource{},
		Outputs: map[string]*v1alpha1.PipelineResource{},
	}

	rtr.TaskSpec, rtr.TaskName, err = getTaskSpec(tr, getTask)
	if err != nil {
		return nil, fmt.Errorf("couldn't retrieve referenced Task %q: %s", tr.Spec.TaskRef.Name, err)
	}

	for _, r := range tr.Spec.Inputs.Resources {
		rr, err := gr(r.ResourceRef.Name)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve referenced input PipelineResource %q: %s", r.ResourceRef.Name, err)
		}
		rtr.Inputs[r.Name] = rr
	}
	for _, r := range tr.Spec.Outputs.Resources {
		rr, err := gr(r.ResourceRef.Name)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve referenced output PipelineResource %q: %s", r.ResourceRef.Name, err)
		}
		rtr.Outputs[r.Name] = rr
	}
	return &rtr, nil
}

func getTaskSpec(taskRun *v1alpha1.TaskRun, getTask GetTask) (*v1alpha1.TaskSpec, string, error) {
	taskSpec := &v1alpha1.TaskSpec{}
	taskName := ""
	if taskRun.Spec.TaskRef != nil && taskRun.Spec.TaskRef.Name != "" {
		// Get related task for taskrun
		t, err := getTask(taskRun.Spec.TaskRef.Name)
		if err != nil {
			return nil, taskName, fmt.Errorf("error when listing tasks for taskRun %s %v", taskRun.Name, err)
		}
		taskSpec = &t.Spec
		taskName = t.Name
	} else if taskRun.Spec.TaskSpec != nil {
		taskSpec = taskRun.Spec.TaskSpec
		taskName = taskRun.Name
	} else {
		return taskSpec, taskName, fmt.Errorf("TaskRun %s not providing TaskRef or TaskSpec", taskRun.Name)
	}
	return taskSpec, taskName, nil
}
