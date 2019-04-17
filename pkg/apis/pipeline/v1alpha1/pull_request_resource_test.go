package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPullRequest_NewResource(t *testing.T) {
	url := "https://github.com/tektoncd/pipeline/pulls/1"
	pr := &PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: PipelineResourceSpec{
			Type: PipelineResourceTypePullRequest,
			Params: []Param{{
				Name:  "type",
				Value: "github",
			}, {
				Name:  "url",
				Value: url,
			}},
			SecretParams: []SecretParam{{
				FieldName:  "githubToken",
				SecretKey:  "test-secret-key",
				SecretName: "test-secret-name",
			}},
		},
	}
	got, err := NewPullRequestResource(pr)
	if err != nil {
		t.Fatalf("Error creating storage resource: %s", err.Error())
	}

	want := &PullRequestResource{
		Name:    pr.Name,
		Type:    PipelineResourceTypePullRequest,
		URL:     url,
		Secrets: pr.Spec.SecretParams,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestPullRequest_NewResource_error(t *testing.T) {
	pr := &PipelineResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: PipelineResourceSpec{
			Type: PipelineResourceTypeGit,
		},
	}
	if _, err := NewPullRequestResource(pr); err == nil {
		t.Error("NewPullRequestResource() want error, got nil")
	}
}

type testcase struct {
	in  *PullRequestResource
	out []corev1.Container
}

func containerTestCases(mode string) []testcase {
	return []testcase{
		{
			in: &PullRequestResource{
				Name:           "nocreds",
				DestinationDir: "/workspace",
				URL:            "https://example.com",
			},
			out: []corev1.Container{{
				Name:       "pr-source-nocreds-9l9zj",
				Image:      "override-with-pr:latest",
				WorkingDir: workspaceDir,
				Command:    []string{"/ko-app/pullrequest-init"},
				Args:       []string{"-url", "https://example.com", "-path", "/workspace", "-mode", mode},
				Env:        []corev1.EnvVar{},
			}},
		},
		{
			in: &PullRequestResource{
				Name:           "creds",
				DestinationDir: "/workspace",
				URL:            "https://example.com",
				Secrets: []SecretParam{{
					FieldName:  "githubToken",
					SecretName: "github-creds",
					SecretKey:  "token",
				}},
			},
			out: []corev1.Container{{
				Name:       "pr-source-creds-mz4c7",
				Image:      "override-with-pr:latest",
				WorkingDir: workspaceDir,
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
