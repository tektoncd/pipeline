// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type pullService struct {
	*issueService
}

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repo, number)
	out := new(pr)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) List(ctx context.Context, repo string, opts scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls?%s", repo, encodePullRequestListOptions(opts))
	out := []*pr{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertPullRequestList(out), res, err
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/files?%s", repo, number, encodeListOptions(opts))
	out := []*file{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertChangeList(out), res, err
}

func (s *pullService) Merge(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/merge", repo, number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repo, number)
	data := map[string]string{"state": "closed"}
	res, err := s.client.do(ctx, "PATCH", path, &data, nil)
	return res, err
}

type prBranch struct {
	Ref  string     `json:"ref"`
	Sha  string     `json:"sha"`
	User user       `json:"user"`
	Repo repository `json:"repo"`
}
type milestone struct {
	Number      int    `json:"number"`
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"Description"`
	Link        string `json:"html_url"`
	State       string `json:"state"`
}

type pr struct {
	Number             int         `json:"number"`
	State              string      `json:"state"`
	Title              string      `json:"title"`
	Body               string      `json:"body"`
	Labels             []*label    `json:"labels"`
	DiffURL            string      `json:"diff_url"`
	User               user        `json:"user"`
	RequestedReviewers []user      `json:"requested_reviewers"`
	Assignees          []user      `json:"assignees"`
	Head               prBranch    `json:"head"`
	Base               prBranch    `json:"base"`
	Draft              bool        `json:"draft"`
	Merged             bool        `json:"merged"`
	Mergeable          bool        `json:"mergeable"`
	MergeableState     string      `json:"mergeable_state"`
	Rebaseable         bool        `json:"rebaseable"`
	MergeSha           string      `json:"merge_commit_sha"`
	Milestone          milestone   `json:"milestone"`
	MergedAt           null.String `json:"merged_at"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
}

type file struct {
	Sha              string `json:"sha"`
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	Patch            string `json:"patch"`
	BlobURL          string `json:"blob_url"`
	PreviousFilename string `json:"previous_filename"`
}

func convertPullRequestList(from []*pr) []*scm.PullRequest {
	to := []*scm.PullRequest{}
	for _, v := range from {
		to = append(to, convertPullRequest(v))
	}
	return to
}

func convertPullRequest(from *pr) *scm.PullRequest {
	return &scm.PullRequest{
		Number:         from.Number,
		Title:          from.Title,
		Body:           from.Body,
		Labels:         convertLabelObjects(from.Labels),
		Sha:            from.Head.Sha,
		Ref:            fmt.Sprintf("refs/pull/%d/head", from.Number),
		State:          from.State,
		Source:         from.Head.Ref,
		Target:         from.Base.Ref,
		Fork:           from.Head.Repo.FullName,
		Base:           *convertPullRequestBranch(&from.Base),
		Head:           *convertPullRequestBranch(&from.Head),
		Link:           from.DiffURL,
		Closed:         from.State != "open",
		Draft:          from.Draft,
		MergeSha:       from.MergeSha,
		Merged:         from.Merged,
		Mergeable:      from.Mergeable,
		MergeableState: scm.ToMergeableState(from.MergeableState),
		Rebaseable:     from.Rebaseable,
		Author:         *convertUser(&from.User),
		Assignees:      convertUsers(from.Assignees),
		Created:        from.CreatedAt,
		Updated:        from.UpdatedAt,
	}
}

func convertPullRequestBranch(src *prBranch) *scm.PullRequestBranch {
	return &scm.PullRequestBranch{
		Ref:  src.Ref,
		Sha:  src.Sha,
		Repo: *convertRepository(&src.Repo),
	}
}

func convertUsers(users []user) []scm.User {
	answer := []scm.User{}
	for _, u := range users {
		user := convertUser(&u)
		if user.Login != "" {
			answer = append(answer, *user)
		}
	}
	if len(answer) == 0 {
		return nil
	}
	return answer
}

func convertChangeList(from []*file) []*scm.Change {
	to := []*scm.Change{}
	for _, v := range from {
		to = append(to, convertChange(v))
	}
	return to
}

func convertChange(from *file) *scm.Change {
	return &scm.Change{
		Path:      from.Filename,
		Added:     from.Status == "added",
		Deleted:   from.Status == "deleted",
		Renamed:   from.Status == "moved",
		Additions: from.Additions,
		Deletions: from.Deletions,
		Changes:   from.Changes,
		BlobURL:   from.BlobURL,
		Sha:       from.Sha,
	}
}
