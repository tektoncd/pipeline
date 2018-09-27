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

// ImageResource defines an endpoint where artifacts can be stored, such as images.
type ImageResource struct {
	Name   string               `json:"name"`
	Type   PipelineResourceType `json:"type"`
	URL    string               `json:"url"`
	Digest string               `json:"digest"`
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// GetName returns the name of the resource
func (s ImageResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "image"
func (s ImageResource) GetType() PipelineResourceType {
	return PipelineResourceTypeImage
}

// GetVersion returns the version of the resource
func (s ImageResource) GetVersion() string {
	return s.Digest
}

// GetServiceAccountName returns the service account to be used with this resource
func (s *ImageResource) GetServiceAccountName() string {
	return s.ServiceAccount
}

// GetParams returns the resoruce params
func (s ImageResource) GetParams() []Param { return []Param{} }
