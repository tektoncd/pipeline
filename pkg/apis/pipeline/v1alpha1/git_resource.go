/*
Copyright 2019 The Tekton Authors

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

	"github.com/tektoncd/pipeline/pkg/names"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
)

const WorkspaceDir = "/workspace"

var (
	gitSource = "git-source"
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
	Revision   string `json:"revision"`
	Submodules bool   `json:"submodules"`

	GitImage string `json:"-"`
}

// NewGitResource creates a new git resource to pass to a Task
func NewGitResource(gitImage string, r *PipelineResource) (*GitResource, error) {
	if r.Spec.Type != PipelineResourceTypeGit {
		return nil, xerrors.Errorf("GitResource: Cannot create a Git resource from a %s Pipeline Resource", r.Spec.Type)
	}
	gitResource := GitResource{
		Name:       r.Name,
		Type:       r.Spec.Type,
		GitImage:   gitImage,
		Submodules: true,
	}
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "URL"):
			gitResource.URL = param.Value
		case strings.EqualFold(param.Name, "Revision"):
			gitResource.Revision = param.Value
		case strings.EqualFold(param.Name, "Submodules"):
			gitResource.Submodules = toBool(param.Value, true)
		}
	}
	// default revision to master if nothing is provided
	if gitResource.Revision == "" {
		gitResource.Revision = "master"
	}
	return &gitResource, nil
}

func toBool(s string, d bool) bool {
	switch s {
	case "true":
		return true
	case "false":
		return false
	default:
		return d
	}
}

// GetName returns the name of the resource
func (s GitResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "Git"
func (s GitResource) GetType() PipelineResourceType {
	return PipelineResourceTypeGit
}

// GetURL returns the url to be used with this resource
func (s *GitResource) GetURL() string {
	return s.URL
}

// Replacements is used for template replacement on a GitResource inside of a Taskrun.
func (s *GitResource) Replacements() map[string]string {
	return map[string]string{
		"name":     s.Name,
		"type":     string(s.Type),
		"url":      s.URL,
		"revision": s.Revision,
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *GitResource) GetInputTaskModifier(_ *TaskSpec, path string) (TaskModifier, error) {
	args := []string{"-url", s.URL,
		"-revision", s.Revision,
	}

	args = append(args, []string{"-path", path}...)

	step := Step{
		Container: corev1.Container{
			Name:       names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(gitSource + "-" + s.Name),
			Image:      s.GitImage,
			Command:    []string{"/ko-app/git-init"},
			Args:       args,
			WorkingDir: WorkspaceDir,
			// This is used to populate the ResourceResult status.
			Env: []corev1.EnvVar{{
				Name:  "TEKTON_RESOURCE_NAME",
				Value: s.Name,
			}},
		},
	}
	return &InternalTaskModifier{
		StepsToPrepend: []Step{step},
	}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *GitResource) GetOutputTaskModifier(_ *TaskSpec, _ string) (TaskModifier, error) {
	return &InternalTaskModifier{}, nil
}
