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

// GitResource is an endpoint from which to get data which is required
// by a Build/Task for context (e.g. a repo from which to build an image).
type GitResource struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URL  string `json:"url"`
	// Git revision (branch, tag, commit SHA or ref) to clone.  See
	// https://git-scm.com/docs/gitrevisions#_specifying_revisions for more
	// information.
	Revision string `json:"revision"`
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

func (s GitResource) getName() string {
	return s.Name
}

func (s GitResource) getType() StandardResourceType {
	return StandardResourceTypeGit
}

func (s GitResource) getVersion() string {
	return s.Revision
}

func (s GitResource) getParams() []Param { return []Param{} }
