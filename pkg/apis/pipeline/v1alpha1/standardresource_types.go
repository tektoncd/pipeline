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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StandardResourceType represents the type of endpoint the standardresource is, so that the
// controller will know this standardresource should be fetched and optionally what
// additional metatdata should be provided for it.
type StandardResourceType string

const (
	// StandardResourceTypeGit indicates that this source is a GitHub repo.
	StandardResourceTypeGit StandardResourceType = "git"

	// StandardResourceTypeGCS indicates that this source is a GCS bucket.
	StandardResourceTypeGCS StandardResourceType = "gcs"

	// StandardResourceTypeImage indicates that this source is a docker Image.
	StandardResourceTypeImage StandardResourceType = "image"
)

// StandardResourceSuper interface to be implemented by different standardresource types
type StandardResourceSuper interface {
	getName() string
	getType() StandardResourceType
	getParams() []Param
	getVersion() string
}

// StandardResourceStatus should implment status for standardresource
type StandardResourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// StandardResourceSpec defines set of standardresources required by all Tasks in the pipeline.
type StandardResourceSpec struct {
	StandardResources []StandardResourceItem `json:"standardResources"`
}

// StandardResourceItem is an individual standardresource object definition
type StandardResourceItem struct {
	Name   string               `json:"name"`
	Type   StandardResourceType `json:"type"`
	Params []Param              `json:"params"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StandardStandardResource is the Schema for the standardresources API
// +k8s:openapi-gen=true
// TODO(aaron-prindle) StandardResource is used by register.go, changed name arbitrarily
type StandardResource struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the StandardResource from the client
	// +optional
	Spec StandardResourceSpec `json:"spec,omitempty"`
	// Status communicates the observed state of the StandardResource from the controller
	// +optional
	Status StandardResourceStatus `json:"status,omitempty"`
}

// StandardResourceVersion defines the desired state of version of the standardresource
type StandardResourceVersion struct {
	StandardResourceRef StandardResourceRef `json:"standardresourceRef"`
	Version             string              `json:"version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StandardResourceList contains a list of StandardResources
type StandardResourceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StandardResource `json:"items"`
}
