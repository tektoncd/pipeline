/*
Copyright 2019 The Tekton Authors.

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

	"github.com/tektoncd/pipeline/test/names"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
)

func Test_Invalid_NewGitResource(t *testing.T) {
	cases := []struct {
		desc             string
		pipelineResource *v1alpha1.PipelineResource
	}{
		{
			desc:             "Wrong resource type",
			pipelineResource: tb.PipelineResource("git-resource", "default", tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGCS)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			_, err := v1alpha1.NewGitResource(tc.pipelineResource)
			if err == nil {
				t.Error("Expected error creating Git resource")
			}
		})
	}
}

func Test_Valid_NewGitResource(t *testing.T) {
	cases := []struct {
		desc             string
		pipelineResource *v1alpha1.PipelineResource
		want             *v1alpha1.GitResource
	}{
		{
			desc: "With Revision",
			pipelineResource: tb.PipelineResource("git-resource", "default",
				tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit,
					tb.PipelineResourceSpecParam("URL", "git@github.com:test/test.git"),
					tb.PipelineResourceSpecParam("Revision", "test"),
				),
			),
			want: &v1alpha1.GitResource{
				Name:     "git-resource",
				Type:     v1alpha1.PipelineResourceTypeGit,
				URL:      "git@github.com:test/test.git",
				Revision: "test",
			},
		},
		{
			desc: "Without Revision",
			pipelineResource: tb.PipelineResource("git-resource", "default",
				tb.PipelineResourceSpec(v1alpha1.PipelineResourceTypeGit,
					tb.PipelineResourceSpecParam("URL", "git@github.com:test/test.git"),
				),
			),
			want: &v1alpha1.GitResource{
				Name:     "git-resource",
				Type:     v1alpha1.PipelineResourceTypeGit,
				URL:      "git@github.com:test/test.git",
				Revision: "master",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := v1alpha1.NewGitResource(tc.pipelineResource)
			if err != nil {
				t.Fatalf("Unexpected error creating Git resource: %s", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Mismatch of Git resource: %s", diff)
			}
		})
	}
}

func Test_GitResource_Replacements(t *testing.T) {
	r := &v1alpha1.GitResource{
		Name:       "git-resource",
		Type:       v1alpha1.PipelineResourceTypeGit,
		URL:        "git@github.com:test/test.git",
		Revision:   "master",
		TargetPath: "/test/test",
	}

	want := map[string]string{
		"name":     "git-resource",
		"type":     string(v1alpha1.PipelineResourceTypeGit),
		"url":      "git@github.com:test/test.git",
		"revision": "master",
		"path":     "/test/test",
	}

	got := r.Replacements()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Mismatch of GitResource Replacements: %s", diff)
	}
}

func Test_GitResource_GetDownloadContainerSpec(t *testing.T) {
	names.TestingSeed()

	r := &v1alpha1.GitResource{
		Name:       "git-resource",
		Type:       v1alpha1.PipelineResourceTypeGit,
		URL:        "git@github.com:test/test.git",
		Revision:   "master",
		TargetPath: "/test/test",
	}

	want := []corev1.Container{{
		Name:    "git-source-git-resource-9l9zj",
		Image:   "override-with-git:latest",
		Command: []string{"/ko-app/git-init"},
		Args: []string{
			"-url",
			"git@github.com:test/test.git",
			"-revision",
			"master",
			"-path",
			"/test/test",
		},
		WorkingDir: "/workspace",
	}}

	got, err := r.GetDownloadContainerSpec()
	if err != nil {
		t.Fatalf("Unexpected error getting DownloadContainerSpec: %s", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Mismatch of GitResource DownloadContainerSpec: %s", diff)
	}
}
