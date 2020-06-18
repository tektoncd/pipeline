/*
Copyright 2019-2020 The Tekton Authors

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

package git

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resource "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

var (
	gitSource = "git-source"
)

// Resource is an endpoint from which to get data which is required
// by a Build/Task for context (e.g. a repo from which to build an image).
type Resource struct {
	Name string                        `json:"name"`
	Type resource.PipelineResourceType `json:"type"`
	URL  string                        `json:"url"`
	// Git revision (branch, tag, commit SHA) to clone, and optionally the refspec to fetch from.
	//See https://git-scm.com/docs/gitrevisions#_specifying_revisions for more information.
	Revision   string `json:"revision"`
	Refspec    string `json:"refspec"`
	Submodules bool   `json:"submodules"`

	Depth      uint   `json:"depth"`
	SSLVerify  bool   `json:"sslVerify"`
	HTTPProxy  string `json:"httpProxy"`
	HTTPSProxy string `json:"httpsProxy"`
	NOProxy    string `json:"noProxy"`
	GitImage   string `json:"-"`
}

// NewResource creates a new git resource to pass to a Task
func NewResource(name, gitImage string, r *resource.PipelineResource) (*Resource, error) {
	if r.Spec.Type != resource.PipelineResourceTypeGit {
		return nil, fmt.Errorf("git.Resource: Cannot create a Git resource from a %s Pipeline Resource", r.Spec.Type)
	}
	gitResource := Resource{
		Name:       name,
		Type:       r.Spec.Type,
		GitImage:   gitImage,
		Submodules: true,
		Depth:      1,
		SSLVerify:  true,
	}
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "URL"):
			gitResource.URL = param.Value
		case strings.EqualFold(param.Name, "Revision"):
			gitResource.Revision = param.Value
		case strings.EqualFold(param.Name, "Refspec"):
			gitResource.Refspec = param.Value
		case strings.EqualFold(param.Name, "Submodules"):
			gitResource.Submodules = toBool(param.Value, true)
		case strings.EqualFold(param.Name, "Depth"):
			gitResource.Depth = toUint(param.Value, 1)
		case strings.EqualFold(param.Name, "SSLVerify"):
			gitResource.SSLVerify = toBool(param.Value, true)
		case strings.EqualFold(param.Name, "HTTPProxy"):
			gitResource.HTTPProxy = param.Value
		case strings.EqualFold(param.Name, "HTTPSProxy"):
			gitResource.HTTPSProxy = param.Value
		case strings.EqualFold(param.Name, "NOProxy"):
			gitResource.NOProxy = param.Value
		}
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

func toUint(s string, d uint) uint {
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return d
	}
	return uint(v)
}

// GetName returns the name of the resource
func (s Resource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "Git"
func (s Resource) GetType() resource.PipelineResourceType {
	return resource.PipelineResourceTypeGit
}

// GetURL returns the url to be used with this resource
func (s *Resource) GetURL() string {
	return s.URL
}

// Replacements is used for template replacement on a GitResource inside of a Taskrun.
func (s *Resource) Replacements() map[string]string {
	return map[string]string{
		"name":       s.Name,
		"type":       s.Type,
		"url":        s.URL,
		"revision":   s.Revision,
		"refspec":    s.Refspec,
		"submodules": strconv.FormatBool(s.Submodules),
		"depth":      strconv.FormatUint(uint64(s.Depth), 10),
		"sslVerify":  strconv.FormatBool(s.SSLVerify),
		"httpProxy":  s.HTTPProxy,
		"httpsProxy": s.HTTPSProxy,
		"noProxy":    s.NOProxy,
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *Resource) GetInputTaskModifier(_ *v1beta1.TaskSpec, path string) (v1beta1.TaskModifier, error) {
	args := []string{
		"-url", s.URL,
		"-path", path,
	}

	if s.Revision != "" {
		args = append(args, "-revision", s.Revision)
	}

	if s.Refspec != "" {
		args = append(args, "-refspec", s.Refspec)
	}
	if !s.Submodules {
		args = append(args, "-submodules=false")
	}
	if s.Depth != 1 {
		args = append(args, "-depth", strconv.FormatUint(uint64(s.Depth), 10))
	}
	if !s.SSLVerify {
		args = append(args, "-sslVerify=false")
	}

	env := []corev1.EnvVar{{
		Name:  "TEKTON_RESOURCE_NAME",
		Value: s.Name,
	}, {
		Name:  "HOME",
		Value: pipeline.HomeDir,
	}}

	if len(s.HTTPProxy) != 0 {
		env = append(env, corev1.EnvVar{Name: "HTTP_PROXY", Value: s.HTTPProxy})
	}

	if len(s.HTTPSProxy) != 0 {
		env = append(env, corev1.EnvVar{Name: "HTTPS_PROXY", Value: s.HTTPSProxy})
	}

	if len(s.NOProxy) != 0 {
		env = append(env, corev1.EnvVar{Name: "NO_PROXY", Value: s.NOProxy})
	}

	step := v1beta1.Step{
		Container: corev1.Container{
			Name:       names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(gitSource + "-" + s.Name),
			Image:      s.GitImage,
			Command:    []string{"/ko-app/git-init"},
			Args:       args,
			WorkingDir: pipeline.WorkspaceDir,
			// This is used to populate the ResourceResult status.
			Env: env,
		},
	}

	return &v1beta1.InternalTaskModifier{
		StepsToPrepend: []v1beta1.Step{step},
	}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *Resource) GetOutputTaskModifier(_ *v1beta1.TaskSpec, _ string) (v1beta1.TaskModifier, error) {
	return &v1beta1.InternalTaskModifier{}, nil
}
