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

package pullrequest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const (
	prSource       = "pr-source"
	authTokenField = "authToken"
	// nolint: gosec
	authTokenEnv = "AUTH_TOKEN"
)

// Resource is an endpoint from which to get data which is required
// by a Build/Task for context.
type Resource struct {
	Name string                                `json:"name"`
	Type resourcev1alpha1.PipelineResourceType `json:"type"`

	// URL pointing to the pull request.
	// Example: https://github.com/owner/repo/pulls/1
	URL string `json:"url"`
	// SCM provider (github or gitlab today). This will be guessed from URL if not set.
	Provider string `json:"provider"`
	// Secrets holds a struct to indicate a field name and corresponding secret name to populate it.
	Secrets []resourcev1alpha1.SecretParam `json:"secrets"`

	PRImage                   string `json:"-"`
	InsecureSkipTLSVerify     bool   `json:"insecure-skip-tls-verify"`
	DisableStrictJSONComments bool   `json:"disable-strict-json-comments"`
}

// NewResource create a new git resource to pass to a Task
func NewResource(name, prImage string, r *resourcev1alpha1.PipelineResource) (*Resource, error) {
	if r.Spec.Type != resourcev1alpha1.PipelineResourceTypePullRequest {
		return nil, fmt.Errorf("cannot create a PR resource from a %s Pipeline Resource", r.Spec.Type)
	}
	prResource := Resource{
		Name:                      name,
		Type:                      r.Spec.Type,
		Secrets:                   r.Spec.SecretParams,
		PRImage:                   prImage,
		InsecureSkipTLSVerify:     false,
		DisableStrictJSONComments: false,
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
		case strings.EqualFold(param.Name, "disable-strict-json-comments"):
			strict, err := strconv.ParseBool(param.Value)
			if err != nil {
				return nil, fmt.Errorf("error occurred converting %q to boolean in Pipeline Resource %s", param.Value, r.Name)
			}
			prResource.DisableStrictJSONComments = strict
		}
	}

	return &prResource, nil
}

// GetName returns the name of the resource
func (s Resource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "Git"
func (s Resource) GetType() resourcev1alpha1.PipelineResourceType {
	return resourcev1alpha1.PipelineResourceTypePullRequest
}

// GetURL returns the url to be used with this resource
func (s *Resource) GetURL() string {
	return s.URL
}

// Replacements is used for template replacement on a PullRequestResource inside of a Taskrun.
func (s *Resource) Replacements() map[string]string {
	return map[string]string{
		"name":                         s.Name,
		"type":                         s.Type,
		"url":                          s.URL,
		"provider":                     s.Provider,
		"insecure-skip-tls-verify":     strconv.FormatBool(s.InsecureSkipTLSVerify),
		"disable-strict-json-comments": strconv.FormatBool(s.DisableStrictJSONComments),
	}
}

// GetInputTaskModifier returns the TaskModifier to be used when this resource is an input.
func (s *Resource) GetInputTaskModifier(ts *pipelinev1beta1.TaskSpec, sourcePath string) (pipelinev1beta1.TaskModifier, error) {
	return &pipelinev1beta1.InternalTaskModifier{
		StepsToPrepend: s.getSteps("download", sourcePath),
	}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *Resource) GetOutputTaskModifier(ts *pipelinev1beta1.TaskSpec, sourcePath string) (pipelinev1beta1.TaskModifier, error) {
	return &pipelinev1beta1.InternalTaskModifier{
		StepsToAppend: s.getSteps("upload", sourcePath),
	}, nil
}

func (s *Resource) getSteps(mode string, sourcePath string) []pipelinev1beta1.Step {
	args := []string{"-url", s.URL, "-path", sourcePath, "-mode", mode}
	if s.Provider != "" {
		args = append(args, []string{"-provider", s.Provider}...)
	}
	if s.InsecureSkipTLSVerify {
		args = append(args, "-insecure-skip-tls-verify=true")
	}
	if s.DisableStrictJSONComments {
		args = append(args, "-disable-strict-json-comments=true")
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

	return []pipelinev1beta1.Step{{Container: corev1.Container{
		Name:       names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(prSource + "-" + s.Name),
		Image:      s.PRImage,
		Command:    []string{"/ko-app/pullrequest-init"},
		Args:       args,
		WorkingDir: pipeline.WorkspaceDir,
		Env:        evs,
	}}}
}
