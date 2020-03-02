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

package v1alpha1

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

// PipelineResourceInterface interface to be implemented by different PipelineResource types
type PipelineResourceInterface interface {
	// GetName returns the name of this PipelineResource instance.
	GetName() string
	// GetType returns the type of this PipelineResource (often a super type, e.g. in the case of storage).
	GetType() PipelineResourceType
	// Replacements returns all the attributes that this PipelineResource has that
	// can be used for variable replacement.
	Replacements() map[string]string
	// GetOutputTaskModifier returns the TaskModifier instance that should be used on a Task
	// in order to add this kind of resource when it is being used as an output.
	GetOutputTaskModifier(ts *TaskSpec, path string) (TaskModifier, error)
	// GetInputTaskModifier returns the TaskModifier instance that should be used on a Task
	// in order to add this kind of resource when it is being used as an input.
	GetInputTaskModifier(ts *TaskSpec, path string) (TaskModifier, error)
}

// TaskModifier is an interface to be implemented by different PipelineResources
type TaskModifier = v1beta1.TaskModifier

// InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.
type InternalTaskModifier = v1beta1.InternalTaskModifier

func checkStepNotAlreadyAdded(s Step, steps []Step) error {
	for _, step := range steps {
		if s.Name == step.Name {
			return fmt.Errorf("Step %s cannot be added again", step.Name)
		}
	}
	return nil
}

// ApplyTaskModifier applies a modifier to the task by appending and prepending steps and volumes.
// If steps with the same name exist in ts an error will be returned. If identical Volumes have
// been added, they will not be added again. If Volumes with the same name but different contents
// have been added, an error will be returned.
// FIXME(vdemeester) de-duplicate this
func ApplyTaskModifier(ts *TaskSpec, tm TaskModifier) error {
	steps := tm.GetStepsToPrepend()
	for _, step := range steps {
		if err := checkStepNotAlreadyAdded(step, ts.Steps); err != nil {
			return err
		}
	}
	ts.Steps = append(steps, ts.Steps...)

	steps = tm.GetStepsToAppend()
	for _, step := range steps {
		if err := checkStepNotAlreadyAdded(step, ts.Steps); err != nil {
			return err
		}
	}
	ts.Steps = append(ts.Steps, steps...)

	volumes := tm.GetVolumes()
	for _, volume := range volumes {
		var alreadyAdded bool
		for _, v := range ts.Volumes {
			if volume.Name == v.Name {
				// If a Volume with the same name but different contents has already been added, we can't add both
				if d := cmp.Diff(volume, v); d != "" {
					return fmt.Errorf("tried to add volume %s already added but with different contents", volume.Name)
				}
				// If an identical Volume has already been added, don't add it again
				alreadyAdded = true
			}
		}
		if !alreadyAdded {
			ts.Volumes = append(ts.Volumes, volume)
		}
	}

	return nil
}

// PipelineResourceBinding connects a reference to an instance of a PipelineResource
// with a PipelineResource dependency that the Pipeline has declared
type PipelineResourceBinding = v1beta1.PipelineResourceBinding

// PipelineResourceResult used to export the image name and digest as json
type PipelineResourceResult = v1beta1.PipelineResourceResult

// ResultType used to find out whether a PipelineResourceResult is from a task result or not
type ResultType = v1beta1.ResultType
