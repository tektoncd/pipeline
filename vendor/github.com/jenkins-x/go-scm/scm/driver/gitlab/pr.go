// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
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

func (s *pullService) ListEvents(ctx context.Context, repo string, index int, opts scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/resource_label_events?%s", encode(repo), index, encodeListOptions(opts))
	out := []*labelEvent{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertLabelEvents(out), res, err
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

func (s *pullService) EditComment(ctx context.Context, repo string, number int, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	in := &updateNoteOptions{Body: input.Body}
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/notes/%d", encode(repo), number, id)
	out := new(issueComment)
	res, err := s.client.do(ctx, "PUT", path, in, out)
	return convertIssueComment(out), res, err
}

func (s *pullService) Merge(ctx context.Context, repo string, number int, options *scm.PullRequestMergeOptions) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d/merge", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d?state_event=closed", encode(repo), number)
	res, err := s.client.do(ctx, "PUT", path, nil, nil)
	return res, err
}

func (s *pullService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	pr, _, err := s.Find(ctx, repo, number)
	if err != nil {
		return nil, err
	}

	allAssignees := map[int]struct{}{}
	for _, assignee := range pr.Assignees {
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

func (s *pullService) setAssignees(ctx context.Context, repo string, number int, ids []int) (*scm.Response, error) {
	if len(ids) == 0 {
		ids = append(ids, 0)
	}
	in := &updateMergeRequestOptions{
		AssigneeIDs: ids,
	}
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d", encode(repo), number)

	return s.client.do(ctx, "PUT", path, in, nil)
}

func (s *pullService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	pr, _, err := s.Find(ctx, repo, number)
	if err != nil {
		return nil, err
	}
	var assignees []int
	for _, assignee := range pr.Assignees {
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

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return s.AssignIssue(ctx, repo, number, logins)
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return s.UnassignIssue(ctx, repo, number, logins)
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests", encode(repo))
	in := &prInput{
		Title:        input.Title,
		SourceBranch: input.Head,
		TargetBranch: input.Base,
		Description:  input.Body,
	}

	out := new(pr)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertPullRequest(out), res, err
}

type updateMergeRequestOptions struct {
	Title              *string `json:"title,omitempty"`
	Description        *string `json:"description,omitempty"`
	TargetBranch       *string `json:"target_branch,omitempty"`
	AssigneeID         *int    `json:"assignee_id,omitempty"`
	AssigneeIDs        []int   `json:"assignee_ids,omitempty"`
	Labels             *string `json:"labels,omitempty"`
	MilestoneID        *int    `json:"milestone_id,omitempty"`
	StateEvent         *string `json:"state_event,omitempty"`
	RemoveSourceBranch *bool   `json:"remove_source_branch,omitempty"`
	Squash             *bool   `json:"squash,omitempty"`
	DiscussionLocked   *bool   `json:"discussion_locked,omitempty"`
	AllowCollaboration *bool   `json:"allow_collaboration,omitempty"`
}

func (s *pullService) updateMergeRequestField(ctx context.Context, repo string, number int, input *updateMergeRequestOptions) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/merge_requests/%d", encode(repo), number)

	out := new(pr)
	res, err := s.client.do(ctx, "PUT", path, input, out)
	return convertPullRequest(out), res, err
}

type pr struct {
	Number          int       `json:"iid"`
	Sha             string    `json:"sha"`
	Title           string    `json:"title"`
	Desc            string    `json:"description"`
	State           string    `json:"state"`
	SourceProjectID int       `json:"source_project_id"`
	TargetProjectID int       `json:"target_project_id"`
	Labels          []*string `json:"labels"`
	Link            string    `json:"web_url"`
	WIP             bool      `json:"work_in_progress"`
	Author          user      `json:"author"`
	MergeStatus     string    `json:"merge_status"`
	SourceBranch    string    `json:"source_branch"`
	TargetBranch    string    `json:"target_branch"`
	Created         time.Time `json:"created_at"`
	Updated         time.Time `json:"updated_at"`
	Closed          time.Time
	DiffRefs        struct {
		BaseSHA string `json:"base_sha"`
		HeadSHA string `json:"head_sha"`
	} `json:"diff_refs"`
	Assignee  *user   `json:"assignee"`
	Assignees []*user `json:"assignees"`
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

type prInput struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

func convertPullRequestList(from []*pr) []*scm.PullRequest {
	to := []*scm.PullRequest{}
	for _, v := range from {
		to = append(to, convertPullRequest(v))
	}
	return to
}

func convertPullRequest(from *pr) *scm.PullRequest {
	// Diff refs only seem to be populated in more recent merge requests. Default
	// to from.Sha for compatibility / consistency, but fallback to HeadSHA if
	// it's not populated.
	headSHA := from.Sha
	if headSHA == "" && from.DiffRefs.HeadSHA != "" {
		headSHA = from.DiffRefs.HeadSHA
	}
	var assignees []scm.User
	if from.Assignee != nil {
		assignees = append(assignees, *convertUser(from.Assignee))
	}
	for _, a := range from.Assignees {
		assignees = append(assignees, *convertUser(a))
	}
	return &scm.PullRequest{
		Number:         from.Number,
		Title:          from.Title,
		Body:           from.Desc,
		State:          gitlabStateToSCMState(from.State),
		Labels:         convertPullRequestLabels(from.Labels),
		Sha:            from.Sha,
		Ref:            fmt.Sprintf("refs/merge-requests/%d/head", from.Number),
		Source:         from.SourceBranch,
		Target:         from.TargetBranch,
		Link:           from.Link,
		Draft:          from.WIP,
		Closed:         from.State != "opened",
		Merged:         from.State == "merged",
		Mergeable:      scm.ToMergeableState(from.MergeStatus) == scm.MergeableStateMergeable,
		MergeableState: scm.ToMergeableState(from.MergeStatus),
		Author:         *convertUser(&from.Author),
		Assignees:      assignees,
		Head: scm.PullRequestBranch{
			Ref: from.SourceBranch,
			Sha: headSHA,
			Repo: scm.Repository{
				ID: strconv.Itoa(from.SourceProjectID),
			},
		},
		Base: scm.PullRequestBranch{
			Ref: from.TargetBranch,
			Sha: from.DiffRefs.BaseSHA,
			Repo: scm.Repository{
				ID: strconv.Itoa(from.TargetProjectID),
			},
		},
		Created: from.Created,
		Updated: from.Updated,
	}
}

func convertPullRequestLabels(from []*string) []*scm.Label {
	var labels []*scm.Label
	for _, label := range from {
		l := *label
		labels = append(labels, &scm.Label{
			Name: l,
		})
	}
	return labels
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
