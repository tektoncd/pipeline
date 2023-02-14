// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type issueService struct {
	client *wrapper
}

func (s *issueService) Search(ctx context.Context, options scm.SearchOptions) ([]*scm.SearchIssue, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) ListLabels(ctx context.Context, s2 string, i int, options *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) ListEvents(ctx context.Context, s2 string, i int, options *scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) EditComment(ctx context.Context, s2 string, i, i2 int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) Reopen(ctx context.Context, s2 string, i int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) SetMilestone(ctx context.Context, repo string, issueID, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) ClearMilestone(ctx context.Context, repo string, id int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) Find(ctx context.Context, repo string, number int) (*scm.Issue, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) FindComment(ctx context.Context, repo string, index, id int) (*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) List(ctx context.Context, repo string, opts scm.IssueListOptions) ([]*scm.Issue, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) ListComments(ctx context.Context, repo string, index int, opts *scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) Create(ctx context.Context, repo string, input *scm.IssueInput) (*scm.Issue, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) CreateComment(ctx context.Context, repo string, number int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) DeleteComment(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) Lock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) Unlock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}
