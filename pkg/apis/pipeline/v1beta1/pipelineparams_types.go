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

// PipelineParamsSpec is the spec for a Pipeline resource
type PipelineParamsSpec struct {
	ServiceAccount string          `json:"serviceAccount"`
	Sources        []Source        `json:"sources"`
	ArtifactStores []ArtifactStore `json:"artifactStores"`
	Results        Results         `json:"results"`
}

// SourceType represents the type of endpoint the Source is, so that the
// controller will know this Source should be fetched and optionally what
// additional metatdata should be provided for it.
type SourceType string

const (
	// SourceTypeGitHub indicates that this source is a GitHub repo.
	SourceTypeGitHub SourceType = "github"

	// SourceTypeGCS indicates that this source is a GCS bucket.
	SourceTypeGCS SourceType = "gcs"
)

// Source is an endpoint from which to get data which is required
// by a Build/Task for context (e.g. a repo from which to build an image).
type Source struct {
	Name           string     `json:"name"`
	Type           SourceType `json:"type"`
	URL            string     `json:"url"`
	Branch         string     `json:"branch"`
	Commit         string     `json:"commit,omitempty"`
	ServiceAccount string     `json:"serviceAccount,omitempty"`
}

// PipelineParamsStatus defines the observed state of PipelineParams
type PipelineParamsStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineParams is the Schema for the pipelineparams API
// +k8s:openapi-gen=true
type PipelineParams struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PipelineParamsSpec   `json:"spec,omitempty"`
	Status PipelineParamsStatus `json:"status,omitempty"`
}

// ArtifactStore defines an endpoint where artifacts can be stored, such as images.
type ArtifactStore struct {
	Name string `json:"name"`
	// TODO: maybe an enum, with values like 'registry', GCS bucket
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Results tells a pipeline where to persist the results of runnign the pipeline.
type Results struct {
	// Runs is used to store the yaml/json of TaskRuns and PipelineRuns.
	Runs ResultTarget `json:"runs"`

	// Logs will store all logs output from running a task.
	Logs ResultTarget `json:"logs"`

	// Tests will store test results, if a task provides them.
	Tests ResultTarget `json:"tests,omitempty"`
}

// ResultTargetType represents the type of endpoint that this result target is,
// so that the controller will know how to write results to it.
type ResultTargetType string

const (
	// ResultTargetTypeGCS indicates that the URL endpoint is a GCS bucket.
	ResultTargetTypeGCS = "gcs"
)

// ResultTarget is used to identify an endpoint where results can be uploaded. The
// serviceaccount used for the pipeline must have access to this endpoint.
type ResultTarget struct {
	Name string           `json:"name"`
	Type ResultTargetType `json:"type"`
	URL  string           `json:"url"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineParamsList contains a list of PipelineParams
type PipelineParamsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PipelineParams `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PipelineParams{}, &PipelineParamsList{})
}
