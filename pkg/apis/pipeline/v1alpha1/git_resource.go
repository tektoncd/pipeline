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
	"strings"
)

// GitResource is an endpoint from which to get data which is required
// by a Build/Task for context (e.g. a repo from which to build an image).
type GitResource struct {
	Name string               `json:"name"`
	Type PipelineResourceType `json:"type"`
	URL  string               `json:"url"`
	// Git revision (branch, tag, commit SHA or ref) to clone.  See
	// https://git-scm.com/docs/gitrevisions#_specifying_revisions for more
	// information.
	Revision string `json:"revision"`
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// NewGitResource create a new git resource to pass to Knativve Build
func NewGitResource(r *PipelineResource) *GitResource {
	gitResource := GitResource{
		Name: r.Name,
		Type: r.Spec.Type,
	}
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "URL"):
			gitResource.URL = param.Value
		case strings.EqualFold(param.Name, "serviceAccount"):
			gitResource.ServiceAccount = param.Value
		case strings.EqualFold(param.Name, "Revision"):
			gitResource.Revision = param.Value
		}
	}
	return &gitResource
}

// GetName returns the name of the resource
func (s GitResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "Git"
func (s GitResource) GetType() PipelineResourceType {
	return PipelineResourceTypeGit
}

// GetVersion returns the revision of the resource, See
// https://git-scm.com/docs/gitrevisions#_specifying_revisions for
// more details what the revison in github is
func (s GitResource) GetVersion() string {
	return s.Revision
}

// GetServiceAccountName returns the service account to be used with this resource
func (s *GitResource) GetServiceAccountName() string {
	return s.ServiceAccount
}

// GetURL returns the url to be used with this resource
func (s *GitResource) GetURL() string {
	return s.URL
}

// GetParams returns the resoruce params
func (s GitResource) GetParams() []Param { return []Param{} }
