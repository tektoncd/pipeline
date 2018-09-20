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

// ResourceVersionSpec defines the desired state of SourceVersion
type ResourceVersionSpec struct {
	ResourceName string `json:"name"`
	Version      string `json:"string"`
}

// ResourceVersionStatus defines the observed state of TaskRun
type ResourceVersionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceVersion is the Schema for the sourceversion API
// +k8s:openapi-gen=true
type ResourceVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResourceVersionSpec   `json:"spec,omitempty"`
	Status ResourceVersionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceVersionList contains a list of ResourceVersion
type ResourceVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResourceVersion{}, &ResourceVersionList{})
}
