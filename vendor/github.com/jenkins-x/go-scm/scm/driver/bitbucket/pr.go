// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bitbucket

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm/labels"

	"github.com/jenkins-x/go-scm/scm"
)

type pullService struct {
	*issueService
}

const debugDump = false

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests/%d", repo, number)
	out := new(pullRequest)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	responsePR := convertPullRequest(out)
	populateMergeableState(ctx, s, out, responsePR)

	return responsePR, res, err
}

func (s *pullService) List(ctx context.Context, repo string, opts *scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests?%s", repo, encodePullRequestListOptions(opts))
	out := new(pullRequests)
	if debugDump {
		var buf bytes.Buffer
		res, err := s.client.do(ctx, "GET", path, nil, &buf)
		fmt.Printf("%s\n", buf.String())
		return nil, res, err
	}
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertPullRequests(ctx, s, out), res, err
}

type prCommentInput struct {
	Content struct {
		Raw string `json:"raw,omitempty"`
	} `json:"content"`
}

type pullRequestComments struct {
	pagination
	Values []*prComment `json:"values"`
}

type prComment struct {
	ID    int    `json:"id"`
	Type  string `json:"type"`
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html,omitempty"`
		Self struct {
			Href string `json:"href"`
		} `json:"self,omitempty"`
		Code struct {
			Href string `json:"href"`
		} `json:"code,omitempty"`
	} `json:"links"`
	PR struct {
		Title string `json:"title"`
		ID    int    `json:"id"`
		Type  string `json:"type"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"pullrequest"`
	User struct {
		AccountID   string `json:"account_id"`
		DisplayName string `json:"display_name"`
		UUID        string `json:"uuid"`
		Type        string `json:"type"`
		NickName    string `json:"nickname"`
		Links       struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
			Avatar struct {
				Href string `json:"href"`
			} `json:"avatar"`
		} `json:"links"`
	} `json:"user"`
	Content struct {
		Raw    string `json:"raw"`
		Markup string `json:"markup"`
		HTML   string `json:"html"`
		Type   string `json:"type"`
	} `json:"content"`
	Inline struct {
		To   int    `json:"to,omitempty"`
		From int    `json:"from,omitempty"`
		Path string `json:"path,omitempty"`
	} `json:"inline,omitempty"`
	Deleted   bool      `json:"deleted"`
	UpdatedOn time.Time `json:"updated_on"`
	CreatedOn time.Time `json:"created_on"`
}

func convertPRComment(from *prComment) *scm.Comment {
	return &scm.Comment{
		ID:   from.ID,
		Body: from.Content.Raw,
		Author: scm.User{
			Name:   from.User.DisplayName,
			Login:  from.User.AccountID,
			Avatar: from.User.Links.Avatar.Href,
		},
		Link:    from.Links.HTML.Href,
		Created: from.CreatedOn,
		Updated: from.UpdatedOn,
	}
}

func (s *pullService) CreateComment(ctx context.Context, repo string, number int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests/%d/comments", repo, number)
	in := new(prCommentInput)
	in.Content.Raw = input.Body
	out := new(prComment)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertPRComment(out), res, err
}

func (s *pullService) DeleteComment(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests/%d/comments/%d", repo, number, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func convertPRCommentList(from *pullRequestComments) []*scm.Comment {
	to := []*scm.Comment{}
	for _, v := range from.Values {
		to = append(to, convertPRComment(v))
	}
	return to
}

func (s *pullService) ListComments(ctx context.Context, repo string, index int, opts *scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests/%d/comments?%s", repo, index, encodeListOptions(opts))
	out := new(pullRequestComments)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertPRCommentList(out), res, err
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests/%d/diffstat?%s", repo, number, encodeListOptions(opts))
	out := new(diffstats)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertDiffstats(out), res, err
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

func (s *pullService) ListCommits(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Commit, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

type prMerge struct {
	Message           string `json:"message"`
	CloseSourceBranch bool   `json:"close_source_branch"`
	MergeStrategy     string `json:"merge_strategy"`
}

func (s *pullService) Merge(ctx context.Context, repo string, number int, options *scm.PullRequestMergeOptions) (*scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests/%d/merge", repo, number)
	in := &prMerge{
		Message:           "Merging PR",
		CloseSourceBranch: false,
		MergeStrategy:     "merge_commit",
	}
	res, err := s.client.do(ctx, "POST", path, in, nil)
	return res, err
}

func (s *pullService) Update(ctx context.Context, repo string, number int, prInput *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type prPatchName struct {
	Name string `json:"name"`
}

type prPatchBranch struct {
	Branch prPatchName `json:"branch,omitempty"`
}

type prInput struct {
	Title   string        `json:"title,omitempty"`
	Source  prPatchBranch `json:"source,omitempty"`
	Project string
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/pullrequests", repo)
	in := &prInput{
		Title: input.Title,
		Source: prPatchBranch{
			Branch: prPatchName{
				Name: input.Head,
			},
		},
	}
	out := new(pullRequest)
	res, err := s.client.do(ctx, "POST", path, in, out)

	responsePR := convertPullRequest(out)

	populateMergeableState(ctx, s, out, responsePR)
	return responsePR, res, err
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type prSource struct {
	Commit struct {
		Type   string `json:"type"`
		Ref    string `json:"ref"`
		Commit string `json:"hash"`
	} `json:"commit"`
	Repository repository `json:"repository"`
	Branch     struct {
		Name string `json:"name"`
	} `json:"branch"`
}

type prDestination struct {
	Commit struct {
		Type   string `json:"type"`
		Ref    string `json:"ref"`
		Commit string `json:"hash"`
	} `json:"commit"`
	Repository repository `json:"repository"`
	Branch     struct {
		Name string `json:"name"`
	} `json:"branch"`
}

type pullRequest struct {
	ID           int           `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	State        string        `json:"state"`
	CreatedDate  time.Time     `json:"created_on"`
	UpdatedDate  time.Time     `json:"updated_on"`
	Source       prSource      `json:"source"`
	Destination  prDestination `json:"destination"`
	Locked       bool          `json:"locked"`
	Author       user          `json:"author"`
	Reviewers    []user        `json:"reviewers"`
	Participants []user        `json:"participants"`
	Links        struct {
		DiffStat link `json:"diffstat"`
		Self     link `json:"self"`
		Diff     link `json:"diff"`
		HTML     link `json:"html"`
	} `json:"links"`
}

type pullRequests struct {
	pagination
	Values []*pullRequest `json:"values"`
}

func findRefs(from *pullRequest) (string, string) {
	baseRef := from.Destination.Commit.Ref
	headRef := from.Source.Commit.Ref
	if baseRef == "" {
		baseRef = from.Destination.Branch.Name
	}
	if headRef == "" {
		headRef = from.Source.Branch.Name
	}
	return baseRef, headRef
}

func convertPullRequest(from *pullRequest) *scm.PullRequest {
	fork := "false"
	closed := !(strings.EqualFold(from.State, "open"))

	baseRef, headRef := findRefs(from)
	return &scm.PullRequest{
		Number:   from.ID,
		Title:    from.Title,
		Body:     from.Description,
		Sha:      from.Source.Commit.Commit,
		Ref:      fmt.Sprintf("refs/pull-requests/%d/from", from.ID),
		Source:   from.Source.Commit.Commit,
		Target:   from.Destination.Commit.Commit,
		Fork:     fork,
		Base:     convertPullRequestBranch(baseRef, from.Destination.Commit.Commit, &from.Destination.Repository),
		Head:     convertPullRequestBranch(headRef, from.Source.Commit.Commit, &from.Source.Repository),
		Link:     from.Links.HTML.Href,
		DiffLink: from.Links.Diff.Href,
		State:    strings.ToLower(from.State),
		Closed:   closed,
		Merged:   from.State == "MERGED",
		Created:  from.CreatedDate,
		Updated:  from.UpdatedDate,
		Author: scm.User{
			Login:  from.Author.GetLogin(),
			Name:   from.Author.DisplayName,
			Email:  from.Author.EmailAddress,
			Link:   from.Author.Links.Self.Href,
			Avatar: from.Author.Links.Avatar.Href,
		},
	}
}

func convertPullRequestBranch(ref, sha string, repo *repository) scm.PullRequestBranch {
	return scm.PullRequestBranch{
		Ref:  ref,
		Sha:  sha,
		Repo: *convertRepository(repo),
	}
}

func convertPullRequests(ctx context.Context, prsvc *pullService, from *pullRequests) []*scm.PullRequest {
	answer := []*scm.PullRequest{}
	for _, pr := range from.Values {
		responsePR := convertPullRequest(pr)
		responsePR = populateMergeableState(ctx, prsvc, pr, responsePR)
		answer = append(answer, responsePR)
	}
	return answer
}

func populateMergeableState(ctx context.Context, prsvc *pullService, from *pullRequest, to *scm.PullRequest) *scm.PullRequest {
	out := new(diffstats)
	_, err := prsvc.client.do(ctx, "GET", from.Links.DiffStat.Href, nil, out)
	if err != nil {
		// error judging PR mergeable status, defaulting to, unknown
		to.MergeableState = scm.MergeableStateUnknown
		to.Mergeable = false
		return to
	}

	mergeableFlag := true
	mergeableState := scm.MergeableStateMergeable

	for _, diff := range out.Values {
		if diff.Status == "merge conflict" {
			// there exists conflict, no need to scan subsequent diffs
			mergeableFlag = false
			mergeableState = scm.MergeableStateConflicting
			break
		}
	}

	to.Mergeable = mergeableFlag
	to.MergeableState = mergeableState

	return to
}
