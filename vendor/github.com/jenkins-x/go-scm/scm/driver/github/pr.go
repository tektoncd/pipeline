// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
	errors2 "k8s.io/apimachinery/pkg/util/errors"
)

var (
	teamRe = regexp.MustCompile(`^(.*)/(.*)$`)
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

func (s *pullService) Merge(ctx context.Context, repo string, number int, options *scm.PullRequestMergeOptions) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/merge", repo, number)
	res, err := s.client.do(ctx, "PUT", path, encodePullRequestMergeOptions(options), nil)
	return res, err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repo, number)
	data := map[string]string{"state": "closed"}
	res, err := s.client.do(ctx, "PATCH", path, &data, nil)
	return res, err
}

func (s *pullService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repo, number)
	data := map[string]string{"state": "open"}
	res, err := s.client.do(ctx, "PATCH", path, &data, nil)
	return res, err
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls", repo)
	in := &prInput{
		Title: input.Title,
		Head:  input.Head,
		Base:  input.Base,
		Body:  input.Body,
	}

	out := new(pr)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) Update(ctx context.Context, repo string, number int, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d", repo, number)
	in := &prInput{}
	if input.Title != "" {
		in.Title = input.Title
	}
	if input.Body != "" {
		in.Body = input.Body
	}
	if input.Head != "" {
		in.Head = input.Head
	}
	if input.Base != "" {
		in.Base = input.Base
	}

	out := new(pr)
	res, err := s.client.do(ctx, "PATCH", path, in, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	_, resp, err := s.tryRequestReview(ctx, repo, number, logins)
	// At least one invalid user. Try adding them individually.
	if err != nil && resp != nil && resp.Status == http.StatusUnprocessableEntity {
		missing := scm.MissingUsers{
			Action: "request a PR review from",
		}
		for _, user := range logins {
			_, resp, err = s.tryRequestReview(ctx, repo, number, []string{user})
			if err != nil && resp != nil && resp.Status == http.StatusUnprocessableEntity {
				missing.Users = append(missing.Users, user)
			} else if err != nil {
				return nil, fmt.Errorf("failed to add reviewer to PR. errmsg: %v", err)
			}
		}
		if len(missing.Users) > 0 {
			return nil, missing
		}
	}
	return resp, err
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	body, err := prepareReviewersBody(logins, strings.Split(repo, "/")[0])
	if err != nil {
		return nil, err
	}
	if len(body.TeamReviewers) == 0 && len(body.Reviewers) == 0 {
		return nil, nil
	}
	path := fmt.Sprintf("repos/%s/pulls/%d/requested_reviewers", repo, number)
	out := new(pr)
	res, err := s.client.do(ctx, "DELETE", path, body, out)
	if err != nil {
		return res, err
	}
	extras := scm.ExtraUsers{Action: "remove the PR review request for"}
	for _, user := range out.RequestedReviewers {
		found := false
		for _, toDelete := range logins {
			if NormLogin(user.Login) == NormLogin(toDelete) {
				found = true
				break
			}
		}
		if found {
			extras.Users = append(extras.Users, user.Login)
		}
	}
	if len(extras.Users) > 0 {
		return res, extras
	}
	return res, nil
}

func prepareReviewersBody(logins []string, org string) (prReviewers, error) {
	body := prReviewers{}
	var errors []error
	for _, login := range logins {
		mat := teamRe.FindStringSubmatch(login)
		if mat == nil {
			body.Reviewers = append(body.Reviewers, login)
		} else if mat[1] == org {
			body.TeamReviewers = append(body.TeamReviewers, mat[2])
		} else {
			errors = append(errors, fmt.Errorf("team %s is not part of %s org", login, org))
		}
	}

	return body, errors2.NewAggregate(errors)
}

func (s *pullService) tryRequestReview(ctx context.Context, orgAndRepo string, number int, logins []string) (*scm.PullRequest, *scm.Response, error) {
	body, err := prepareReviewersBody(logins, strings.Split(orgAndRepo, "/")[0])
	if err != nil {
		// At least one team not in org,
		// let RequestReview handle retries and alerting for each login.
		return nil, nil, err
	}

	out := new(pr)
	path := fmt.Sprintf("repos/%s/pulls/%d/requested_reviewers", orgAndRepo, number)
	res, err := s.client.do(ctx, "POST", path, body, out)
	return convertPullRequest(out), res, err
}

type prReviewers struct {
	Reviewers     []string `json:"reviewers,omitempty"`
	TeamReviewers []string `json:"team_reviewers,omitempty"`
}

type pullRequestMergeRequest struct {
	CommitMessage string `json:"commit_message,omitempty"`
	CommitTitle   string `json:"commit_title,omitempty"`
	MergeMethod   string `json:"merge_method,omitempty"`
	SHA           string `json:"sha,omitempty"`
}

type prBranch struct {
	Ref  string     `json:"ref"`
	Sha  string     `json:"sha"`
	User user       `json:"user"`
	Repo repository `json:"repo"`
}

type pr struct {
	Number             int         `json:"number"`
	State              string      `json:"state"`
	Title              string      `json:"title"`
	Body               string      `json:"body"`
	Labels             []*label    `json:"labels"`
	DiffURL            string      `json:"diff_url"`
	HTMLURL            string      `json:"html_url"`
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

type prInput struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	Head  string `json:"head,omitempty"`
	Base  string `json:"base,omitempty"`
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
		DiffLink:       from.DiffURL,
		Link:           from.HTMLURL,
		Closed:         from.State != "open",
		Draft:          from.Draft,
		MergeSha:       from.MergeSha,
		Merged:         from.Merged,
		Mergeable:      from.Mergeable,
		MergeableState: scm.ToMergeableState(from.MergeableState),
		Rebaseable:     from.Rebaseable,
		Author:         *convertUser(&from.User),
		Assignees:      convertUsers(from.Assignees),
		Reviewers:      convertUsers(from.RequestedReviewers),
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
		Path:         from.Filename,
		PreviousPath: from.PreviousFilename,
		Added:        from.Status == "added",
		Deleted:      from.Status == "deleted",
		Renamed:      from.Status == "moved",
		Patch:        from.Patch,
		Additions:    from.Additions,
		Deletions:    from.Deletions,
		Changes:      from.Changes,
		BlobURL:      from.BlobURL,
		Sha:          from.Sha,
	}
}
