/*
Copyright 2018 The Knative Authors.

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

	"github.com/knative/pkg/apis"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType string

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = "git"

	// PipelineResourceTypeStorage indicates that this source is a storage blob resource.
	PipelineResourceTypeStorage PipelineResourceType = "storage"

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = "image"

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = "cluster"
)

// AllResourceTypes can be used for validation to check if a provided Resource type is one of the known types.
var AllResourceTypes = []PipelineResourceType{PipelineResourceTypeGit, PipelineResourceTypeStorage, PipelineResourceTypeImage, PipelineResourceTypeCluster}

// PipelineResourceInterface interface to be implemented by different PipelineResource types
type PipelineResourceInterface interface {
	GetName() string
	GetType() PipelineResourceType
	GetParams() []Param
	Replacements() map[string]string
	GetDownloadContainerSpec() ([]corev1.Container, error)
	GetUploadContainerSpec() ([]corev1.Container, error)
	SetDestinationDirectory(string)
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
	Params []Param              `json:"params"`
	// Secrets to fetch to populate some of resource fields
	// +optional
	SecretParams []SecretParam `json:"secrets,omitempty"`
}

// PipelineResourceStatus does not contain anything because Resources on their own
// do not have a status, they just hold data which is later used by PipelineRuns
// and TaskRuns.
type PipelineResourceStatus struct {
}

// Check that PipelineResource may be validated and defaulted.
var _ apis.Validatable = (*PipelineResource)(nil)
var _ apis.Defaultable = (*PipelineResource)(nil)

// TaskResource defines an input or output Resource declared as a requirement
// by a Task. The Name field will be used to refer to these Resources within
// the Task definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this Resource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type TaskResource struct {
	Name string               `json:"name"`
	Type PipelineResourceType `json:"type"`
	// +optional
	// TargetPath is the path in workspace directory where the task resource will be copied.
	TargetPath string `json:"targetPath"`
	// +optional
	// Path to the index.json file for output container images
	OutputImageDir string `json:"outputImageDir"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineResource is the Schema for the pipelineResources API
// +k8s:openapi-gen=true
type PipelineResource struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the PipelineResource from the client
	// +optional
	Spec PipelineResourceSpec `json:"spec,omitempty"`
	// Status communicates the observed state of the PipelineResource from the controller
	// +optional
	Status PipelineResourceStatus `json:"status,omitempty"`
}

// PipelineResourceBinding connects a reference to an instance of a PipelineResource
// with a PipelineResource dependency that the Pipeline has declared
type PipelineResourceBinding struct {
	// Name is the name of the PipelineResource in the Pipeline's declaration
	Name string `json:"name,omitempty"`
	// ResourceRef is a reference to the instance of the actual PipelineResource
	// that should be used
	ResourceRef PipelineResourceRef `json:"resourceRef,omitempty"`
}

// TaskResourceBinding points to the PipelineResource that
// will be used for the Task input or output called Name. The optional Path field
// corresponds to a path on disk at which the Resource can be found (used when providing
// the resource via mounted volume, overriding the default logic to fetch the Resource).
type TaskResourceBinding struct {
	Name string `json:"name"`
	// no more than one of the ResourceRef and ResourceSpec may be specified.
	// +optional
	ResourceRef PipelineResourceRef `json:"resourceRef,omitempty"`
	// +optional
	ResourceSpec *PipelineResourceSpec `json:"resourceSpec,omitempty"`
	// +optional
	Paths []string `json:"paths,omitempty"`
}

// PipelineResourceResult used to export the image name and digest as json
type PipelineResourceResult struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineResourceList contains a list of PipelineResources
type PipelineResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineResource `json:"items"`
}

// ResourceFromType returns a PipelineResourceInterface from a PipelineResource's type.
func ResourceFromType(r *PipelineResource) (PipelineResourceInterface, error) {
	switch r.Spec.Type {
	case PipelineResourceTypeGit:
		return NewGitResource(r)
	case PipelineResourceTypeImage:
		return NewImageResource(r)
	case PipelineResourceTypeCluster:
		return NewClusterResource(r)
	case PipelineResourceTypeStorage:
		return NewStorageResource(r)
	}
	return nil, fmt.Errorf("%s is an invalid or unimplemented PipelineResource", r.Spec.Type)
}
