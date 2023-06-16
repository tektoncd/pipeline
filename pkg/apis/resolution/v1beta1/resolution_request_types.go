/*
Copyright 2022 The Tekton Authors

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
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResolutionRequest is an object for requesting the content of
// a Tekton resource like a pipeline.yaml.
//
// +genclient
// +genreconciler
type ResolutionRequest struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the information for the request part of the resource request.
	// +optional
	Spec ResolutionRequestSpec `json:"spec,omitempty"`

	// Status communicates the state of the request and, ultimately,
	// the content of the resolved resource.
	// +optional
	Status ResolutionRequestStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResolutionRequestList is a list of ResolutionRequests.
type ResolutionRequestList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata"`
	Items           []ResolutionRequest `json:"items"`
}

// ResolutionRequestSpec are all the fields in the spec of the
// ResolutionRequest CRD.
type ResolutionRequestSpec struct {
	// Parameters are the runtime attributes passed to
	// the resolver to help it figure out how to resolve the
	// resource being requested. For example: repo URL, commit SHA,
	// path to file, the kind of authentication to leverage, etc.
	// +optional
	// +listType=atomic
	Params []pipelinev1.Param `json:"params,omitempty"`
}

// ResolutionRequestStatus are all the fields in a ResolutionRequest's
// status subresource.
type ResolutionRequestStatus struct {
	duckv1.Status                 `json:",inline"`
	ResolutionRequestStatusFields `json:",inline"`
}

// ResolutionRequestStatusFields are the ResolutionRequest-specific fields
// for the status subresource.
type ResolutionRequestStatusFields struct {
	// Data is a string representation of the resolved content
	// of the requested resource in-lined into the ResolutionRequest
	// object.
	Data string `json:"data"`
	// Deprecated: Use RefSource instead
	Source *pipelinev1.RefSource `json:"source"`

	// RefSource is the source reference of the remote data that records the url, digest
	// and the entrypoint.
	RefSource *pipelinev1.RefSource `json:"refSource"`
}

// GetStatus implements KRShaped.
func (rr *ResolutionRequest) GetStatus() *duckv1.Status {
	return &rr.Status.Status
}
