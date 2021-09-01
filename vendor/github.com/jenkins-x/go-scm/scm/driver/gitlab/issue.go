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
	issue, _, err := s.Find(ctx, repo, number)
	if err != nil {
		return nil, err
	}

	allAssignees := map[int]struct{}{}
	for _, assignee := range issue.Assignees {
		allAssignees[assignee.ID] = struct{}{}
	}
	for _, l := range logins {
		u, _, err := s.client.Users.FindLogin(ctx, l)
		if err != nil {
			return nil, err
		}
		allAssignees[u.ID] = struct{}{}
	}

	var assigneeIDs []int
	for i := range allAssignees {
		assigneeIDs = append(assigneeIDs, i)
	}

	return s.setAssignees(ctx, repo, number, assigneeIDs)
}

func (s *issueService) setAssignees(ctx context.Context, repo string, number int, ids []int) (*scm.Response, error) {
	in := &updateIssueOptions{
		AssigneeIDs: ids,
	}
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d", encode(repo), number)

	return s.client.do(ctx, "PUT", path, in, nil)
}

func (s *issueService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	issue, _, err := s.Find(ctx, repo, number)
	if err != nil {
		return nil, err
	}
	var assignees []int
	for _, assignee := range issue.Assignees {
		shouldKeep := true
		for _, l := range logins {
			if assignee.Login == l {
				shouldKeep = false
			}
		}
		if shouldKeep {
			assignees = append(assignees, assignee.ID)
		}
	}

	return s.setAssignees(ctx, repo, number, assignees)
}

func (s *issueService) ListEvents(ctx context.Context, repo string, index int, opts scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d/resource_label_events?%s", encode(repo), index, encodeListOptions(opts))
	out := []*labelEvent{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertLabelEvents(out), res, err
}

func (s *issueService) ListLabels(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	issue, issueResp, err := s.Find(ctx, repo, number)
	if err != nil {
		return nil, issueResp, err
	}
	var labels []*scm.Label
	for _, l := range issue.Labels {
		labels = append(labels, &scm.Label{
			Name: l,
		})
	}
	return labels, issueResp, err
}

func (s *issueService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
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

func (s *issueService) setLabels(ctx context.Context, repo string, number int, labels []string) (*scm.Response, error) {
	in := url.Values{}
	labelsStr := strings.Join(labels, ",")
	in.Set("labels", labelsStr)
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d?%s", encode(repo), number, in.Encode())

	return s.client.do(ctx, "PUT", path, nil, nil)
}

func (s *issueService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
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

func (s *issueService) EditComment(ctx context.Context, repo string, number int, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	in := &updateNoteOptions{Body: input.Body}
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d/notes/%d", encode(repo), number, id)
	out := new(issueComment)
	res, err := s.client.do(ctx, "PUT", path, in, out)
	return convertIssueComment(out), res, err
}

func (s *issueService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d?state_event=close", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *issueService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d?state_event=reopen", encode(repo), number)
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

func (s *issueService) SetMilestone(ctx context.Context, repo string, issueID int, number int) (*scm.Response, error) {
	in := &updateIssueOptions{
		MilestoneID: &number,
	}
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d", encode(repo), issueID)

	return s.client.do(ctx, "PUT", path, in, nil)
}

func (s *issueService) ClearMilestone(ctx context.Context, repo string, id int) (*scm.Response, error) {
	zeroVal := 0
	in := &updateIssueOptions{
		MilestoneID: &zeroVal,
	}
	path := fmt.Sprintf("api/v4/projects/%s/issues/%d", encode(repo), id)

	return s.client.do(ctx, "PUT", path, in, nil)
}

type updateIssueOptions struct {
	Title            *string    `json:"title,omitempty"`
	Description      *string    `json:"description,omitempty"`
	Confidential     *bool      `json:"confidential,omitempty"`
	AssigneeIDs      []int      `json:"assignee_ids,omitempty"`
	MilestoneID      *int       `json:"milestone_id,omitempty"`
	Labels           []string   `json:"labels,omitempty"`
	StateEvent       *string    `json:"state_event,omitempty"`
	UpdatedAt        *time.Time `json:"updated_at,omitempty"`
	Weight           *int       `json:"weight,omitempty"`
	DiscussionLocked *bool      `json:"discussion_locked,omitempty"`
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
	Assignee  *issueAssignee   `json:"assignee"`
	Assignees []*issueAssignee `json:"assignees"`
	Created   time.Time        `json:"created_at"`
	Updated   time.Time        `json:"updated_at"`
}

type issueAssignee struct {
	ID        int    `json:"id"`
	State     string `json:"state"`
	WebURL    string `json:"web_url"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Username  string `json:"username"`
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

// helper function to convert from the gitlab issue list to
// the common issue structure.
func convertIssueList(from []*issue) []*scm.Issue {
	to := []*scm.Issue{}
	for _, v := range from {
		to = append(to, convertIssue(v))
	}
	return to
}

// helper function to convert from the gitlab issue structure to
// the common issue structure.
func convertIssue(from *issue) *scm.Issue {
	return &scm.Issue{
		Number: from.Number,
		Title:  from.Title,
		Body:   from.Desc,
		State:  gitlabStateToSCMState(from.State),
		Link:   from.Link,
		Labels: from.Labels,
		Locked: from.Locked,
		Closed: from.State == "closed",
		Author: scm.User{
			Name:   from.Author.Name,
			Login:  from.Author.Username,
			Avatar: from.Author.Avatar.String,
		},
		Assignees: convertIssueAssignees(from.Assignee, from.Assignees),
		Created:   from.Created,
		Updated:   from.Updated,
	}
}

// helper function to convert from the gitlab issue assignee(s) to the common user structure.
func convertIssueAssignees(from *issueAssignee, fromList []*issueAssignee) []scm.User {
	users := make(map[int]scm.User)
	if from != nil {
		users[from.ID] = convertSingleIssueAssignee(from)
	}
	for _, a := range fromList {
		if _, exists := users[a.ID]; !exists {
			users[a.ID] = convertSingleIssueAssignee(a)
		}
	}

	var userList []scm.User
	for _, u := range users {
		userList = append(userList, u)
	}
	return userList
}

// helper function to convert an individual gitlab issue assignee to a common user.
func convertSingleIssueAssignee(from *issueAssignee) scm.User {
	return scm.User{
		ID:     from.ID,
		Login:  from.Username,
		Name:   from.Name,
		Avatar: from.AvatarURL,
		Link:   from.WebURL,
	}
}

// helper function to convert from the gitlab issue comment list
// to the common issue structure.
func convertIssueCommentList(from []*issueComment) []*scm.Comment {
	to := []*scm.Comment{}
	for _, v := range from {
		to = append(to, convertIssueComment(v))
	}
	return to
}

// helper function to convert from the gitlab issue comment to
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
