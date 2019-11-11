// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type pullService struct {
	client *wrapper
}

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d", encode(repo), number)
	out := new(pr)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) FindComment(ctx context.Context, repo string, index, id int) (*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/notes/%d", encode(repo), index, id)
	out := new(issueComment)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertIssueComment(out), res, err
}

func (s *pullService) List(ctx context.Context, repo string, opts scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests?%s", encode(repo), encodePullRequestListOptions(opts))
	out := []*pr{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertPullRequestList(out), res, err
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/changes?%s", encode(repo), number, encodeListOptions(opts))
	out := new(changes)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertChangeList(out.Changes), res, err
}

func (s *pullService) ListComments(ctx context.Context, repo string, index int, opts scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/notes?%s", encode(repo), index, encodeListOptions(opts))
	out := []*issueComment{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertIssueCommentList(out), res, err
}

func (s *pullService) ListLabels(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	mr, _, err := s.Find(ctx, repo, number)
	if err != nil {
		return nil, nil, err
	}

	return mr.Labels, nil, nil
}

func (s *pullService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	existingLabels, _, err := s.ListLabels(ctx, repo, number, scm.ListOptions{})
	if err != nil {
		return nil, err
	}

	allLabels := map[string]struct{}{}
	for _, l := range existingLabels {
		allLabels[l.Name] = struct{}{}
	}
	allLabels[label] = struct{}{}

	labelNames := []string{}
	for l := range allLabels {
		labelNames = append(labelNames, l)
	}

	return s.setLabels(ctx, repo, number, labelNames)
}

func (s *pullService) setLabels(ctx context.Context, repo string, number int, labels []string) (*scm.Response, error) {
	in := url.Values{}
	labelsStr := strings.Join(labels, ",")
	in.Set("labels", labelsStr)
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d?%s", encode(repo), number, in.Encode())

	return s.client.do(ctx, "PUT", path, nil, nil)
}

func (s *pullService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	existingLabels, _, err := s.ListLabels(ctx, repo, number, scm.ListOptions{})
	if err != nil {
		return nil, err
	}
	labels := []string{}
	for _, l := range existingLabels {
		if l.Name != label {
			labels = append(labels, l.Name)
		}
	}
	return s.setLabels(ctx, repo, number, labels)
}

func (s *pullService) CreateComment(ctx context.Context, repo string, index int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	in := url.Values{}
	in.Set("body", input.Body)
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/notes?%s", encode(repo), index, in.Encode())
	out := new(issueComment)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertIssueComment(out), res, err
}

func (s *pullService) DeleteComment(ctx context.Context, repo string, index, id int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/notes/%d", encode(repo), index, id)
	res, err := s.client.do(ctx, "DELETE", path, nil, nil)
	return res, err
}

func (s *pullService) Merge(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/merge", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d?state_event=closed", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

type pr struct {
	Number int    `json:"iid"`
	Sha    string `json:"sha"`
	Title  string `json:"title"`
	Desc   string `json:"description"`
	State  string `json:"state"`
	Link   string `json:"web_url"`
	Author struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Avatar   string `json:"avatar_url"`
	}
	SourceBranch string    `json:"source_branch"`
	TargetBranch string    `json:"target_branch"`
	Created      time.Time `json:"created_at"`
	Updated      time.Time `json:"updated_at"`
	Closed       time.Time
}

type changes struct {
	Changes []*change
}

type change struct {
	OldPath string `json:"old_path"`
	NewPath string `json:"new_path"`
	Added   bool   `json:"new_file"`
	Renamed bool   `json:"renamed_file"`
	Deleted bool   `json:"deleted_file"`
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
		Number: from.Number,
		Title:  from.Title,
		Body:   from.Desc,
		Sha:    from.Sha,
		Ref:    fmt.Sprintf("refs/merge-requests/%d/head", from.Number),
		Source: from.SourceBranch,
		Target: from.TargetBranch,
		Link:   from.Link,
		Closed: from.State != "opened",
		Merged: from.State == "merged",
		Author: scm.User{
			Name:   from.Author.Name,
			Login:  from.Author.Username,
			Avatar: from.Author.Avatar,
		},
		Created: from.Created,
		Updated: from.Updated,
	}
}

func convertChangeList(from []*change) []*scm.Change {
	to := []*scm.Change{}
	for _, v := range from {
		to = append(to, convertChange(v))
	}
	return to
}

func convertChange(from *change) *scm.Change {
	to := &scm.Change{
		Path:    from.NewPath,
		Added:   from.Added,
		Deleted: from.Deleted,
		Renamed: from.Renamed,
	}
	if to.Path == "" {
		to.Path = from.OldPath
	}
	return to
}
