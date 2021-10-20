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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource shouldx be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType = string

var (
	// AllowedOutputResources are the resource types that can be used as outputs
	AllowedOutputResources = map[PipelineResourceType]bool{
		PipelineResourceTypeStorage: true,
		PipelineResourceTypeGit:     true,
	}
)

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = "git"

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = "storage"

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = "image"

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = "cluster"

	// PipelineResourceTypePullRequest indicates that this source is a SCM Pull Request.
	PipelineResourceTypePullRequest PipelineResourceType = "pullRequest"

	// PipelineResourceTypeCloudEvent indicates that this source is a cloud event URI
	PipelineResourceTypeCloudEvent PipelineResourceType = "cloudEvent"

	// PipelineResourceTypeGCS is the subtype for the GCSResources, which is backed by a GCS blob/directory.
	PipelineResourceTypeGCS PipelineResourceType = "gcs"
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = []PipelineResourceType{PipelineResourceTypeGit, PipelineResourceTypeStorage, PipelineResourceTypeImage, PipelineResourceTypeCluster, PipelineResourceTypePullRequest, PipelineResourceTypeCloudEvent}

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

// PipelineResourceSpec defines  an individual resources used in the pipeline.
type PipelineResourceSpec struct {
	// Description is a user-facing description of the resource that may be
	// used to populate a UI.
	// +optional
	Description string               `json:"description,omitempty"`
	Type        PipelineResourceType `json:"type"`
	Params      []ResourceParam      `json:"params"`
	// Secrets to fetch to populate some of resource fields
	// +optional
	SecretParams []SecretParam `json:"secrets,omitempty"`
}

// SecretParam indicates which secret can be used to populate a field of the resource
type SecretParam struct {
	FieldName  string `json:"fieldName"`
	SecretKey  string `json:"secretKey"`
	SecretName string `json:"secretName"`
}

// ResourceParam declares a string value to use for the parameter called Name, and is used in
// the specific context of PipelineResources.
type ResourceParam struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ResourceDeclaration defines an input or output PipelineResource declared as a requirement
// by another type such as a Task or Condition. The Name field will be used to refer to these
// PipelineResources within the type's definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this PipelineResource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type ResourceDeclaration struct {
	// Name declares the name by which a resource is referenced in the
	// definition. Resources may be referenced by name in the definition of a
	// Task's steps.
	Name string `json:"name"`
	// Type is the type of this resource;
	Type PipelineResourceType `json:"type"`
	// Description is a user-facing description of the declared resource that may be
	// used to populate a UI.
	// +optional
	Description string `json:"description,omitempty"`
	// TargetPath is the path in workspace directory where the resource
	// will be copied.
	// +optional
	TargetPath string `json:"targetPath,omitempty"`
	// Optional declares the resource as optional.
	// By default optional is set to false which makes a resource required.
	// optional: true - the resource is considered optional
	// optional: false - the resource is considered required (equivalent of not specifying it)
	Optional bool `json:"optional,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineResourceList contains a list of PipelineResources
type PipelineResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineResource `json:"items"`
}
