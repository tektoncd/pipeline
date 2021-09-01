// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type issueService struct {
	client *wrapper
}

type searchIssue struct {
	issue
	Score             float32 `json:"score"`
	AuthorAssociation string  `json:"author_association"`
	URL               string  `json:"url"`
	RepositoryURL     string  `json:"repository_url"`
	LabelsURL         string  `json:"labels_url"`
	CommentsURL       string  `json:"comments_url"`
	EventsURL         string  `json:"events_url"`
	Comments          int     `json:"comments"`
}

type searchResults struct {
	TotalCount        int            `json:"total_count"`
	IncompleteResults bool           `json:"incomplete_results"`
	Items             []*searchIssue `json:"items"`
}

func (s *issueService) Search(ctx context.Context, opts scm.SearchOptions) ([]*scm.SearchIssue, *scm.Response, error) {
	suffix := encodeIssueSearchOptions(opts)
	query := opts.QueryArgument()
	if suffix != "" {
		query += "&" + suffix
	}
	path := fmt.Sprintf("/search/issues?q=%s", query)
	out := searchResults{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertSearchIssueList(out.Items), res, err
}

// AssignIssue adds logins to org/repo#number, returning an error if any login is missing after making the call.
//
// See https://developer.github.com/v3/issues/assignees/#add-assignees-to-an-issue
func (s *issueService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/assignees", repo, number)
	in := map[string][]string{"assignees": logins}
	out := new(issue)
	res, err := s.client.do(ctx, "POST", path, in, out)

	assigned := make(map[string]bool)
	if err != nil {
		return res, err
	}
	for _, assignee := range out.Assignees {
		assigned[NormLogin(assignee.Login)] = true
	}
	missing := scm.MissingUsers{Action: "assign"}
	for _, login := range logins {
		if !assigned[NormLogin(login)] {
			missing.Users = append(missing.Users, login)
		}
	}
	if len(missing.Users) > 0 {
		return res, missing
	}
	return res, nil
}

// UnassignIssue removes logins from org/repo#number, returns an error if any login remains assigned.
//
// See https://developer.github.com/v3/issues/assignees/#remove-assignees-from-an-issue
func (s *issueService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/assignees", repo, number)
	in := map[string][]string{"assignees": logins}
	out := new(issue)
	res, err := s.client.do(ctx, http.MethodDelete, path, in, out)

	assigned := make(map[string]bool)
	if err != nil {
		return res, err
	}
	for _, assignee := range out.Assignees {
		assigned[NormLogin(assignee.Login)] = true
	}
	extra := scm.ExtraUsers{Action: "unassign"}
	for _, login := range logins {
		if assigned[NormLogin(login)] {
			extra.Users = append(extra.Users, login)
		}
	}
	if len(extra.Users) > 0 {
		return res, extra
	}
	return res, nil
}

func (s *issueService) Find(ctx context.Context, repo string, number int) (*scm.Issue, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d", repo, number)
	out := new(issue)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertIssue(out), res, err
}

func (s *issueService) FindComment(ctx context.Context, repo string, index, id int) (*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/comments/%d", repo, id)
	out := new(issueComment)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertIssueComment(out), res, err
}

func (s *issueService) List(ctx context.Context, repo string, opts scm.IssueListOptions) ([]*scm.Issue, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues?%s", repo, encodeIssueListOptions(opts))
	out := []*issue{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertIssueList(out), res, err
}

func (s *issueService) ListComments(ctx context.Context, repo string, index int, opts scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/comments?%s", repo, index, encodeListOptions(opts))
	out := []*issueComment{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertIssueCommentList(out), res, err
}

func (s *issueService) Create(ctx context.Context, repo string, input *scm.IssueInput) (*scm.Issue, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues", repo)
	in := &issueInput{
		Title: input.Title,
		Body:  input.Body,
	}
	out := new(issue)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertIssue(out), res, err
}

func (s *issueService) ListLabels(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/labels?%s", repo, number, encodeListOptions(opts))
	out := []*label{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertLabelObjects(out), res, err
}

func (s *issueService) ListEvents(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/events?%s", repo, number, encodeListOptions(opts))
	out := []*listedIssueEvent{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertListedIssueEvents(out), res, err
}

func (s *issueService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/labels", repo, number)
	in := []string{label}
	res, err := s.client.do(ctx, "POST", path, in, nil)
	return res, err
}

func (s *issueService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/labels/%s", repo, number, label)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *issueService) CreateComment(ctx context.Context, repo string, number int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/comments", repo, number)
	in := &issueCommentInput{
		Body: input.Body,
	}
	out := new(issueComment)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertIssueComment(out), res, err
}

func (s *issueService) DeleteComment(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/comments/%d", repo, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *issueService) EditComment(ctx context.Context, repo string, number int, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/comments/%d", repo, id)
	in := &issueCommentInput{
		Body: input.Body,
	}
	out := new(issueComment)
	res, err := s.client.do(ctx, "PATCH", path, in, out)
	return convertIssueComment(out), res, err
}

func (s *issueService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d", repo, number)
	data := map[string]string{"state": "closed"}
	out := new(issue)
	res, err := s.client.do(ctx, "PATCH", path, &data, out)
	return res, err
}

func (s *issueService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d", repo, number)
	data := map[string]string{"state": "open"}
	out := new(issue)
	res, err := s.client.do(ctx, "PATCH", path, &data, out)
	return res, err
}

func (s *issueService) Lock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/lock", repo, number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *issueService) Unlock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d/lock", repo, number)
	res, err := s.client.do(ctx, "DELETE", path, nil, nil)
	return res, err
}

func (s *issueService) SetMilestone(ctx context.Context, repo string, issueID int, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d", repo, issueID)
	in := &struct {
		Milestone int `json:"milestone"`
	}{
		Milestone: number,
	}
	res, err := s.client.do(ctx, "PATCH", path, in, nil)
	return res, err
}

func (s *issueService) ClearMilestone(ctx context.Context, repo string, id int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/issues/%d", repo, id)
	in := &struct {
		Milestone interface{} `json:"milestone"`
	}{}
	res, err := s.client.do(ctx, "PATCH", path, in, nil)
	return res, err
}

type issue struct {
	ID      int    `json:"id"`
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
	State   string `json:"state"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	User    struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	ClosedBy *struct {
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"closed_by"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []user    `json:"assignees"`
	Locked    bool      `json:"locked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// This will be non-nil if it is a pull request.
	PullRequest *struct{} `json:"pull_request,omitempty"`
}

type issueInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type issueComment struct {
	ID      int    `json:"id"`
	HTMLURL string `json:"html_url"`
	User    struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type issueCommentInput struct {
	Body string `json:"body"`
}

// listedIssueEvent represents an issue event from the events API (not from a webhook payload).
// https://developer.github.com/v3/issues/events/
type listedIssueEvent struct {
	Event   string    `json:"event"` // This is the same as IssueEvent.Action.
	Actor   user      `json:"actor"`
	Label   label     `json:"label"`
	Created time.Time `json:"created_at"`
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

// helper function to convert from the gogs issue list to
// the common issue structure.
func convertSearchIssueList(from []*searchIssue) []*scm.SearchIssue {
	to := []*scm.SearchIssue{}
	for _, v := range from {
		issue := convertIssue(&v.issue)
		searchIssue := &scm.SearchIssue{
			Issue: *issue,
		}
		populateRepositoryFromURL(&searchIssue.Repository, v.RepositoryURL)
		to = append(to, searchIssue)
	}
	return to
}

func populateRepositoryFromURL(repo *scm.Repository, u string) {
	if u == "" {
		return
	}
	paths := strings.Split(strings.TrimSuffix(u, "/"), "/")
	l := len(paths)
	if l > 0 {
		repo.Name = paths[l-1]
	}
	if l > 1 {
		repo.Namespace = paths[l-2]
	}
}

// helper function to convert from the gogs issue structure to
// the common issue structure.
func convertIssue(from *issue) *scm.Issue {
	var closedBy *scm.User
	if from.ClosedBy != nil {
		closedBy = &scm.User{
			Login:  from.ClosedBy.Login,
			Avatar: from.ClosedBy.AvatarURL,
		}
	}
	return &scm.Issue{
		Number: from.Number,
		Title:  from.Title,
		Body:   from.Body,
		Link:   from.HTMLURL,
		Labels: convertLabels(from),
		Locked: from.Locked,
		State:  from.State,
		Closed: from.State == "closed",
		Author: scm.User{
			Login:  from.User.Login,
			Avatar: from.User.AvatarURL,
		},
		ClosedBy:    closedBy,
		Assignees:   convertUsers(from.Assignees),
		PullRequest: from.PullRequest != nil,
		Created:     from.CreatedAt,
		Updated:     from.UpdatedAt,
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
			Login:  from.User.Login,
			Avatar: from.User.AvatarURL,
		},
		Link:    from.HTMLURL,
		Created: from.CreatedAt,
		Updated: from.UpdatedAt,
	}
}

func convertLabels(from *issue) []string {
	var labels []string
	for _, label := range from.Labels {
		labels = append(labels, label.Name)
	}
	return labels
}

func convertLabelObjects(from []*label) []*scm.Label {
	var labels []*scm.Label
	for _, label := range from {
		labels = append(labels, &scm.Label{
			Name:        label.Name,
			Description: label.Description,
			URL:         label.URL,
			Color:       label.Color,
		})
	}
	return labels
}

func convertListedIssueEvents(src []*listedIssueEvent) []*scm.ListedIssueEvent {
	var answer []*scm.ListedIssueEvent
	for _, from := range src {
		answer = append(answer, convertListedIssueEvent(from))
	}
	return answer
}

func convertListedIssueEvent(from *listedIssueEvent) *scm.ListedIssueEvent {
	return &scm.ListedIssueEvent{
		Event:   from.Event,
		Actor:   *convertUser(&from.Actor),
		Label:   convertLabel(from.Label),
		Created: from.Created,
	}
}
