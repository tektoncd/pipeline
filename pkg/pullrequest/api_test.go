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
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/hashicorp/go-multierror"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/fake"
	"github.com/tektoncd/pipeline/test/diff"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

const (
	repo  = "foo/bar"
	prNum = 1
)

func defaultResource() *Resource {
	pr := &scm.PullRequest{
		Number: 123,
		Sha:    "sha1",
		Head: scm.PullRequestBranch{
			Ref:  "refs/heads/branch1",
			Sha:  "sha1",
			Repo: scm.Repository{Name: "repo1"},
		},
		Base: scm.PullRequestBranch{
			Ref:  "refs/heads/branch1",
			Sha:  "sha2",
			Repo: scm.Repository{Name: "repo1"},
		},
		Labels: []*scm.Label{},
	}
	r := &Resource{
		PR: pr,
		Comments: []*scm.Comment{
			{
				ID:   1,
				Body: "testing",
			},
			{
				ID:   2,
				Body: "abc123",
			},
		},
		Statuses: []*scm.Status{
			{
				Label:  "Tekton",
				State:  scm.StateSuccess,
				Desc:   "Test all the things!",
				Target: "https://tekton.dev",
			},
		},
	}
	populateManifest(r)
	return r
}

func newHandler(t *testing.T) (*Handler, *fake.Data) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())).Sugar()
	client, data := fake.NewDefault()

	r := defaultResource()
	data.PullRequests[prNum] = r.PR
	data.IssueComments[prNum] = r.Comments
	data.PullRequestComments[prNum] = r.Comments
	data.Statuses[r.PR.Sha] = r.Statuses

	return NewHandler(logger, client, repo, prNum), data
}

func TestDownload(t *testing.T) {
	ctx := context.Background()
	h, data := newHandler(t)

	dir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pr := data.PullRequests[prNum]
	want := &Resource{
		PR:       pr,
		Comments: data.IssueComments[prNum],
		Statuses: data.Statuses[pr.Sha],
	}
	populateManifest(want)

	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("Get PullRequest: %s", diff.PrintWantGot(d))
	}
}

func TestUploadFromDisk(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	dir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	r, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ToDisk(r, dir); err != nil {
		t.Fatal(err)
	}

	r, err = FromDisk(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}
}

func TestUpload_NewComment(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	c := &scm.Comment{Body: "hello world!"}
	r.Comments = append(r.Comments, c)

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c.Author = scm.User{Login: "k8s-ci-robot"}

	// Only compare comments since the resource manifest will change between
	// downloads.
	if d := cmp.Diff(r.Comments, got.Comments); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_DeleteComment(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	r.Comments = r.Comments[1:]

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(r.Comments, got.Comments); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_ManifestComment(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()

	// Create a new comment out of band of the resource. The upload should not
	// affect this.
	if _, _, err := h.client.PullRequests.CreateComment(ctx, h.repo, h.prNum, &scm.CommentInput{
		Body: "hello world!",
	}); err != nil {
		t.Fatal(err)
	}

	r.Comments = []*scm.Comment{}

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r.Comments = []*scm.Comment{{
		Body:   "hello world!",
		Author: scm.User{Login: "k8s-ci-robot"},
	}}

	if d := cmp.Diff(r.Comments, got.Comments); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_NewStatus(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	s := &scm.Status{
		Label: "CI",
		State: scm.StateFailure,
	}
	r.Statuses = append(r.Statuses, s)

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(r, got); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	r.Statuses[0].State = scm.StateCanceled

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(r, got); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_Invalid_Status(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	r.Statuses = []*scm.Status{
		{
			Label:  "Tekton",
			Desc:   "Status with empty State field",
			Target: "https://tekton.dev",
		},
		{
			State:  scm.StateSuccess,
			Desc:   "Status without label",
			Target: "https://tekton.dev",
		},
	}

	expectedErrors := []string{
		"invalid status: \"State\" is empty or has invalid value: {unknown Tekton Status with empty State field https://tekton.dev }",
		"invalid status: \"Label\" should not be empty: {success  Status without label https://tekton.dev }",
	}
	err := h.Upload(ctx, r)
	if err == nil {
		t.Fatal("expected errors, got nil")
	}
	merr, ok := err.(*multierror.Error)
	if !ok {
		t.Fatalf("expected error of type multierror, got %#v", merr)
	}
	if len(merr.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(merr.Errors))
	}
	for i, err := range merr.Errors {
		if d := cmp.Diff(expectedErrors[i], err.Error()); d != "" {
			t.Errorf("Upload status error diff %s", diff.PrintWantGot(d))
		}
	}
}

func TestUpload_NewLabel(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	r.PR.Labels = append(r.PR.Labels, &scm.Label{Name: "z"})

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(r.PR, got.PR); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_DeleteLabel(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()
	r.PR.Labels = []*scm.Label{}

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(r.PR, got.PR); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}

func TestUpload_ManifestLabel(t *testing.T) {
	ctx := context.Background()
	h, _ := newHandler(t)

	r := defaultResource()

	// Create a new label out of band of the resource. The upload should not
	// affect this.
	if _, err := h.client.PullRequests.AddLabel(ctx, h.repo, h.prNum, "z"); err != nil {
		t.Fatal(err)
	}

	if err := h.Upload(ctx, r); err != nil {
		t.Fatal(err)
	}

	got, err := h.Download(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r.PR.Labels = append(r.PR.Labels, &scm.Label{Name: "z"})

	if d := cmp.Diff(r.PR, got.PR); d != "" {
		t.Errorf(diff.PrintWantGot(d))
	}
}
