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
	Name string `json:"name"`
	// TODO: maybe an enum, with values like 'registry', GCS bucket
	Type   string `json:"type"`
	URL    string `json:"url"`
	Digest string `json:"digest"`
}

func (s ImageResource) getName() string {
	return s.Name
}

func (s ImageResource) getType() StandardResourceType {
	return StandardResourceTypeImage
}

func (s ImageResource) getVersion() string {
	return s.Digest
}

func (s ImageResource) getParams() []Param { return []Param{} }
