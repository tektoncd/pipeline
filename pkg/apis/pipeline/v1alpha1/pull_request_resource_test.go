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

package v1alpha1_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"

	tb "github.com/tektoncd/pipeline/test/builder"
)

func TestPullRequest_NewResource(t *testing.T) {
	url := "https://github.com/tektoncd/pipeline/pulls/1"
	pr := tb.PipelineResource("foo", "default", tb.PipelineResourceSpec(
		v1alpha1.PipelineResourceTypePullRequest,
		tb.PipelineResourceSpecParam("type", "github"),
		tb.PipelineResourceSpecParam("url", url),
		tb.PipelineResourceSpecSecretParam("githubToken", "test-secret-key", "test-secret-name"),
	))
	got, err := v1alpha1.NewPullRequestResource(pr)
	if err != nil {
		t.Fatalf("Error creating storage resource: %s", err.Error())
	}

	want := &v1alpha1.PullRequestResource{
		Name:    pr.Name,
		Type:    v1alpha1.PipelineResourceTypePullRequest,
		URL:     url,
		Secrets: pr.Spec.SecretParams,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestPullRequest_NewResource_error(t *testing.T) {
	pr := tb.PipelineResource("foo", "default", tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit))
	if _, err := v1alpha1.NewPullRequestResource(pr); err == nil {
		t.Error("NewPullRequestResource() want error, got nil")
	}
}

type testcase struct {
	in  *v1alpha1.PullRequestResource
	out []corev1.Container
}

func containerTestCases(mode string) []testcase {
	return []testcase{
		{
			in: &v1alpha1.PullRequestResource{
				Name:           "nocreds",
				DestinationDir: "/workspace",
				URL:            "https://example.com",
			},
			out: []corev1.Container{{
				Name:       "pr-source-nocreds-9l9zj",
				Image:      "override-with-pr:latest",
				WorkingDir: v1alpha1.WorkspaceDir,
				Command:    []string{"/ko-app/pullrequest-init"},
				Args:       []string{"-url", "https://example.com", "-path", "/workspace", "-mode", mode},
				Env:        []corev1.EnvVar{},
			}},
		},
		{
			in: &v1alpha1.PullRequestResource{
				Name:           "creds",
				DestinationDir: "/workspace",
				URL:            "https://example.com",
				Secrets: []v1alpha1.SecretParam{{
					FieldName:  "githubToken",
					SecretName: "github-creds",
					SecretKey:  "token",
				}},
			},
			out: []corev1.Container{{
				Name:       "pr-source-creds-mz4c7",
				Image:      "override-with-pr:latest",
				WorkingDir: v1alpha1.WorkspaceDir,
				Command:    []string{"/ko-app/pullrequest-init"},
				Args:       []string{"-url", "https://example.com", "-path", "/workspace", "-mode", mode},
				Env: []corev1.EnvVar{{
					Name: "GITHUBTOKEN",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "github-creds",
							},
							Key: "token",
						},
					},
				}},
			}},
		},
	}
}

func TestPullRequest_GetDownloadContainerSpec(t *testing.T) {
	names.TestingSeed()

	for _, tc := range containerTestCases("download") {
		t.Run(tc.in.GetName(), func(t *testing.T) {
			got, err := tc.in.GetDownloadContainerSpec()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.out, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestPullRequest_GetUploadContainerSpec(t *testing.T) {
	names.TestingSeed()

	for _, tc := range containerTestCases("upload") {
		t.Run(tc.in.GetName(), func(t *testing.T) {
			got, err := tc.in.GetUploadContainerSpec()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.out, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}
