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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType = v1alpha2.PipelineResourceType

var (
	AllowedOutputResources = v1alpha2.AllowedOutputResources
)

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = v1alpha2.PipelineResourceTypeGit

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = v1alpha2.PipelineResourceTypeStorage

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = v1alpha2.PipelineResourceTypeImage

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = v1alpha2.PipelineResourceTypeCluster

	// PipelineResourceTypePullRequest indicates that this source is a SCM Pull Request.
	PipelineResourceTypePullRequest PipelineResourceType = v1alpha2.PipelineResourceTypePullRequest

	// PipelineResourceTypeCloudEvent indicates that this source is a cloud event URI
	PipelineResourceTypeCloudEvent PipelineResourceType = v1alpha2.PipelineResourceTypeCloudEvent
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = v1alpha2.AllResourceTypes

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

// SecretParam indicates which secret can be used to populate a field of the resource
type SecretParam struct {
	FieldName  string `json:"fieldName"`
	SecretKey  string `json:"secretKey"`
	SecretName string `json:"secretName"`
}

// PipelineResourceSpec defines  an individual resources used in the pipeline.
type PipelineResourceSpec struct {
	Type   PipelineResourceType `json:"type"`
	Params []ResourceParam      `json:"params"`
	// Secrets to fetch to populate some of resource fields
	// +optional
	SecretParams []SecretParam `json:"secrets,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:noStatus

// PipelineResource describes a resource that is an input to or output from a
// Task.
//
// +k8s:openapi-gen=true
type PipelineResource struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the PipelineResource from the client
	Spec PipelineResourceSpec `json:"spec,omitempty"`

	// Status is deprecated.
	// It usually is used to communicate the observed state of the PipelineResource from
	// the controller, but was unused as there is no controller for PipelineResource.
	// +optional
	Status *PipelineResourceStatus `json:"status,omitempty"`
}

// PipelineResourceStatus does not contain anything because PipelineResources on their own
// do not have a status
// Deprecated
type PipelineResourceStatus struct {
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
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineResourceList contains a list of PipelineResources
type PipelineResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineResource `json:"items"`
}

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type ResourceDeclaration = v1alpha2.ResourceDeclaration

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
