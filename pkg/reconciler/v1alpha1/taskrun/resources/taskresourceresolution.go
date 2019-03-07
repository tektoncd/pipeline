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
)

// ResolvedTaskResources contains all the data that is needed to execute
// the TaskRun: the TaskRun, it's Task and the PipelineResources it needs.
type ResolvedTaskResources struct {
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

// ResolveTaskResources looks up PipelineResources referenced by inputs and outputs and returns
// a structure that unites the resolved references and the Task Spec. If referenced PipelineResources
// can't be found, an error is returned.
func ResolveTaskResources(ts *v1alpha1.TaskSpec, taskName string, inputs []v1alpha1.TaskResourceBinding, outputs []v1alpha1.TaskResourceBinding, gr GetResource) (*ResolvedTaskResources, error) {
	rtr := ResolvedTaskResources{
		TaskName: taskName,
		TaskSpec: ts,
		Inputs:   map[string]*v1alpha1.PipelineResource{},
		Outputs:  map[string]*v1alpha1.PipelineResource{},
	}

	for _, r := range inputs {
		rr, err := getResource(&r, gr)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve referenced input PipelineResource %q: %s", r.ResourceRef.Name, err)
		}

		rtr.Inputs[r.Name] = rr
	}

	for _, r := range outputs {
		rr, err := getResource(&r, gr)

		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve referenced output PipelineResource %q: %s", r.ResourceRef.Name, err)
		}

		rtr.Outputs[r.Name] = rr
	}
	return &rtr, nil
}
