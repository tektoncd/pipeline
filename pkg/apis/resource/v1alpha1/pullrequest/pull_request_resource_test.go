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

package pullrequest_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/pullrequest"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestPullRequest_NewResource(t *testing.T) {
	url := "https://github.com/tektoncd/pipeline/pulls/1"
	pr := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypePullRequest,
			Params: []resourcev1alpha1.ResourceParam{
				{
					Name:  "url",
					Value: url,
				},
				{
					Name:  "provider",
					Value: "github",
				},
				{
					Name:  "disable-strict-json-comments",
					Value: "true",
				},
			},
			SecretParams: []resourcev1alpha1.SecretParam{{
				FieldName:  "authToken",
				SecretKey:  "test-secret-key",
				SecretName: "test-secret-name",
			}},
		},
	}
	got, err := pullrequest.NewResource("test-resource", "override-with-pr:latest", pr)
	if err != nil {
		t.Fatalf("Error creating storage resource: %s", err.Error())
	}

	want := &pullrequest.Resource{
		Name:                      "test-resource",
		Type:                      resourcev1alpha1.PipelineResourceTypePullRequest,
		URL:                       url,
		Provider:                  "github",
		Secrets:                   pr.Spec.SecretParams,
		PRImage:                   "override-with-pr:latest",
		InsecureSkipTLSVerify:     false,
		DisableStrictJSONComments: true,
	}
	if d := cmp.Diff(want, got); d != "" {
		t.Error(diff.PrintWantGot(d))
	}
}

func TestPullRequest_NewResource_error(t *testing.T) {
	pr := &resourcev1alpha1.PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: resourcev1alpha1.PipelineResourceSpec{
			Type: resourcev1alpha1.PipelineResourceTypeGit,
		},
	}
	if _, err := pullrequest.NewResource("test-resource", "override-with-pr:latest", pr); err == nil {
		t.Error("NewPullRequestResource() want error, got nil")
	}
}

type testcase struct {
	in  *pullrequest.Resource
	out []v1beta1.Step
}

const workspace = "/workspace"

func containerTestCases(mode string) []testcase {
	return []testcase{{
		in: &pullrequest.Resource{
			Name:                  "nocreds",
			URL:                   "https://example.com",
			PRImage:               "override-with-pr:latest",
			InsecureSkipTLSVerify: false,
		},
		out: []v1beta1.Step{{Container: corev1.Container{
			Name:       "pr-source-nocreds-9l9zj",
			Image:      "override-with-pr:latest",
			WorkingDir: pipeline.WorkspaceDir,
			Command:    []string{"/ko-app/pullrequest-init"},
			Args:       []string{"-url", "https://example.com", "-path", workspace, "-mode", mode},
			Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "nocreds"}},
		}}},
	}, {
		in: &pullrequest.Resource{
			Name:                  "creds",
			URL:                   "https://example.com",
			InsecureSkipTLSVerify: false,
			Secrets: []resourcev1alpha1.SecretParam{{
				FieldName:  "authToken",
				SecretName: "github-creds",
				SecretKey:  "token",
			}},
			PRImage:  "override-with-pr:latest",
			Provider: "github",
		},
		out: []v1beta1.Step{{Container: corev1.Container{
			Name:       "pr-source-creds-mz4c7",
			Image:      "override-with-pr:latest",
			WorkingDir: pipeline.WorkspaceDir,
			Command:    []string{"/ko-app/pullrequest-init"},
			Args:       []string{"-url", "https://example.com", "-path", "/workspace", "-mode", mode, "-provider", "github"},
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "creds"},
				{
					Name: "AUTH_TOKEN",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "github-creds",
							},
							Key: "token",
						},
					},
				}},
		}}},
	}, {
		in: &pullrequest.Resource{
			Name:                  "nocreds",
			URL:                   "https://example.com",
			PRImage:               "override-with-pr:latest",
			InsecureSkipTLSVerify: true,
		},
		out: []v1beta1.Step{{Container: corev1.Container{
			Name:       "pr-source-nocreds-mssqb",
			Image:      "override-with-pr:latest",
			WorkingDir: pipeline.WorkspaceDir,
			Command:    []string{"/ko-app/pullrequest-init"},
			Args:       []string{"-url", "https://example.com", "-path", workspace, "-mode", mode, "-insecure-skip-tls-verify=true"},
			Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "nocreds"}},
		}}},
	}, {
		in: &pullrequest.Resource{
			Name:                      "strict-json-comments",
			URL:                       "https://example.com",
			PRImage:                   "override-with-pr:latest",
			DisableStrictJSONComments: true,
		},
		out: []v1beta1.Step{{Container: corev1.Container{
			Name:       "pr-source-strict-json-comments-78c5n",
			Image:      "override-with-pr:latest",
			WorkingDir: pipeline.WorkspaceDir,
			Command:    []string{"/ko-app/pullrequest-init"},
			Args:       []string{"-url", "https://example.com", "-path", workspace, "-mode", mode, "-disable-strict-json-comments=true"},
			Env:        []corev1.EnvVar{{Name: "TEKTON_RESOURCE_NAME", Value: "strict-json-comments"}},
		}}},
	}}
}

func TestPullRequest_GetDownloadSteps(t *testing.T) {
	names.TestingSeed()

	for _, tc := range containerTestCases("download") {
		t.Run(tc.in.GetName(), func(t *testing.T) {
			ts := v1beta1.TaskSpec{}
			got, err := tc.in.GetInputTaskModifier(&ts, workspace)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(tc.out, got.GetStepsToPrepend()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestPullRequest_GetOutputSteps(t *testing.T) {
	names.TestingSeed()

	for _, tc := range containerTestCases("upload") {
		t.Run(tc.in.GetName(), func(t *testing.T) {
			ts := v1beta1.TaskSpec{}
			got, err := tc.in.GetOutputTaskModifier(&ts, workspace)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(tc.out, got.GetStepsToAppend()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
