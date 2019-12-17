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

package resources

import (
	"errors"
	"fmt"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResolvedTaskResources contains all the data that is needed to execute
// the TaskRun: the TaskRun, it's Task and the PipelineResources it needs.
type ResolvedTaskResources struct {
	TaskName string
	Kind     v1alpha1.TaskKind
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
func ResolveTaskResources(ts *v1alpha1.TaskSpec, taskName string, kind v1alpha1.TaskKind, inputs []v1alpha1.TaskResourceBinding, outputs []v1alpha1.TaskResourceBinding, gr GetResource) (*ResolvedTaskResources, error) {
	rtr := ResolvedTaskResources{
		TaskName: taskName,
		TaskSpec: ts,
		Kind:     kind,
		Inputs:   map[string]*v1alpha1.PipelineResource{},
		Outputs:  map[string]*v1alpha1.PipelineResource{},
	}

	for _, r := range inputs {
		rr, err := GetResourceFromBinding(&r.PipelineResourceBinding, gr)
		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve referenced input PipelineResource: %w", err)
		}

		rtr.Inputs[r.Name] = rr
	}

	for _, r := range outputs {
		rr, err := GetResourceFromBinding(&r.PipelineResourceBinding, gr)

		if err != nil {
			return nil, fmt.Errorf("couldn't retrieve referenced output PipelineResource: %w", err)
		}

		rtr.Outputs[r.Name] = rr
	}
	return &rtr, nil
}

// GetResourceFromBinding will return an instance of a PipelineResource to use for r, either by getting it with getter or by
// instantiating it from the embedded spec.
func GetResourceFromBinding(r *v1alpha1.PipelineResourceBinding, getter GetResource) (*v1alpha1.PipelineResource, error) {
	if (r.ResourceRef != nil && r.ResourceRef.Name != "") && r.ResourceSpec != nil {
		return nil, errors.New("Both ResourseRef and ResourceSpec are defined. Expected only one")
	}
	if r.ResourceRef != nil && r.ResourceRef.Name != "" {
		return getter(r.ResourceRef.Name)
	}
	if r.ResourceSpec != nil {
		return &v1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: r.Name,
			},
			Spec: *r.ResourceSpec,
		}, nil
	}
	return nil, errors.New("Neither ResourseRef nor ResourceSpec is defined")
}
