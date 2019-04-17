package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-github/github"
)

func TestFakeGitHubPullRequest(t *testing.T) {
	ctx := context.Background()
	gh := NewFakeGitHub()
	client, close := githubClient(t, gh)
	defer close()

	if _, resp, err := client.PullRequests.Get(ctx, owner, repo, prNum); err == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("Get PullRequest: wanted not found, got %+v, %v", resp, err)
	}
	gh.AddPullRequest(pr)

	got, resp, err := client.PullRequests.Get(ctx, owner, repo, prNum)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("Get PullRequest: wanted OK, got %+v, %v", resp, err)
	}
	if diff := cmp.Diff(pr, got); diff != "" {
		t.Errorf("Get PullRequest: -want +got: %s", diff)
	}
}

func TestFakeGitHubComments(t *testing.T) {
	ctx := context.Background()
	gh := NewFakeGitHub()
	client, close := githubClient(t, gh)
	defer close()

	if got, resp, err := client.Issues.ListComments(ctx, owner, repo, prNum, nil); err != nil || resp.StatusCode != http.StatusOK || len(got) != 0 {
		t.Fatalf("List Issues: wanted [], got %+v, %+v, %v", got, resp, err)
	}

	if _, _, err := client.Issues.CreateComment(ctx, owner, repo, prNum, comment); err != nil {
		t.Fatalf("CreateComment: %v", err)
	}

	got, resp, err := client.Issues.ListComments(ctx, owner, repo, prNum, nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("List Issues: wanted OK, got %+v, %v", resp, err)
	}
	want := []*github.IssueComment{comment}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("List Issues: -want +got: %s", diff)
	}
}

func TestFakeGitHubBadKey(t *testing.T) {
	gh := NewFakeGitHub()
	s := httptest.NewServer(gh)
	defer s.Close()

	if resp, err := http.Get(fmt.Sprintf("%s/repos/1/2/pulls/foo", s.URL)); err != nil || resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want BadRequest, got %+v, %v", resp, err)
	}
}
