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
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
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
type TaskModifier = v1alpha2.TaskModifier

// InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.
type InternalTaskModifier = v1alpha2.InternalTaskModifier

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
type PipelineResourceBinding struct {
	// Name is the name of the PipelineResource in the Pipeline's declaration
	Name string `json:"name,omitempty"`
	// ResourceRef is a reference to the instance of the actual PipelineResource
	// that should be used
	// +optional
	ResourceRef *PipelineResourceRef `json:"resourceRef,omitempty"`
	// ResourceSpec is specification of a resource that should be created and
	// consumed by the task
	// +optional
	ResourceSpec *PipelineResourceSpec `json:"resourceSpec,omitempty"`
}

// PipelineResourceResult used to export the image name and digest as json
type PipelineResourceResult struct {
	// Name and Digest are deprecated.
	Name   string `json:"name"`
	Digest string `json:"digest"`
	// These will replace Name and Digest (https://github.com/tektoncd/pipeline/issues/1392)
	Key         string              `json:"key"`
	Value       string              `json:"value"`
	ResourceRef PipelineResourceRef `json:"resourceRef,omitempty"`
	ResultType  ResultType          `json:"type,omitempty"`
}

// ResultType used to find out whether a PipelineResourceResult is from a task result or not
type ResultType string

// ResourceFromType returns an instance of the correct PipelineResource object type which can be
// used to add input and output containers as well as volumes to a TaskRun's pod in order to realize
// a PipelineResource in a pod.
func ResourceFromType(r *PipelineResource, images pipeline.Images) (PipelineResourceInterface, error) {
	switch r.Spec.Type {
	case PipelineResourceTypeGit:
		return NewGitResource(images.GitImage, r)
	case PipelineResourceTypeImage:
		return NewImageResource(r)
	case PipelineResourceTypeCluster:
		return NewClusterResource(images.KubeconfigWriterImage, r)
	case PipelineResourceTypeStorage:
		return NewStorageResource(images, r)
	case PipelineResourceTypePullRequest:
		return NewPullRequestResource(images.PRImage, r)
	case PipelineResourceTypeCloudEvent:
		return NewCloudEventResource(r)
	}
	return nil, fmt.Errorf("%s is an invalid or unimplemented PipelineResource", r.Spec.Type)
}
