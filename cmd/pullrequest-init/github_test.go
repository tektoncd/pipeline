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

const (
	owner = "foo"
	repo  = "bar"
	prNum = 1
	sha   = "tacocat"
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
	comment = &github.IssueComment{
		ID:   github.Int64(1),
		Body: github.String("hello world!"),
	}
)

func TestGitHubParseURL(t *testing.T) {
	wantOwner := "owner"
	wantRepo := "repo"
	wantPR := 1

	for _, tt := range []struct{
		url      string
		wantHost string
	}{
		{"https://github.com/owner/repo/pulls/1", "https://github.com"},
		{"https://github.com/owner/repo/pulls/1/", "https://github.com"},
		{"https://github.com/owner/repo/pulls/1/files", "https://github.com"},
		{"http://github.com/owner/repo/pulls/1", "http://github.com"},
		{"ssh://github.com/owner/repo/pulls/1", "ssh://github.com"},
		{"https://example.com/owner/repo/pulls/1", "https://example.com"},
		{"https://github.com/owner/repo/foo/1", "https://github.com"},
	} {
		t.Run(tt.url, func(t *testing.T) {
			owner, repo, host, pr, err := parseGitHubURL(tt.url)
			if err != nil {
				t.Fatal(err)
			}
			if owner != wantOwner {
				t.Errorf("Owner: %s, want: %s", owner, wantOwner)
			}
			if repo != wantRepo {
				t.Errorf("Repo: %s, want: %s", repo, wantRepo)
			}
			if host != tt.wantHost {
				t.Errorf("Host: %s, want: %s", host, tt.wantHost)
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
			if o, r, h, pr, err := parseGitHubURL(url); err == nil {
				t.Errorf("Expected error, got (%s, %s, %s, %d)", o, r, h, pr)
			}
		})
	}
}

func TestNewGitHubHandler(t *testing.T) {
	ctx := context.Background()

	for _, tt := range []struct{
		in  string
		out string
	} {
		{"https://github.com/tektoncd/pipeline/pull/1", "api.github.com"},
		{"https://github.tekton.dev/tektoncd/pipeline/pull/1", "github.tekton.dev"},
	} {
		t.Run(tt.in, func(t *testing.T) {
			h, err := NewGitHubHandler(ctx, zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())).Sugar(), tt.in)
			if err != nil {
				t.Fatalf("error creating GitHubHandler: %v", err)
			}
			if h.BaseURL.Host != tt.out {
				t.Fatalf("error unexpected BaseURL.Host: %v", h.BaseURL.Host)
			}
		})
	}
}

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

	// Automatically prepopulate GitHub server to ease test setup.
	gh.AddPullRequest(pr)
	if _, _, err := client.Issues.CreateComment(ctx, owner, repo, prNum, comment); err != nil {
		t.Fatalf("Create Comment: %v", err)
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
	rawStatusPath := filepath.Join(dir, "github/status.json")
	rawPRPath := filepath.Join(dir, "github/pr.json")
	rawCommentPath := filepath.Join(dir, "github/comments/1.json")

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
		Statuses: []*Status{},
		Comments: []*Comment{{
			ID:     comment.GetID(),
			Author: comment.GetUser().GetLogin(),
			Text:   comment.GetBody(),
			Raw:    rawCommentPath,
		}},
		Labels:    []*Label{},
		Raw:       rawPRPath,
		RawStatus: rawStatusPath,
	}

	gotPR := new(PullRequest)
	diffFile(t, prPath, wantPR, gotPR)
	diffFile(t, rawPRPath, pr, new(github.PullRequest))
	if rawPRPath != gotPR.Raw {
		t.Errorf("Raw PR path: want [%s], got [%s]", rawPRPath, gotPR.Raw)
	}
	diffFile(t, rawCommentPath, comment, new(github.IssueComment))
	if rawCommentPath != gotPR.Comments[0].Raw {
		t.Errorf("Raw PR path: want [%s], got [%s]", rawCommentPath, gotPR.Comments[0].Raw)
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
			ID:     comment.GetID(),
			Author: comment.GetUser().GetLogin(),
			Text:   comment.GetBody(),
		}, {
			Text: "abc123",
		}},
		Labels: []*Label{{
			Text: "tacocat",
		}},
		Statuses: []*Status{{
			ID:          "Tekton",
			Code:        Success,
			Description: "Test all the things!",
			URL:         "https://tekton.dev",
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
	wantComments := []*github.IssueComment{comment, &github.IssueComment{
		ID:   github.Int64(2),
		Body: github.String(tektonPR.Comments[1].Text),
	}}
	if diff := cmp.Diff(wantComments, ghComments); diff != "" {
		t.Errorf("Upload comment -want +got: %s", diff)
	}

	tektonStatus := tektonPR.Statuses[0]
	wantStatus := &github.CombinedStatus{
		TotalCount: github.Int(1),
		Statuses: []github.RepoStatus{{
			Context:     github.String(tektonStatus.ID),
			Description: github.String(tektonStatus.Description),
			State:       github.String("success"),
			TargetURL:   github.String(tektonStatus.URL),
		}},
	}
	gotStatus, resp, err := h.Client.Repositories.GetCombinedStatus(ctx, owner, repo, tektonPR.Head.SHA, nil)
	if err != nil {
		t.Fatalf("GetCombinedStatus: wanted OK, got %+v, %v", resp, err)
	}
	if diff := cmp.Diff(wantStatus, gotStatus); diff != "" {
		t.Errorf("GetCombinedStatus: -want +got: %s", diff)
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
	good := []struct{
		url string
		wantHost string
	} {
		{fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, prNum), "https://github.com"},
		{fmt.Sprintf("https://github.com/%s/%s/foo/%d", owner, repo, prNum), "https://github.com"},
		{fmt.Sprintf("http://github.com/%s/%s/pull/%d", owner, repo, prNum), "http://github.com"},
		{fmt.Sprintf("tacocat://github.com/%s/%s/pull/%d", owner, repo, prNum), "tacocat://github.com"},
		{fmt.Sprintf("https://example.com/%s/%s/pull/%d", owner, repo, prNum), "https://example.com"},
		{fmt.Sprintf("https://github.com/%s/%s/pull/%d/foo", owner, repo, prNum), "https://github.com"}, 
		{fmt.Sprintf("github.com/%s/%s/pull/%d/foo", owner, repo, prNum), "://"},
	}
	for _, u := range good {
		t.Run(u.url, func(t *testing.T) {
			gotOwner, gotRepo, gotHost, gotPR, err := parseGitHubURL(u.url)
			if err != nil {
				t.Fatal(err)
			}
			if gotOwner != owner || gotRepo != repo || gotHost != u.wantHost || gotPR != prNum {
				t.Errorf("want (%s, %s, %s, %d), got (%s, %s, %s, %d)", owner, repo, u.wantHost, prNum, gotOwner, gotRepo, gotHost, gotPR)
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
			if owner, repo, host, pr, err := parseGitHubURL(u); err == nil {
				t.Errorf("want error, got (%s, %s, %s, %d)", owner, repo, host, pr)
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

	comments := []*Comment{
		{
			// Forces comment creation to fail.
			Text: ErrorKeyword,
		},
		{
			Text: "b",
		},
	}
	if err := h.createNewComments(ctx, comments); err == nil {
		t.Error("expected error, got nil")
	}

	got, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	want := []*github.IssueComment{
		comment,
		{
			ID:   github.Int64(2),
			Body: github.String(comments[1].Text),
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}

func TestUpdateExistingComments(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	comment2, _, err := h.Client.Issues.CreateComment(ctx, owner, repo, prNum, &github.IssueComment{
		Body: github.String("a"),
	})
	if err != nil {
		t.Fatalf("error creating comment: %v", err)
	}
	comment3, _, err := h.Client.Issues.CreateComment(ctx, owner, repo, prNum, &github.IssueComment{
		Body: github.String("b"),
	})
	if err != nil {
		t.Fatalf("error creating comment: %v", err)
	}
	comment3.Body = github.String("tacocat")

	comments := map[int64]*Comment{
		// Non-existant comment. Should be ignored.
		8675309: &Comment{
			ID:   8675309,
			Text: comment.GetBody(),
		},
		// Comment that fails to update. Should not affect later updates.
		comment2.GetID(): &Comment{
			ID:   comment2.GetID(),
			Text: ErrorKeyword,
		},
		// Normal update.
		comment3.GetID(): &Comment{
			ID:   comment3.GetID(),
			Text: comment3.GetBody(),
		},
		// Comment 1 should be deleted.
	}
	if err := h.updateExistingComments(ctx, comments); err == nil {
		t.Error("expected error, got nil")
	}

	got, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	want := []*github.IssueComment{comment2, comment3}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}

func TestUploadComments(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	comment2, _, err := h.Client.Issues.CreateComment(ctx, owner, repo, prNum, &github.IssueComment{
		Body: github.String("a"),
	})
	if err != nil {
		t.Fatalf("error creating comment: %v", err)
	}
	comment3, _, err := h.Client.Issues.CreateComment(ctx, owner, repo, prNum, &github.IssueComment{
		Body: github.String("b"),
	})
	if err != nil {
		t.Fatalf("error creating comment: %v", err)
	}
	comment3.Body = github.String("tacocat")

	comments := []*Comment{
		// Non-existant comment. Should be ignored.
		{
			ID:   8675309,
			Text: comment.GetBody(),
		},
		// Comment that fails to update. Should not affect later updates.
		{
			ID:   comment2.GetID(),
			Text: ErrorKeyword,
		},
		// Normal update.
		{
			ID:   comment3.GetID(),
			Text: comment3.GetBody(),
		},
		// Comment 1 should be deleted.
	}
	if err := h.uploadComments(ctx, comments); err == nil {
		t.Error("expected error, got nil")
	}

	got, _, err := h.Client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	want := []*github.IssueComment{comment2, comment3}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("-want +got: %v", diff)
	}
}

func TestGetStatuses(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	rs := []*github.RepoStatus{
		{
			Context: github.String("a"),
			State:   github.String("success"),
		},
		{
			Context: github.String("b"),
			State:   github.String("failure"),
		},
	}
	for _, s := range rs {
		if _, _, err := h.Client.Repositories.CreateStatus(ctx, owner, repo, sha, s); err != nil {
			t.Fatalf("CreateStatus: %v", err)
		}
	}

	testCases := []struct {
		sha  string
		want []*Status
	}{
		{
			sha: sha,
			want: []*Status{
				{
					ID:   "a",
					Code: Success,
				},
				{
					ID:   "b",
					Code: Failure,
				},
			},
		},
		{
			sha:  "deadbeef",
			want: []*Status{},
		},
	}

	dir := os.TempDir()
	for _, tc := range testCases {
		t.Run(tc.sha, func(t *testing.T) {
			path := filepath.Join(dir, tc.sha)
			s, err := h.getStatuses(ctx, tc.sha, path)
			if err != nil {
				t.Fatalf("getStatuses: %v", err)
			}
			if diff := cmp.Diff(tc.want, s); diff != "" {
				t.Errorf("-want +got: %s", diff)
			}
		})
	}
}

func TestGetStatusesError(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	s := &github.RepoStatus{
		Context: github.String("a"),
		// Not a valid GitHub status state
		State: github.String("unknown"),
	}
	if _, _, err := h.Client.Repositories.CreateStatus(ctx, owner, repo, sha, s); err != nil {
		t.Fatalf("CreateStatus: %v", err)
	}

	got, err := h.getStatuses(ctx, sha, "")
	if err == nil {
		t.Fatalf("getStatuses: want err, got %+v", got)
	}
}

func TestUploadStatuses(t *testing.T) {
	ctx := context.Background()
	h, close := newHandler(ctx, t)
	defer close()

	s := []*Status{
		// Invaid status code. Should fail.
		{
			Code: StatusCode(""),
		},
		{
			Code: Failure,
		},
		// Should overwrite previous status.
		{
			Code: "success",
		},
	}

	if err := h.uploadStatuses(ctx, sha, s); err == nil {
		t.Error("uploadStatus want error, got nil")
	}
	want := &github.CombinedStatus{
		TotalCount: github.Int(1),
		Statuses: []github.RepoStatus{{
			Context:     github.String(""),
			State:       github.String("success"),
			Description: github.String(""),
			TargetURL:   github.String(""),
		}},
	}
	got, resp, err := h.Client.Repositories.GetCombinedStatus(ctx, owner, repo, sha, nil)
	if err != nil {
		t.Fatalf("GetCombinedStatus: wanted OK, got %+v, %v", resp, err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("GetCombinedStatus: -want +got: %s", diff)
	}
}
