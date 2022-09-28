// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm/labels"

	"github.com/jenkins-x/go-scm/scm"
)

type pullService struct {
	client *wrapper
}

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", namespace, name, number)
	out := new(pullRequest)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) FindComment(ctx context.Context, repo string, number, id int) (*scm.Comment, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d", namespace, name, number, id)
	out := new(pullRequestComment)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertPullRequestComment(out), res, err
}

func (s *pullService) List(ctx context.Context, repo string, opts *scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests", namespace, name)
	out := new(pullRequests)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertPullRequests(out), res, err
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/changes", namespace, name, number)
	out := new(diffstats)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertDiffstats(out), res, err
}

func (s *pullService) ListCommits(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Commit, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) ListLabels(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	// Get all comments, parse out labels (removing and added based off time)
	cs, res, err := s.ListComments(ctx, repo, number, opts)
	if err == nil {
		l, err := labels.ConvertLabelComments(cs)
		return l, res, err
	}
	return nil, res, err
}

func (s *pullService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	input := labels.CreateLabelAddComment(label)
	_, res, err := s.CreateComment(ctx, repo, number, input)
	return res, err
}

func (s *pullService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	input := labels.CreateLabelRemoveComment(label)
	_, res, err := s.CreateComment(ctx, repo, number, input)
	return res, err
}

func (s *pullService) ListEvents(context.Context, string, int, *scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) ListComments(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	// TODO(bradrydzewski) the challenge with comments is that we need to use
	// the activities endpoint, which returns entries that may or may not be
	// comments. This complicates how we handle counts and pagination.

	// GET /rest/api/1.0/projects/PRJ/repos/my-repo/pull-requests/1/activities

	projectName, repoName := scm.Split(repo)
	out := new(pullRequestActivities)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/activities", projectName, repoName, number)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertPullRequestActivities(out), res, err
}

func (s *pullService) Merge(ctx context.Context, repo string, number int, options *scm.PullRequestMergeOptions) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	getPath := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", namespace, name, number)
	getOut := new(pullRequest)
	res, err := s.client.do(ctx, "GET", getPath, nil, getOut)
	if err != nil {
		return res, err
	}
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/merge?version=%d", namespace, name, number, getOut.Version)
	res, err = s.client.do(ctx, "POST", path, nil, nil)
	return res, err
}

type prUpdateInput struct {
	ID          int    `json:"id"`
	Version     int    `json:"version"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

func (s *pullService) Update(ctx context.Context, repo string, number int, prInput *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	// TODO: Figure out how to handle updating the destination branch, because I can't find the syntax currently (apb)
	namespace, name := scm.Split(repo)
	getPath := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", namespace, name, number)
	getOut := new(pullRequest)
	res, err := s.client.do(ctx, "GET", getPath, nil, getOut)
	if err != nil {
		return nil, res, err
	}
	input := &prUpdateInput{
		ID:          getOut.ID,
		Version:     getOut.Version,
		Title:       prInput.Title,
		Description: prInput.Body,
	}
	out := new(pullRequest)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d", namespace, name, number)
	res, err = s.client.do(ctx, "PUT", path, input, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/decline", namespace, name, number)
	res, err := s.client.do(ctx, "POST", path, nil, nil)
	return res, err
}

func (s *pullService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/reopen", namespace, name, number)
	res, err := s.client.do(ctx, "POST", path, nil, nil)
	return res, err
}

func (s *pullService) CreateComment(ctx context.Context, repo string, number int, in *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	input := pullRequestCommentInput{Text: in.Body}
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments", namespace, name, number)
	out := new(pullRequestComment)
	res, err := s.client.do(ctx, "POST", path, &input, out)
	return convertPullRequestComment(out), res, err
}

func (s *pullService) DeleteComment(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	existingComment, res, err := s.FindComment(ctx, repo, number, id)
	if err != nil {
		if res != nil && res.Status == http.StatusNotFound {
			return res, nil
		}
		return res, err
	}
	if existingComment == nil {
		return res, nil
	}
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d?version=%d", namespace, name, number, id, existingComment.Version)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *pullService) EditComment(ctx context.Context, repo string, number, id int, in *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	input := pullRequestCommentInput{Text: in.Body}
	namespace, name := scm.Split(repo)
	existingComment, res, err := s.FindComment(ctx, repo, number, id)
	if err != nil {
		if res != nil && res.Status == http.StatusNotFound {
			return nil, res, nil
		}
		return nil, res, err
	}
	if existingComment == nil {
		return nil, res, nil
	}
	input.Version = existingComment.Version
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/comments/%d", namespace, name, number, id)
	out := new(pullRequestComment)
	res, err = s.client.do(ctx, "PUT", path, &input, out)
	return convertPullRequestComment(out), res, err
}

func (s *pullService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return s.RequestReview(ctx, repo, number, logins)
}

func (s *pullService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return s.UnrequestReview(ctx, repo, number, logins)
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests", namespace, name)

	in := &createPRInput{
		Title:       input.Title,
		Description: input.Body,
		State:       "OPEN",
		Open:        true,
		Closed:      false,
		FromRef: createPRInputRef{
			ID: fmt.Sprintf("refs/heads/%s", input.Head),
			Repository: createPRInputRepo{
				Slug:    name,
				Project: createPRInputRepoProject{Key: namespace},
			},
		},
		ToRef: createPRInputRef{
			ID: fmt.Sprintf("refs/heads/%s", input.Base),
			Repository: createPRInputRepo{
				Slug:    name,
				Project: createPRInputRepoProject{Key: namespace},
			},
		},
		Locked: false,
	}

	out := new(pullRequest)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	missing := scm.MissingUsers{
		Action: "request a PR review from",
	}

	var res *scm.Response
	var err error

	for _, l := range logins {
		input := pullRequestAssignInput{
			User: struct {
				Name string `json:"name"`
			}{
				Name: l,
			},
			Approved: false,
			Status:   "UNAPPROVED",
		}
		path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/participants/%s", namespace, name, number, url.PathEscape(l))
		res, err = s.client.do(ctx, "PUT", path, &input, nil)
		if err != nil && res != nil {
			missing.Users = append(missing.Users, l)
		} else if err != nil {
			return nil, fmt.Errorf("failed to add reviewer to PR. errmsg: %v", err)
		}
		if len(missing.Users) > 0 {
			return nil, missing
		}
	}
	return res, err
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	var res *scm.Response
	var err error

	for _, l := range logins {
		path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/pull-requests/%d/participants/%s", namespace, name, number, url.PathEscape(l))
		res, err = s.client.do(ctx, "DELETE", path, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to add reviewer to PR. errmsg: %v", err)
		}
	}
	return res, err
}

func (s *pullService) SetMilestone(ctx context.Context, repo string, prID, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) ClearMilestone(ctx context.Context, repo string, prID int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type createPRInput struct {
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	State       string           `json:"state,omitempty"`
	Open        bool             `json:"open,omitempty"`
	Closed      bool             `json:"closed,omitempty"`
	FromRef     createPRInputRef `json:"fromRef,omitempty"`
	ToRef       createPRInputRef `json:"toRef,omitempty"`
	Locked      bool             `json:"locked,omitempty"`
}

type createPRInputRef struct {
	ID         string            `json:"id"`
	Repository createPRInputRepo `json:"repository"`
}

type createPRInputRepo struct {
	Slug    string                   `json:"slug"`
	Project createPRInputRepoProject `json:"project"`
}

type createPRInputRepoProject struct {
	Key string `json:"key"`
}

type pullRequest struct {
	ID           int           `json:"id"`
	Version      int           `json:"version"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	State        string        `json:"state"`
	Open         bool          `json:"open"`
	Closed       bool          `json:"closed"`
	CreatedDate  int64         `json:"createdDate"`
	UpdatedDate  int64         `json:"updatedDate"`
	FromRef      prRepoRef     `json:"fromRef"`
	ToRef        prRepoRef     `json:"toRef"`
	Locked       bool          `json:"locked"`
	Author       prUser        `json:"author"`
	Reviewers    []prUser      `json:"reviewers"`
	Participants []interface{} `json:"participants"`
	Links        struct {
		Self []link `json:"self"`
	} `json:"links"`
}

type prRepoRef struct {
	ID           string     `json:"id"`
	DisplayID    string     `json:"displayId"`
	LatestCommit string     `json:"latestCommit"`
	Repository   repository `json:"repository"`
}

type prUser struct {
	User               user   `json:"user"`
	LastReviewedCommit string `json:"lastReviewedCommit"`
	Role               string `json:"role"`
	Approved           bool   `json:"approved"`
	Status             string `json:"status"`
}

type pullRequests struct {
	pagination
	Values []*pullRequest `json:"values"`
}

func convertPullRequests(from *pullRequests) []*scm.PullRequest {
	to := []*scm.PullRequest{}
	for _, v := range from.Values {
		to = append(to, convertPullRequest(v))
	}
	return to
}

func convertPullRequest(from *pullRequest) *scm.PullRequest {
	fork := scm.Join(
		from.FromRef.Repository.Project.Key,
		from.FromRef.Repository.Slug,
	)
	toRepo := convertRepository(&from.ToRef.Repository)
	fromRepo := convertRepository(&from.FromRef.Repository)
	return &scm.PullRequest{
		Number: from.ID,
		Title:  from.Title,
		Body:   from.Description,
		Sha:    from.FromRef.LatestCommit,
		Ref:    fmt.Sprintf("refs/pull-requests/%d/from", from.ID),
		Source: from.FromRef.DisplayID,
		Target: from.ToRef.DisplayID,
		Fork:   fork,
		Base: scm.PullRequestBranch{
			Ref:  from.ToRef.DisplayID,
			Sha:  from.ToRef.LatestCommit,
			Repo: *toRepo,
		},
		Head: scm.PullRequestBranch{
			Ref:  from.FromRef.DisplayID,
			Sha:  from.FromRef.LatestCommit,
			Repo: *fromRepo,
		},
		Link:      extractSelfLink(from.Links.Self),
		State:     strings.ToLower(from.State),
		Closed:    from.Closed,
		Merged:    from.State == "MERGED",
		Reviewers: convertReviewers(from.Reviewers),
		Created:   time.Unix(from.CreatedDate/1000, 0),
		Updated:   time.Unix(from.UpdatedDate/1000, 0),
		Author: scm.User{
			Login:  from.Author.User.Slug,
			Name:   from.Author.User.DisplayName,
			Email:  from.Author.User.EmailAddress,
			Avatar: avatarLink(from.Author.User.EmailAddress),
		},
	}
}

type pullRequestComment struct {
	Properties struct {
		RepositoryID int `json:"repositoryId"`
	} `json:"properties"`
	ID                  int                  `json:"id"`
	Version             int                  `json:"version"`
	Text                string               `json:"text"`
	Author              user                 `json:"author"`
	CreatedDate         int64                `json:"createdDate"`
	UpdatedDate         int64                `json:"updatedDate"`
	Comments            []pullRequestComment `json:"comments"`
	Tasks               []interface{}        `json:"tasks"`
	PermittedOperations struct {
		Editable  bool `json:"editable"`
		Deletable bool `json:"deletable"`
	} `json:"permittedOperations"`
}

type pullRequestCommentInput struct {
	Text    string `json:"text"`
	Version int    `json:"version"`
}

type pullRequestAssignInput struct {
	User struct {
		Name string `json:"name"`
	}
	Approved bool   `json:"approved"`
	Status   string `json:"status"`
}

type pullRequestActivities struct {
	pagination
	Values []*pullRequestActivity `json:"values"`
}

type pullRequestActivity struct {
	ID            int                 `json:"id"`
	CreatedDate   int64               `json:"createdDate"`
	User          user                `json:"user"`
	Action        string              `json:"action"`
	CommentAction string              `json:"commentAction"`
	Comment       *pullRequestComment `json:"comment"`
	CommentAnchor interface{}         `json:"commentAnchor"`
}

func convertPullRequestActivities(from *pullRequestActivities) []*scm.Comment {
	var to []*scm.Comment

	for _, v := range from.Values {
		if v.Comment != nil && v.Action == "COMMENTED" {
			to = append(to, convertPullRequestComment(v.Comment))
		}
	}
	return to
}

func convertPullRequestComment(from *pullRequestComment) *scm.Comment {
	return &scm.Comment{
		ID:      from.ID,
		Body:    from.Text,
		Version: from.Version,
		Created: time.Unix(from.CreatedDate/1000, 0),
		Updated: time.Unix(from.UpdatedDate/1000, 0),
		Author: scm.User{
			Login:  from.Author.Slug,
			Name:   from.Author.DisplayName,
			Email:  from.Author.EmailAddress,
			Avatar: avatarLink(from.Author.EmailAddress),
		},
	}
}

func convertReviewers(from []prUser) []scm.User {
	var answer []scm.User

	for k := range from {
		answer = append(answer, *convertUser(&from[k].User))
	}

	return answer
}
