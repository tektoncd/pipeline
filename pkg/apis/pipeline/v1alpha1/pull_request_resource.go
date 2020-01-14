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
	"fmt"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	prSource       = "pr-source"
	authTokenField = "authToken"
	// nolint: gosec
	authTokenEnv = "AUTH_TOKEN"
)

// PullRequestResource is an endpoint from which to get data which is required
// by a Build/Task for context.
type PullRequestResource struct {
	Name string               `json:"name"`
	Type PipelineResourceType `json:"type"`

	// URL pointing to the pull request.
	// Example: https://github.com/owner/repo/pulls/1
	URL string `json:"url"`
	// SCM provider (github or gitlab today). This will be guessed from URL if not set.
	Provider string `json:"provider"`
	// Secrets holds a struct to indicate a field name and corresponding secret name to populate it.
	Secrets []SecretParam `json:"secrets"`

	PRImage               string `json:"-"`
	InsecureSkipTLSVerify bool   `json:"insecure-skip-tls-verify"`
}

// NewPullRequestResource create a new git resource to pass to a Task
func NewPullRequestResource(prImage string, r *PipelineResource) (*PullRequestResource, error) {
	if r.Spec.Type != PipelineResourceTypePullRequest {
		return nil, fmt.Errorf("cannot create a PR resource from a %s Pipeline Resource", r.Spec.Type)
	}
	prResource := PullRequestResource{
		Name:                  r.Name,
		Type:                  r.Spec.Type,
		Secrets:               r.Spec.SecretParams,
		PRImage:               prImage,
		InsecureSkipTLSVerify: false,
	}
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "URL"):
			prResource.URL = param.Value
		case strings.EqualFold(param.Name, "Provider"):
			prResource.Provider = param.Value
		case strings.EqualFold(param.Name, "insecure-skip-tls-verify"):
			verify, err := strconv.ParseBool(param.Value)
			if err != nil {
				return nil, fmt.Errorf("error occurred converting %q to boolean in Pipeline Resource %s", param.Value, r.Name)
			}
			prResource.InsecureSkipTLSVerify = verify
		}
	}

	return &prResource, nil
}

// GetName returns the name of the resource
func (s PullRequestResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "Git"
func (s PullRequestResource) GetType() PipelineResourceType {
	return PipelineResourceTypePullRequest
}

// GetURL returns the url to be used with this resource
func (s *PullRequestResource) GetURL() string {
	return s.URL
}

// Replacements is used for template replacement on a PullRequestResource inside of a Taskrun.
func (s *PullRequestResource) Replacements() map[string]string {
	return map[string]string{
		"name":                     s.Name,
		"type":                     string(s.Type),
		"url":                      s.URL,
		"provider":                 s.Provider,
		"insecure-skip-tls-verify": strconv.FormatBool(s.InsecureSkipTLSVerify),
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *PullRequestResource) GetInputTaskModifier(ts *TaskSpec, sourcePath string) (TaskModifier, error) {
	return &InternalTaskModifier{
		StepsToPrepend: s.getSteps("download", sourcePath),
	}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *PullRequestResource) GetOutputTaskModifier(ts *TaskSpec, sourcePath string) (TaskModifier, error) {
	return &InternalTaskModifier{
		StepsToAppend: s.getSteps("upload", sourcePath),
	}, nil
}

func (s *PullRequestResource) getSteps(mode string, sourcePath string) []Step {
	args := []string{"-url", s.URL, "-path", sourcePath, "-mode", mode}
	if s.Provider != "" {
		args = append(args, []string{"-provider", s.Provider}...)
	}
	if s.InsecureSkipTLSVerify {
		args = append(args, "-insecure-skip-tls-verify=true")
	}

	evs := []corev1.EnvVar{}
	for _, sec := range s.Secrets {
		if strings.EqualFold(sec.FieldName, authTokenField) {
			ev := corev1.EnvVar{
				Name: authTokenEnv,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: sec.SecretName,
						},
						Key: sec.SecretKey,
					},
				},
			}
			evs = append(evs, ev)
		}
	}

	return []Step{{Container: corev1.Container{
		Name:       names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(prSource + "-" + s.Name),
		Image:      s.PRImage,
		Command:    []string{"/ko-app/pullrequest-init"},
		Args:       args,
		WorkingDir: pipeline.WorkspaceDir,
		Env:        evs,
	}}}
}
