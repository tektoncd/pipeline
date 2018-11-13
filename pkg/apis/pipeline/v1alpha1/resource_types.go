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
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineResourceType represents the type of endpoint the pipelineResource is, so that the
// controller will know this pipelineResource should be fetched and optionally what
// additional metatdata should be provided for it.
type PipelineResourceType string

const (
	// PipelineResourceTypeGit indicates that this source is a GitHub repo.
	PipelineResourceTypeGit PipelineResourceType = "git"

	// PipelineResourceTypeGCS indicates that this source is a GCS bucket.
	PipelineResourceTypeGCS PipelineResourceType = "gcs"

	// PipelineResourceTypeImage indicates that this source is a docker Image.
	PipelineResourceTypeImage PipelineResourceType = "image"

	// PipelineResourceTypeCluster indicates that this source is a k8s cluster Image.
	PipelineResourceTypeCluster PipelineResourceType = "cluster"
)

var AllResourceTypes = []PipelineResourceType{PipelineResourceTypeGit, PipelineResourceTypeGCS, PipelineResourceTypeImage, PipelineResourceTypeCluster}

// PipelineResourceInterface interface to be implemented by different PipelineResource types
type PipelineResourceInterface interface {
	GetName() string
	GetType() PipelineResourceType
	GetParams() []Param
	GetVersion() string
	Replacements() map[string]string
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
	// +optional
	Generation int64 `json:"generation,omitempty"`
}

// PipelineResourceStatus should implment status for PipelineResource
type PipelineResourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// Check that PipelineResource may be validated and defaulted.
var _ apis.Validatable = (*PipelineResource)(nil)
var _ apis.Defaultable = (*PipelineResource)(nil)

// Assert that PipelineResource implements the GenericCRD interface.
var _ webhook.GenericCRD = (*PipelineResource)(nil)

// TaskResource defines an input or output Resource declared as a requirement
// by a Task. The Name field will be used to refer to these Resources within
// the Task definition, and when provided as an Input, the Name will be the
// path to the volume mounted containing this Resource as an input (e.g.
// an input Resource named `workspace` will be mounted at `/workspace`).
type TaskResource struct {
	Name       string               `json:"name"`
	Type       PipelineResourceType `json:"type"`
	TargetPath string               `json:"targetPath"`
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

// TaskRunResourceVersion defines the version of the PipelineResource that
// will be used for the Task input or output called Name.
type TaskRunResourceVersion struct {
	Name        string              `json:"name"`
	ResourceRef PipelineResourceRef `json:"resourceRef"`
	Version     string              `json:"version"`
	// +optional
	SourcePaths []string `json:"sourcePaths"`
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
	}
	return nil, fmt.Errorf("%s is an invalid or unimplemented PipelineResource", r.Spec.Type)
}

func (tr TaskResource) GetTargetPath() string {
	if tr.TargetPath == "" {
		return "/"
	}
	return tr.TargetPath
}
