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

package v1beta1

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/go-multierror"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType = resource.PipelineResourceType

var (
	// AllowedOutputResources are the resource types that can be used as outputs
	AllowedOutputResources = resource.AllowedOutputResources
)

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = resource.PipelineResourceTypeGit

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = resource.PipelineResourceTypeStorage

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = resource.PipelineResourceTypeImage

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = resource.PipelineResourceTypeCluster

	// PipelineResourceTypePullRequest indicates that this source is a SCM Pull Request.
	PipelineResourceTypePullRequest PipelineResourceType = resource.PipelineResourceTypePullRequest

	// PipelineResourceTypeCloudEvent indicates that this source is a cloud event URI
	PipelineResourceTypeCloudEvent PipelineResourceType = resource.PipelineResourceTypeCloudEvent
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = resource.AllResourceTypes

// TaskResources allows a Pipeline to declare how its DeclaredPipelineResources
// should be provided to a Task as its inputs and outputs.
type TaskResources struct {
	// Inputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	Inputs []TaskResource `json:"inputs,omitempty"`
	// Outputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	Outputs []TaskResource `json:"outputs,omitempty"`
}

// TaskResource defines an input or output Resource declared as a requirement
// by a Task. The Name field will be used to refer to these Resources within
// the Task definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this Resource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type TaskResource struct {
	ResourceDeclaration `json:",inline"`
}

// TaskRunResources allows a TaskRun to declare inputs and outputs TaskResourceBinding
type TaskRunResources struct {
	// Inputs holds the inputs resources this task was invoked with
	Inputs []TaskResourceBinding `json:"inputs,omitempty"`
	// Outputs holds the inputs resources this task was invoked with
	Outputs []TaskResourceBinding `json:"outputs,omitempty"`
}

// TaskResourceBinding points to the PipelineResource that
// will be used for the Task input or output called Name.
type TaskResourceBinding struct {
	PipelineResourceBinding `json:",inline"`
	// Paths will probably be removed in #1284, and then PipelineResourceBinding can be used instead.
	// The optional Path field corresponds to a path on disk at which the Resource can be found
	// (used when providing the resource via mounted volume, overriding the default logic to fetch the Resource).
	// +optional
	Paths []string `json:"paths,omitempty"`
}

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type ResourceDeclaration = resource.ResourceDeclaration

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
	ResourceSpec *resource.PipelineResourceSpec `json:"resourceSpec,omitempty"`
}

// PipelineResourceResult used to export the image name and digest as json
type PipelineResourceResult struct {
	Key          string `json:"key"`
	Value        string `json:"value"`
	ResourceName string `json:"resourceName,omitempty"`
	// The field ResourceRef should be deprecated and removed in the next API version.
	// See https://github.com/tektoncd/pipeline/issues/2694 for more information.
	ResourceRef *PipelineResourceRef `json:"resourceRef,omitempty"`
	ResultType  ResultType           `json:"type,omitempty"`
}

// ResultType used to find out whether a PipelineResourceResult is from a task result or not
type ResultType int

// UnmarshalJSON unmarshals either an int or a string into a ResultType. String
// ResultTypes were removed because they made JSON messages bigger, which in
// turn limited the amount of space in termination messages for task results. String
// support is maintained for backwards compatibility - the Pipelines controller could
// be stopped midway through TaskRun execution, updated with support for int in place
// of string, and then fail the running TaskRun because it doesn't know how to interpret
// the string value that the TaskRun's entrypoint will emit when it completes.
func (r *ResultType) UnmarshalJSON(data []byte) error {

	var asInt int
	var intErr error

	if err := json.Unmarshal(data, &asInt); err != nil {
		intErr = err
	} else {
		*r = ResultType(asInt)
		return nil
	}

	var asString string

	if err := json.Unmarshal(data, &asString); err != nil {
		return fmt.Errorf("unsupported value type, neither int nor string: %v", multierror.Append(intErr, err).ErrorOrNil())
	}

	switch asString {
	case "TaskRunResult":
		*r = TaskRunResultType
	case "PipelineResourceResult":
		*r = PipelineResourceResultType
	case "InternalTektonResult":
		*r = InternalTektonResultType
	default:
		*r = UnknownResultType
	}

	return nil
}

// PipelineResourceRef can be used to refer to a specific instance of a Resource
type PipelineResourceRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

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
type TaskModifier interface {
	GetStepsToPrepend() []Step
	GetStepsToAppend() []Step
	GetVolumes() []v1.Volume
}

// InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.
type InternalTaskModifier struct {
	StepsToPrepend []Step
	StepsToAppend  []Step
	Volumes        []v1.Volume
}

// GetStepsToPrepend returns a set of Steps to prepend to the Task.
func (tm *InternalTaskModifier) GetStepsToPrepend() []Step {
	return tm.StepsToPrepend
}

// GetStepsToAppend returns a set of Steps to append to the Task.
func (tm *InternalTaskModifier) GetStepsToAppend() []Step {
	return tm.StepsToAppend
}

// GetVolumes returns a set of Volumes to prepend to the Task pod.
func (tm *InternalTaskModifier) GetVolumes() []v1.Volume {
	return tm.Volumes
}

// ApplyTaskModifier applies a modifier to the task by appending and prepending steps and volumes.
// If steps with the same name exist in ts an error will be returned. If identical Volumes have
// been added, they will not be added again. If Volumes with the same name but different contents
// have been added, an error will be returned.
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

func checkStepNotAlreadyAdded(s Step, steps []Step) error {
	for _, step := range steps {
		if s.Name == step.Name {
			return fmt.Errorf("Step %s cannot be added again", step.Name)
		}
	}
	return nil
}
