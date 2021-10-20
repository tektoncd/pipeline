/*
Copyright 2019-2020 The Tekton Authors.

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

package git_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
	pipeline "github.com/tektoncd/pipeline/pkg/apis/pipeline"
	resourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/git"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
)

func TestNewGitResource_Invalid(t *testing.T) {
	if _, err := git.NewResource("test-resource", "override-with-git:latest",
		&resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGCS,
			},
		}); err == nil {
		t.Error("Expected error creating Git resource")
	}
}

func TestNewGitResource_Valid(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		pipelineResource *resourcev1alpha1.PipelineResource
		want             *git.Resource
	}{{
		desc: "With Revision",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "Without Revision",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{{
					Name:  "URL",
					Value: "git@github.com:test/test.git",
				}},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "With Refspec",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Refspec",
						Value: "refs/changes/22/222134",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "",
			Refspec:    "refs/changes/22/222134",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "Without Refspec",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{{
					Name:  "URL",
					Value: "git@github.com:test/test.git",
				}},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "With Submodules",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "Without Submodules",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Submodules",
						Value: "false",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "With positive depth",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Depth",
						Value: "8",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      8,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "With zero depth",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Depth",
						Value: "0",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      0,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "Without SSLVerify",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Depth",
						Value: "0",
					},
					{
						Name:  "SSLVerify",
						Value: "false",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      0,
			SSLVerify:  false,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "With HTTPProxy",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Depth",
						Value: "0",
					},
					{
						Name:  "HTTPProxy",
						Value: "http-proxy.git.com",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      0,
			SSLVerify:  true,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "",
			NOProxy:    "",
		},
	}, {
		desc: "With HTTPSProxy",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Depth",
						Value: "0",
					},
					{
						Name:  "HTTPSProxy",
						Value: "https-proxy.git.com",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      0,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "",
		},
	}, {
		desc: "With NOProxy",
		pipelineResource: &resourcev1alpha1.PipelineResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-resource",
			},
			Spec: resourcev1alpha1.PipelineResourceSpec{
				Type: resourcev1alpha1.PipelineResourceTypeGit,
				Params: []resourcev1alpha1.ResourceParam{
					{
						Name:  "URL",
						Value: "git@github.com:test/test.git",
					},
					{
						Name:  "Revision",
						Value: "test",
					},
					{
						Name:  "Depth",
						Value: "0",
					},
					{
						Name:  "NOProxy",
						Value: "*",
					},
				},
			},
		},
		want: &git.Resource{
			Name:       "test-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "test",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      0,
			SSLVerify:  true,
			HTTPProxy:  "",
			HTTPSProxy: "",
			NOProxy:    "*",
		},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := git.NewResource("test-resource", "override-with-git:latest", tc.pipelineResource)
			if err != nil {
				t.Fatalf("Unexpected error creating Git resource: %s", err)
			}

			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Mismatch of Git resource %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGitResource_Replacements(t *testing.T) {
	r := &git.Resource{
		Name:       "git-resource",
		Type:       resourcev1alpha1.PipelineResourceTypeGit,
		URL:        "git@github.com:test/test.git",
		Revision:   "master",
		Refspec:    "",
		Submodules: false,
		Depth:      16,
		SSLVerify:  false,
		HTTPProxy:  "http-proxy.git.com",
		HTTPSProxy: "https-proxy.git.com",
		NOProxy:    "*",
	}

	want := map[string]string{
		"name":       "git-resource",
		"type":       resourcev1alpha1.PipelineResourceTypeGit,
		"url":        "git@github.com:test/test.git",
		"revision":   "master",
		"refspec":    "",
		"submodules": "false",
		"depth":      "16",
		"sslVerify":  "false",
		"httpProxy":  "http-proxy.git.com",
		"httpsProxy": "https-proxy.git.com",
		"noProxy":    "*",
	}

	got := r.Replacements()

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Mismatch of GitResource Replacements %s", diff.PrintWantGot(d))
	}
}

func TestGitResource_GetDownloadTaskModifier(t *testing.T) {
	names.TestingSeed()

	for _, tc := range []struct {
		desc        string
		gitResource *git.Resource
		want        corev1.Container
	}{{
		desc: "With basic values",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-9l9zj",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "Without submodules",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-mz4c7",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-submodules=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "With more depth",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: true,
			Depth:      8,
			SSLVerify:  true,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-mssqb",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-depth",
				"8",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "Without sslVerify",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  false,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-78c5n",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-submodules=false",
				"-sslVerify=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "Without httpProxy",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  false,
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-6nl7g",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-submodules=false",
				"-sslVerify=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "Without httpsProxy",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  false,
			HTTPProxy:  "http-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-j2tds",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-submodules=false",
				"-sslVerify=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "Without noProxy",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  false,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-vr6ds",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-submodules=false",
				"-sslVerify=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
			},
		},
	}, {
		desc: "With Refspec",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "master",
			Refspec:    "refs/tags/v1.0:refs/tags/v1.0 refs/heads/master:refs/heads/master",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-l22wn",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-revision",
				"master",
				"-refspec",
				"refs/tags/v1.0:refs/tags/v1.0 refs/heads/master:refs/heads/master",
				"-submodules=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}, {
		desc: "Without Refspec and without revision",
		gitResource: &git.Resource{
			Name:       "git-resource",
			Type:       resourcev1alpha1.PipelineResourceTypeGit,
			URL:        "git@github.com:test/test.git",
			Revision:   "",
			Refspec:    "",
			GitImage:   "override-with-git:latest",
			Submodules: false,
			Depth:      1,
			SSLVerify:  true,
			HTTPProxy:  "http-proxy.git.com",
			HTTPSProxy: "https-proxy.git.com",
			NOProxy:    "no-proxy.git.com",
		},
		want: corev1.Container{
			Name:    "git-source-git-resource-twkr2",
			Image:   "override-with-git:latest",
			Command: []string{"/ko-app/git-init"},
			Args: []string{
				"-url",
				"git@github.com:test/test.git",
				"-path",
				"/test/test",
				"-submodules=false",
			},
			WorkingDir: "/workspace",
			Env: []corev1.EnvVar{
				{Name: "TEKTON_RESOURCE_NAME", Value: "git-resource"},
				{Name: "HOME", Value: pipeline.HomeDir},
				{Name: "HTTP_PROXY", Value: "http-proxy.git.com"},
				{Name: "HTTPS_PROXY", Value: "https-proxy.git.com"},
				{Name: "NO_PROXY", Value: "no-proxy.git.com"},
			},
		},
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			ts := v1beta1.TaskSpec{}
			modifier, err := tc.gitResource.GetInputTaskModifier(&ts, "/test/test")
			if err != nil {
				t.Fatalf("Unexpected error getting GetDownloadTaskModifier: %s", err)
			}

			if d := cmp.Diff([]v1beta1.Step{{Container: tc.want}}, modifier.GetStepsToPrepend()); d != "" {
				t.Errorf("Mismatch of GitResource DownloadContainerSpec %s", diff.PrintWantGot(d))
			}
		})
	}
}
