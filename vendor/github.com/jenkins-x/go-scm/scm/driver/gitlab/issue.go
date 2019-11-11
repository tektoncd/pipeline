// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type issueService struct {
	client *wrapper
}

func (s *issueService) Search(context.Context, scm.SearchOptions) ([]*scm.SearchIssue, *scm.Response, error) {
	// TODO implemment
	return nil, nil, nil
}

func (s *issueService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) ListEvents(context.Context, string, int, scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	panic("implement me")
}

func (s *issueService) ListLabels(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	path := fmt.Sprintf("projects/%s/labels?%s", encode(repo), encodeListOptions(opts))
	out := []*label{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertLabelObjects(out), res, err
}

func (s *issueService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) Find(ctx context.Context, repo string, number int) (*scm.Issue, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d", encode(repo), number)
	out := new(issue)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertIssue(out), res, err
}

func (s *issueService) FindComment(ctx context.Context, repo string, index, id int) (*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d/notes/%d", encode(repo), index, id)
	out := new(issueComment)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertIssueComment(out), res, err
}

func (s *issueService) List(ctx context.Context, repo string, opts scm.IssueListOptions) ([]*scm.Issue, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues?%s", encode(repo), encodeIssueListOptions(opts))
	out := []*issue{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertIssueList(out), res, err
}

func (s *issueService) ListComments(ctx context.Context, repo string, index int, opts scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d/notes?%s", encode(repo), index, encodeListOptions(opts))
	out := []*issueComment{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertIssueCommentList(out), res, err
}

func (s *issueService) Create(ctx context.Context, repo string, input *scm.IssueInput) (*scm.Issue, *scm.Response, error) {
	in := url.Values{}
	in.Set("title", input.Title)
	in.Set("description", input.Body)
	path := fmt.Sprintf("api/v4/projects/%s/issues?%s", encode(repo), in.Encode())
	out := new(issue)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertIssue(out), res, err
}

func (s *issueService) CreateComment(ctx context.Context, repo string, number int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	in := url.Values{}
	in.Set("body", input.Body)
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d/notes?%s", encode(repo), number, in.Encode())
	out := new(issueComment)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertIssueComment(out), res, err
}

func (s *issueService) DeleteComment(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d/notes/%d", encode(repo), number, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *issueService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d?state_event=close", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *issueService) Lock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d?discussion_locked=true", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *issueService) Unlock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d?discussion_locked=false", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

type issue struct {
	ID     int      `json:"id"`
	Number int      `json:"iid"`
	State  string   `json:"state"`
	Title  string   `json:"title"`
	Desc   string   `json:"description"`
	Link   string   `json:"web_url"`
	Locked bool     `json:"discussion_locked"`
	Labels []string `json:"labels"`
	Author struct {
		Name     string      `json:"name"`
		Username string      `json:"username"`
		Avatar   null.String `json:"avatar_url"`
	} `json:"author"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
}

type issueComment struct {
	ID     int `json:"id"`
	Number int `json:"noteable_iid"`
	User   struct {
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
		Name      string `json:"name"`
	} `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type issueCommentInput struct {
	Body string `json:"body"`
}

// helper function to convert from the gogs issue list to
// the common issue structure.
func convertIssueList(from []*issue) []*scm.Issue {
	to := []*scm.Issue{}
	for _, v := range from {
		to = append(to, convertIssue(v))
	}
	return to
}

// helper function to convert from the gogs issue structure to
// the common issue structure.
func convertIssue(from *issue) *scm.Issue {
	return &scm.Issue{
		Number: from.Number,
		Title:  from.Title,
		Body:   from.Desc,
		Link:   from.Link,
		Labels: from.Labels,
		Locked: from.Locked,
		Closed: from.State == "closed",
		Author: scm.User{
			Name:   from.Author.Name,
			Login:  from.Author.Username,
			Avatar: from.Author.Avatar.String,
		},
		Created: from.Created,
		Updated: from.Updated,
	}
}

// helper function to convert from the gogs issue comment list
// to the common issue structure.
func convertIssueCommentList(from []*issueComment) []*scm.Comment {
	to := []*scm.Comment{}
	for _, v := range from {
		to = append(to, convertIssueComment(v))
	}
	return to
}

// helper function to convert from the gogs issue comment to
// the common issue comment structure.
func convertIssueComment(from *issueComment) *scm.Comment {
	return &scm.Comment{
		ID:   from.ID,
		Body: from.Body,
		Author: scm.User{
			Name:   from.User.Name,
			Login:  from.User.Username,
			Avatar: from.User.AvatarURL,
		},
		Created: from.CreatedAt,
		Updated: from.UpdatedAt,
	}
}
