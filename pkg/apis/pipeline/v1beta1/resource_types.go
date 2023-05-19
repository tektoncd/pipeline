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
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/result"
	v1 "k8s.io/api/core/v1"
)

// RunResult is used to write key/value pairs to TaskRun pod termination messages.
// It has been migrated to the result package and kept for backward compatibility
type RunResult = result.RunResult

// PipelineResourceResult has been deprecated with the migration of PipelineResources
// Deprecated: Use RunResult instead
type PipelineResourceResult = result.RunResult

// ResultType of PipelineResourceResult has been deprecated with the migration of PipelineResources
// Deprecated: v1beta1.ResultType is only kept for backward compatibility
type ResultType = result.ResultType

// ResourceParam declares a string value to use for the parameter called Name, and is used in
// the specific context of PipelineResources.
//
// Deprecated: Unused, preserved only for backwards compatibility
type ResourceParam = resource.ResourceParam

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
//
// Deprecated: Unused, preserved only for backwards compatibility
type PipelineResourceType = resource.PipelineResourceType

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	// Deprecated: Unused, preserved only for backwards compatibility
	PipelineResourceTypeGit PipelineResourceType = resource.PipelineResourceTypeGit

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	// Deprecated: Unused, preserved only for backwards compatibility
	PipelineResourceTypeStorage PipelineResourceType = resource.PipelineResourceTypeStorage
)

// PipelineDeclaredResource is used by a Pipeline to declare the types of the
// PipelineResources that it will required to run and names which can be used to
// refer to these PipelineResources in PipelineTaskResourceBindings.
//
// Deprecated: Unused, preserved only for backwards compatibility
type PipelineDeclaredResource struct {
	// Name is the name that will be used by the Pipeline to refer to this resource.
	// It does not directly correspond to the name of any PipelineResources Task
	// inputs or outputs, and it does not correspond to the actual names of the
	// PipelineResources that will be bound in the PipelineRun.
	Name string `json:"name"`
	// Type is the type of the PipelineResource.
	Type PipelineResourceType `json:"type"`
	// Optional declares the resource as optional.
	// optional: true - the resource is considered optional
	// optional: false - the resource is considered required (default/equivalent of not specifying it)
	Optional bool `json:"optional,omitempty"`
}

// TaskResources allows a Pipeline to declare how its DeclaredPipelineResources
// should be provided to a Task as its inputs and outputs.
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskResources struct {
	// Inputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	// +listType=atomic
	Inputs []TaskResource `json:"inputs,omitempty"`
	// Outputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	// +listType=atomic
	Outputs []TaskResource `json:"outputs,omitempty"`
}

// TaskResource defines an input or output Resource declared as a requirement
// by a Task. The Name field will be used to refer to these Resources within
// the Task definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this Resource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskResource struct {
	ResourceDeclaration `json:",inline"`
}

// TaskRunResources allows a TaskRun to declare inputs and outputs TaskResourceBinding
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskRunResources struct {
	// Inputs holds the inputs resources this task was invoked with
	// +listType=atomic
	Inputs []TaskResourceBinding `json:"inputs,omitempty"`
	// Outputs holds the inputs resources this task was invoked with
	// +listType=atomic
	Outputs []TaskResourceBinding `json:"outputs,omitempty"`
}

// TaskResourceBinding points to the PipelineResource that
// will be used for the Task input or output called Name.
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskResourceBinding struct {
	PipelineResourceBinding `json:",inline"`
	// Paths will probably be removed in #1284, and then PipelineResourceBinding can be used instead.
	// The optional Path field corresponds to a path on disk at which the Resource can be found
	// (used when providing the resource via mounted volume, overriding the default logic to fetch the Resource).
	// +optional
	// +listType=atomic
	Paths []string `json:"paths,omitempty"`
}

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
//
// Deprecated: Unused, preserved only for backwards compatibility
type ResourceDeclaration = resource.ResourceDeclaration

// PipelineResourceBinding connects a reference to an instance of a PipelineResource
// with a PipelineResource dependency that the Pipeline has declared
//
// Deprecated: Unused, preserved only for backwards compatibility
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

// PipelineTaskResources allows a Pipeline to declare how its DeclaredPipelineResources
// should be provided to a Task as its inputs and outputs.
//
// Deprecated: Unused, preserved only for backwards compatibility
type PipelineTaskResources struct {
	// Inputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	// +listType=atomic
	Inputs []PipelineTaskInputResource `json:"inputs,omitempty"`
	// Outputs holds the mapping from the PipelineResources declared in
	// DeclaredPipelineResources to the input PipelineResources required by the Task.
	// +listType=atomic
	Outputs []PipelineTaskOutputResource `json:"outputs,omitempty"`
}

// PipelineTaskInputResource maps the name of a declared PipelineResource input
// dependency in a Task to the resource in the Pipeline's DeclaredPipelineResources
// that should be used. This input may come from a previous task.
//
// Deprecated: Unused, preserved only for backwards compatibility
type PipelineTaskInputResource struct {
	// Name is the name of the PipelineResource as declared by the Task.
	Name string `json:"name"`
	// Resource is the name of the DeclaredPipelineResource to use.
	Resource string `json:"resource"`
	// From is the list of PipelineTask names that the resource has to come from.
	// (Implies an ordering in the execution graph.)
	// +optional
	// +listType=atomic
	From []string `json:"from,omitempty"`
}

// PipelineTaskOutputResource maps the name of a declared PipelineResource output
// dependency in a Task to the resource in the Pipeline's DeclaredPipelineResources
// that should be used.
//
// Deprecated: Unused, preserved only for backwards compatibility
type PipelineTaskOutputResource struct {
	// Name is the name of the PipelineResource as declared by the Task.
	Name string `json:"name"`
	// Resource is the name of the DeclaredPipelineResource to use.
	Resource string `json:"resource"`
}

// TaskRunInputs holds the input values that this task was invoked with.
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskRunInputs struct {
	// +optional
	// +listType=atomic
	Resources []TaskResourceBinding `json:"resources,omitempty"`
	// +optional
	// +listType=atomic
	Params []Param `json:"params,omitempty"`
}

// TaskRunOutputs holds the output values that this task was invoked with.
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskRunOutputs struct {
	// +optional
	// +listType=atomic
	Resources []TaskResourceBinding `json:"resources,omitempty"`
}

// PipelineResourceRef can be used to refer to a specific instance of a Resource
//
// Deprecated: Unused, preserved only for backwards compatibility
type PipelineResourceRef struct {
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name,omitempty"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// PipelineResourceInterface interface to be implemented by different PipelineResource types
//
// Deprecated: Unused, preserved only for backwards compatibility
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
//
// Deprecated: Unused, preserved only for backwards compatibility
type TaskModifier interface {
	GetStepsToPrepend() []Step
	GetStepsToAppend() []Step
	GetVolumes() []v1.Volume
}

// InternalTaskModifier implements TaskModifier for resources that are built-in to Tekton Pipelines.
//
// Deprecated: Unused, preserved only for backwards compatibility
type InternalTaskModifier struct {
	// +listType=atomic
	StepsToPrepend []Step `json:"stepsToPrepend"`
	// +listType=atomic
	StepsToAppend []Step `json:"stepsToAppend"`
	// +listType=atomic
	Volumes []v1.Volume `json:"volumes"`
}
