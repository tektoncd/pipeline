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

// ImageSource defines an endpoint where artifacts can be stored, such as images.
type ImageSource struct {
	Name string `json:"name"`
	// TODO: maybe an enum, with values like 'registry', GCS bucket
	Type string `json:"type"`
	URL  string `json:"url"`
	Sha  string `json:"sha"`
}

func (s ImageSource) getName() string {
	return s.Name
}

func (s ImageSource) getType() string {
	return "image"
}

func (s ImageSource) getVersion() string {
	return s.Sha
}

func (s ImageSource) getParams() []Param {
	var result []Param
	return result
}
