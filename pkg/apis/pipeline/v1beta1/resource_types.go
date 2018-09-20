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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResourceType represents the type of endpoint the resource is, so that the
// controller will know this resource should be fetched and optionally what
// additional metatdata should be provided for it.
type ResourceType string

const (
	// ResourceTypeGit indicates that this source is a GitHub repo.
	ResourceTypeGit ResourceType = "git"

	// ResourceTypeGCS indicates that this source is a GCS bucket.
	ResourceTypeGCS ResourceType = "gcs"

	// ResourceTypeImage indicates that this source is a docker Image.
	ResourceTypeImage ResourceType = "image"
)

// ResourceSuper interface to be implemented by different resource types
type ResourceSuper interface {
	getName() string
	getType() ResourceType
	getParams() []Param
	getVersion() string
}

// ResourceStatus should implment status for resource
type ResourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// ResourceSpec defines set of resources required by all Tasks in the pipeline.
type ResourceSpec struct {
	Resources []ResourceItem `json:"resources"`
}

// ResourceItem is an individual resource object definition
type ResourceItem struct {
	Name   string       `json:"name"`
	Type   ResourceType `json:"type"`
	Params []Param      `json:"params"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Resource is the Schema for the resources API
// +k8s:openapi-gen=true
type Resource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceSpec   `json:"spec,omitempty"`
	Status ResourceStatus `json:"status,omitempty"`
}

// ResourceVersion defines the desired state of version of the resource
type ResourceVersion struct {
	ResourceRef ResourceRef `json:"resourceRef"`
	Version     string      `json:"version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceList contains a list of Resources
type ResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Resource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Resource{}, &ResourceList{})
}
