// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"fmt"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

type issueService struct {
	client *wrapper
}

func (s *issueService) Search(context.Context, scm.SearchOptions) ([]*scm.SearchIssue, *scm.Response, error) {
	// TODO implemment
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	issue, res, err := s.Find(ctx, repo, number)
	if err != nil {
		return res, errors.Wrapf(err, "couldn't lookup issue %d in repository %s", number, repo)
	}
	if issue == nil {
		return res, fmt.Errorf("couldn't find issue %d in repository %s", number, repo)
	}
	assignees := sets.NewString(logins...)
	for k := range issue.Assignees {
		assignees.Insert(issue.Assignees[k].Login)
	}

	namespace, name := scm.Split(repo)
	in := gitea.EditIssueOption{
		Title:     issue.Title,
		Assignees: assignees.List(),
	}
	_, giteaResp, err := s.client.GiteaClient.EditIssue(namespace, name, int64(number), in)
	return toSCMResponse(giteaResp), err
}

func (s *issueService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	issue, res, err := s.Find(ctx, repo, number)
	if err != nil {
		return res, errors.Wrapf(err, "couldn't lookup issue %d in repository %s", number, repo)
	}
	if issue == nil {
		return res, fmt.Errorf("couldn't find issue %d in repository %s", number, repo)
	}
	assignees := sets.NewString()
	for k := range issue.Assignees {
		assignees.Insert(issue.Assignees[k].Login)
	}
	assignees.Delete(logins...)

	namespace, name := scm.Split(repo)
	in := gitea.EditIssueOption{
		Title:     issue.Title,
		Assignees: assignees.List(),
	}
	_, giteaResp, err := s.client.GiteaClient.EditIssue(namespace, name, int64(number), in)
	return toSCMResponse(giteaResp), err
}

func (s *issueService) ListEvents(context.Context, string, int, *scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) ListLabels(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.GetIssueLabels(namespace, name, int64(number), gitea.ListLabelsOptions{ListOptions: toGiteaListOptions(opts)})
	return convertLabels(out), toSCMResponse(resp), err
}

func (s *issueService) lookupLabel(ctx context.Context, repo, lbl string) (int64, *scm.Response, error) {
	var labelID int64
	labelID = -1
	var repoLabels []*scm.Label
	var res *scm.Response
	var labels []*scm.Label
	var err error
	firstRun := false
	opts := scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (res != nil && opts.Page <= res.Page.Last) {
		labels, res, err = s.client.Repositories.ListLabels(ctx, repo, &opts)
		if err != nil {
			return labelID, res, err
		}
		firstRun = true
		repoLabels = append(repoLabels, labels...)
		opts.Page++
	}
	for _, l := range repoLabels {
		if l.Name == lbl {
			labelID = l.ID
			break
		}
	}
	return labelID, res, nil
}

func (s *issueService) AddLabel(ctx context.Context, repo string, number int, lbl string) (*scm.Response, error) {
	labelID, res, err := s.lookupLabel(ctx, repo, lbl)
	if err != nil {
		return res, err
	}
	namespace, name := scm.Split(repo)

	if labelID == -1 {
		lblInput := gitea.CreateLabelOption{
			Color:       "#00aabb",
			Description: "",
			Name:        lbl,
		}
		newLabel, giteaResp, err := s.client.GiteaClient.CreateLabel(namespace, name, lblInput)
		if err != nil {
			return toSCMResponse(giteaResp), errors.Wrapf(err, "failed to create label %s in repository %s", lbl, repo)
		}
		labelID = newLabel.ID
	}

	in := gitea.IssueLabelsOption{Labels: []int64{labelID}}
	_, giteaResp, err := s.client.GiteaClient.AddIssueLabels(namespace, name, int64(number), in)
	return toSCMResponse(giteaResp), err
}

func (s *issueService) DeleteLabel(ctx context.Context, repo string, number int, lbl string) (*scm.Response, error) {
	labelID, res, err := s.lookupLabel(ctx, repo, lbl)
	if err != nil {
		return res, err
	}
	if labelID == -1 {
		return res, nil
	}

	namespace, name := scm.Split(repo)
	giteaResp, err := s.client.GiteaClient.DeleteIssueLabel(namespace, name, int64(number), labelID)
	return toSCMResponse(giteaResp), err
}

func (s *issueService) Find(ctx context.Context, repo string, number int) (*scm.Issue, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.GetIssue(namespace, name, int64(number))
	return convertIssue(out), toSCMResponse(resp), err
}

func (s *issueService) FindComment(ctx context.Context, repo string, index, id int) (*scm.Comment, *scm.Response, error) {
	var comments []*scm.Comment
	var res *scm.Response
	var commentsPage []*scm.Comment
	var err error
	firstRun := false
	opts := &scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (res != nil && opts.Page <= res.Page.Last) {
		commentsPage, res, err = s.ListComments(ctx, repo, index, opts)
		if err != nil {
			return nil, res, err
		}
		firstRun = true
		comments = append(comments, commentsPage...)
		opts.Page++
	}
	for _, comment := range comments {
		if comment.ID == id {
			return comment, res, nil
		}
	}
	return nil, res, nil
}

func (s *issueService) List(ctx context.Context, repo string, opts scm.IssueListOptions) ([]*scm.Issue, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.ListIssueOption{
		ListOptions: gitea.ListOptions{
			Page:     opts.Page,
			PageSize: opts.Size,
		},
		Type: gitea.IssueTypeIssue,
	}
	if opts.Open && !opts.Closed {
		in.State = gitea.StateOpen
	} else if opts.Closed && !opts.Open {
		in.State = gitea.StateClosed
	}
	out, resp, err := s.client.GiteaClient.ListRepoIssues(namespace, name, in)
	return convertIssueList(out), toSCMResponse(resp), err
}

func (s *issueService) ListComments(ctx context.Context, repo string, index int, opts *scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.ListIssueComments(namespace, name, int64(index), gitea.ListIssueCommentOptions{ListOptions: toGiteaListOptions(opts)})
	return convertIssueCommentList(out), toSCMResponse(resp), err
}

func (s *issueService) Create(ctx context.Context, repo string, input *scm.IssueInput) (*scm.Issue, *scm.Response, error) {
	namespace, name := scm.Split(repo)

	in := gitea.CreateIssueOption{
		Title: input.Title,
		Body:  input.Body,
	}
	out, resp, err := s.client.GiteaClient.CreateIssue(namespace, name, in)
	return convertIssue(out), toSCMResponse(resp), err
}

func (s *issueService) CreateComment(ctx context.Context, repo string, index int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.CreateIssueCommentOption{Body: input.Body}
	out, resp, err := s.client.GiteaClient.CreateIssueComment(namespace, name, int64(index), in)
	return convertIssueComment(out), toSCMResponse(resp), err
}

func (s *issueService) DeleteComment(ctx context.Context, repo string, index, id int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	resp, err := s.client.GiteaClient.DeleteIssueComment(namespace, name, int64(id))
	return toSCMResponse(resp), err
}

func (s *issueService) EditComment(ctx context.Context, repo string, number, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.EditIssueCommentOption{Body: input.Body}
	out, resp, err := s.client.GiteaClient.EditIssueComment(namespace, name, int64(id), in)
	return convertIssueComment(out), toSCMResponse(resp), err
}

func (s *issueService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	closed := gitea.StateClosed
	in := gitea.EditIssueOption{
		State: &closed,
	}
	_, resp, err := s.client.GiteaClient.EditIssue(namespace, name, int64(number), in)
	return toSCMResponse(resp), err
}

func (s *issueService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	reopen := gitea.StateOpen
	in := gitea.EditIssueOption{
		State: &reopen,
	}
	_, resp, err := s.client.GiteaClient.EditIssue(namespace, name, int64(number), in)
	return toSCMResponse(resp), err
}

func (s *issueService) Lock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) Unlock(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) SetMilestone(ctx context.Context, repo string, issueID, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	num64 := int64(number)
	in := gitea.EditIssueOption{
		Milestone: &num64,
	}
	_, resp, err := s.client.GiteaClient.EditIssue(namespace, name, int64(issueID), in)
	return toSCMResponse(resp), err
}

func (s *issueService) ClearMilestone(ctx context.Context, repo string, id int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.EditIssueOption{}
	_, resp, err := s.client.GiteaClient.EditIssue(namespace, name, int64(id), in)
	return toSCMResponse(resp), err
}

//
// native data structure conversion
//

func convertIssueList(from []*gitea.Issue) []*scm.Issue {
	to := []*scm.Issue{}
	for _, v := range from {
		to = append(to, convertIssue(v))
	}
	return to
}

func convertIssue(from *gitea.Issue) *scm.Issue {
	return &scm.Issue{
		Number:    int(from.Index),
		Title:     from.Title,
		Body:      from.Body,
		Link:      from.URL,
		Closed:    from.State == gitea.StateClosed,
		Labels:    convertIssueLabels(from),
		Author:    *convertUser(from.Poster),
		Assignees: convertUsers(from.Assignees),
		Created:   from.Created,
		Updated:   from.Updated,
	}
}

func convertIssueCommentList(from []*gitea.Comment) []*scm.Comment {
	to := []*scm.Comment{}
	for _, v := range from {
		to = append(to, convertIssueComment(v))
	}
	return to
}

func convertIssueComment(from *gitea.Comment) *scm.Comment {
	if from == nil || from.Poster == nil {
		return nil
	}
	return &scm.Comment{
		ID:      int(from.ID),
		Body:    from.Body,
		Author:  *convertUser(from.Poster),
		Created: from.Created,
		Updated: from.Updated,
	}
}

func convertIssueLabels(from *gitea.Issue) []string {
	var labels []string
	for _, label := range from.Labels {
		labels = append(labels, label.Name)
	}
	return labels
}

func convertLabels(from []*gitea.Label) []*scm.Label {
	var labels []*scm.Label
	for _, label := range from {
		labels = append(labels, &scm.Label{
			ID:          label.ID,
			Name:        label.Name,
			Description: label.Description,
			URL:         label.URL,
			Color:       label.Color,
		})
	}
	return labels
}
