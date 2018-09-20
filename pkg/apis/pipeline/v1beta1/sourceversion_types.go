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

// SourceVersionSpec defines the desired state of SourceVersion
type SourceVersionSpec struct {
	SourceRef SourceRef `json:"sourceRef"`
	Version   string    `json:"string"`
}

// SourceVersionStatus defines the observed state of TaskRun
type SourceVersionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceVersion is the Schema for the sourceversion API
// +k8s:openapi-gen=true
type SourceVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SourceVersionSpec   `json:"spec,omitempty"`
	Status SourceVersionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceVersionList contains a list of SourceVersion
type SourceVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SourceVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SourceVersion{}, &SourceVersionList{})
}
