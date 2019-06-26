package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/github"
	"github.com/mohae/deepcopy"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestGitHubParseURL(t *testing.T) {
	wantOwner := "owner"
	wantRepo := "repo"
	wantPR := 1

	for _, url := range []string{
		"https://github.com/owner/repo/pulls/1",
		"https://github.com/owner/repo/pulls/1/",
		"https://github.com/owner/repo/pulls/1/files",
		"http://github.com/owner/repo/pulls/1",
		"ssh://github.com/owner/repo/pulls/1",
		"https://example.com/owner/repo/pulls/1",
		"https://github.com/owner/repo/foo/1",
	} {
		t.Run(url, func(t *testing.T) {
			owner, repo, pr, err := parseGitHubURL(url)
			if err != nil {
				t.Fatal(err)
			}
			if owner != wantOwner {
				t.Errorf("Owner: %s, want: %s", owner, wantOwner)
			}
			if repo != wantRepo {
				t.Errorf("Repo: %s, want: %s", repo, wantRepo)
			}
			if pr != wantPR {
				t.Errorf("PR Number: %d, want: %d", pr, wantPR)
			}
		})
	}
}

func TestGitHubParseURL_errors(t *testing.T) {
	for _, url := range []string{
		"",
		"https://github.com/owner/repo",
		"https://github.com/owner/repo/pulls/foo",
	} {
		t.Run(url, func(t *testing.T) {
			if o, r, pr, err := parseGitHubURL(url); err == nil {
				t.Errorf("Expected error, got (%s, %s, %d)", o, r, pr)
			}
		})
	}
}

const (
	owner = "foo"
	repo  = "bar"
	prNum = 1
)

var (
	pr = &github.PullRequest{
		ID:      github.Int64(int64(prNum)),
		HTMLURL: github.String(fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNum)),
		Base: &github.PullRequestBranch{
			User: &github.User{
				Login: github.String(owner),
			},
			Repo: &github.Repository{
				Name:     github.String(repo),
				CloneURL: github.String(fmt.Sprintf("https://github.com/%s/%s", owner, repo)),
			},
			Ref: github.String("master"),
			SHA: github.String("1"),
		},
		Head: &github.PullRequestBranch{
			User: &github.User{
				Login: github.String("baz"),
			},
			Repo: &github.Repository{
				Name:     github.String(repo),
				CloneURL: github.String(fmt.Sprintf("https://github.com/baz/%s", repo)),
			},
			Ref: github.String("feature"),
			SHA: github.String("2"),
		},
	}
	comments []*github.IssueComment
)

func githubClient(t *testing.T, gh *FakeGitHub) (*github.Client, func()) {
	t.Helper()

	s := httptest.NewServer(gh)
	client := github.NewClient(s.Client())
	u, err := url.Parse(fmt.Sprintf("%s/", s.URL))
	if err != nil {
		t.Fatalf("error parsing HTTP test server URL: %v", err)
	}
	client.BaseURL = u

	return client, s.Close
}

func newHandler(ctx context.Context, t *testing.T) (*GitHubHandler, func()) {
	t.Helper()

	gh := NewFakeGitHub()
	client, close := githubClient(t, gh)

	// Prepopulate GitHub server with defaults to ease test setup.
	gh.AddPullRequest(pr)
	commentBody := []string{
		"hello world!",
		"a",
		"b",
	}
	comments = make([]*github.IssueComment, 0, len(commentBody))
	for _, b := range commentBody {
		c, _, err := client.Issues.CreateComment(ctx, owner, repo, prNum, &github.IssueComment{
			Body: github.String(b),
		})
		if err != nil {
			t.Fatalf("error creating comment: %v", err)
		}
		comments = append(comments, c)
	}

	h, err := NewGitHubHandler(ctx, zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())).Sugar(), pr.GetHTMLURL())
	if err != nil {
		t.Fatalf("error creating GitHubHandler: %v", err)
	}
	// Override GitHub client to point to local test server.
	h.Client = client
	return h, close
}

func TestGitHub(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	dir := os.TempDir()
	if err := h.Download(ctx, dir); err != nil {
		t.Fatal(err)
	}

	prPath := filepath.Join(dir, "pr.json")
	rawPRPath := filepath.Join(dir, "github/pr.json")

	tknComments := []*Comment{
		githubCommentToTekton(comments[0], dir),
		githubCommentToTekton(comments[1], dir),
		githubCommentToTekton(comments[2], dir),
	}

	wantPR := &PullRequest{
		Type: "github",
		ID:   int64(prNum),
		Head: &GitReference{
			Repo:   pr.GetHead().GetRepo().GetCloneURL(),
			Branch: pr.GetHead().GetRef(),
			SHA:    pr.GetHead().GetSHA(),
		},
		Base: &GitReference{
			Repo:   pr.GetBase().GetRepo().GetCloneURL(),
			Branch: pr.GetBase().GetRef(),
			SHA:    pr.GetBase().GetSHA(),
		},
		Comments: tknComments,
		Labels:   []*Label{},
		Raw:      rawPRPath,
	}

	gotPR := new(PullRequest)
	diffFile(t, prPath, wantPR, gotPR)
	diffFile(t, rawPRPath, pr, new(github.PullRequest))
	if rawPRPath != gotPR.Raw {
		t.Errorf("Raw PR path: want [%s], got [%s]", rawPRPath, gotPR.Raw)
	}
	for i, c := range tknComments {
		t.Run(fmt.Sprintf("Comment%d", c.GetID()), func(t *testing.T) {
			diffFile(t, c.GetRaw(), comments[i], new(github.IssueComment))
			if c.GetRaw() != gotPR.Comments[i].Raw {
				t.Errorf("Raw PR path: want [%s], got [%s]", c.GetRaw(), gotPR.Comments[i].Raw)
			}
		})
	}
}

func TestUpload(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	tektonPR := &PullRequest{
		Type: "github",
		ID:   int64(prNum),
		Head: &GitReference{
			Repo:   pr.GetHead().GetRepo().GetCloneURL(),
			Branch: pr.GetHead().GetRef(),
			SHA:    pr.GetHead().GetSHA(),
		},
		Base: &GitReference{
			Repo:   pr.GetBase().GetRepo().GetCloneURL(),
			Branch: pr.GetBase().GetRef(),
			SHA:    pr.GetBase().GetSHA(),
		},
		Comments: []*Comment{{
			ID:     comments[0].GetID(),
			Author: comments[0].GetUser().GetLogin(),
			Text:   comments[0].GetBody(),
		}, {
			Text: "abc123",
		}},
		Labels: []*Label{{
			Text: "tacocat",
		}},
	}
	dir := os.TempDir()
	prPath := filepath.Join(dir, "pr.json")
	f, err := os.Create(prPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(f).Encode(tektonPR); err != nil {
		t.Fatal(err)
	}

	if err := h.Upload(ctx, dir); err != nil {
		t.Fatal(err)
	}

	wantPR := deepcopy.Copy(pr).(*github.PullRequest)
	wantPR.Labels = []*github.Label{{
		Name: github.String(tektonPR.Labels[0].Text),
	}}
	gotPR, _, err := h.Client.PullRequests.Get(ctx, owner, repo, prNum)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantPR, gotPR); diff != "" {
		t.Errorf("Upload PR -want +got: %s", diff)
	}

	ghComments, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantComments := []*github.IssueComment{comments[0], &github.IssueComment{
		ID:   github.Int64(2),
		Body: github.String(tektonPR.Comments[1].Text),
	}}
	if diff := cmp.Diff(wantComments, ghComments); diff != "" {
		t.Errorf("Upload comment -want +got: %s", diff)
	}

}

func diffFile(t *testing.T, path string, want interface{}, got interface{}) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(f).Decode(got); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("PR -want +got: %s", diff)
	}
}

func TestParseGitHubURL(t *testing.T) {
	good := []string{
		fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNum),
		fmt.Sprintf("https://github.com/%s/%s/foo/%d", owner, repo, prNum),
		fmt.Sprintf("http://github.com/%s/%s/pull/%d", owner, repo, prNum),
		fmt.Sprintf("tacocat://github.com/%s/%s/pull/%d", owner, repo, prNum),
		fmt.Sprintf("https://example.com/%s/%s/pull/%d", owner, repo, prNum),
		fmt.Sprintf("https://github.com/%s/%s/pull/%d/foo", owner, repo, prNum),
		fmt.Sprintf("github.com/%s/%s/pull/%d/foo", owner, repo, prNum),
	}
	for _, u := range good {
		t.Run(u, func(t *testing.T) {
			gotOwner, gotRepo, gotPR, err := parseGitHubURL(u)
			if err != nil {
				t.Fatal(err)
			}
			if gotOwner != owner || gotRepo != repo || gotPR != prNum {
				t.Errorf("want (%s, %s, %d), got (%s, %s, %d)", owner, repo, prNum, gotOwner, gotRepo, gotPR)
			}
		})
	}

	bad := []string{
		fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		fmt.Sprintf("https://github.com/%s/%s/pulls", owner, repo),
		fmt.Sprintf("https://github.com/%s/%s/pulls/foo", owner, repo),
		"https://github.com",
	}
	for _, u := range bad {
		t.Run(u, func(t *testing.T) {
			if owner, repo, pr, err := parseGitHubURL(u); err == nil {
				t.Errorf("want error, got (%s, %s, %d)", owner, repo, pr)
			}
		})
	}
}

func TestBaseGitHubPullRequest(t *testing.T) {
	want := &PullRequest{
		Type: "github",
		ID:   1,
		Head: &GitReference{
			Repo:   "https://github.com/baz/bar",
			Branch: "feature",
			SHA:    "2",
		},
		Base: &GitReference{
			Repo:   "https://github.com/foo/bar",
			Branch: "master",
			SHA:    "1",
		},
		Labels: []*Label{{
			Text: "tacocat",
		}},
	}

	// Copy the default PR so we don't modify for other tests.
	in := deepcopy.Copy(pr).(*github.PullRequest)
	in.Labels = []*github.Label{{
		Name: github.String("tacocat"),
	}}
	got := baseGitHubPullRequest(in)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %s", diff)
	}
}

func TestReadWrite(t *testing.T) {
	path := fmt.Sprintf("%s/foo", os.TempDir())
	if err := writeJSON(path, pr); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := new(github.PullRequest)
	if err := readJSON(path, got); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if diff := cmp.Diff(pr, got); diff != "" {
		t.Errorf("-want +got: %s", diff)
	}
}

func TestCreateNewComments(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	req := []*Comment{
		{
			// Forces comment creation to fail.
			Text: ErrorKeyword,
		},
		{
			Text: "b",
		},
	}
	if err := h.createNewComments(ctx, req); err == nil {
		t.Error("expected error, got nil")
	}

	got, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	want := append(comments, &github.IssueComment{
		ID:   github.Int64(int64(len(comments) + 1)),
		Body: github.String(req[1].GetText()),
	})
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}

func TestUpdateExistingComments(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	req := map[int64]*Comment{
		// Non-existant comment. Should be ignored.
		8675309: &Comment{
			ID:   8675309,
			Text: comments[0].GetBody(),
		},
		// Comment that fails to update. Should not affect later updates.
		comments[1].GetID(): &Comment{
			ID:   comments[1].GetID(),
			Text: ErrorKeyword,
		},
		// Normal update.
		comments[2].GetID(): &Comment{
			ID:   comments[2].GetID(),
			Text: "tacocat",
		},
		// Comment 1 should be deleted.
	}
	if err := h.updateExistingComments(ctx, req); err == nil {
		t.Error("expected error, got nil")
	}

	got, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	want := []*github.IssueComment{
		comments[1],
		{
			ID:   comments[2].ID,
			Body: github.String(req[comments[2].GetID()].GetText()),
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}

func TestUploadComments(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	if err := h.uploadComments(ctx, nil); err != nil {
		t.Errorf("uploadComments(nil): %v", err)
	}

	req := []*Comment{
		// Non-existant comment. Should be ignored.
		{
			ID:   8675309,
			Text: comments[0].GetBody(),
		},
		// Comment that fails to update. Should not affect later updates.
		{
			ID:   comments[1].GetID(),
			Text: ErrorKeyword,
		},
		// Normal update.
		{
			ID:   comments[2].GetID(),
			Text: "tacocat",
		},
		// Comment 1 should be deleted.
	}
	if err := h.uploadComments(ctx, req); err == nil {
		t.Error("expected error, got nil")
	}

	got, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	want := []*github.IssueComment{
		comments[1],
		{
			ID:   comments[2].ID,
			Body: github.String(req[2].GetText()),
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}

	// Perform a no-op upload, verify it doesn't error and that it doesn't delete
	// all existing comments.
	if err := h.uploadComments(ctx, nil); err != nil {
		t.Fatalf("uploadComments: %v", err)
	}
	got, _, err = h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}

func TestUploadLabels(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	text := "tacocat"
	l := []*Label{{Text: text}}

	if err := h.uploadLabels(ctx, l); err != nil {
		t.Errorf("uploadLabels: %v", err)
	}

	pr, _, err := h.Client.PullRequests.Get(ctx, h.owner, h.repo, h.prNum)
	if err != nil {
		t.Fatalf("GetPullRequest: %v", err)
	}
	want := []*github.Label{{Name: github.String(text)}}
	if diff := cmp.Diff(want, pr.Labels); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}

	// Perform a no-op upload, verify it doesn't error and that it doesn't delete
	// all existing labels.
	if err := h.uploadLabels(ctx, nil); err != nil {
		t.Errorf("uploadLabels(nil): %v", err)
	}
	pr, _, err = h.Client.PullRequests.Get(ctx, h.owner, h.repo, h.prNum)
	if err != nil {
		t.Fatalf("GetPullRequest: %v", err)
	}
	if diff := cmp.Diff(want, pr.Labels); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}
