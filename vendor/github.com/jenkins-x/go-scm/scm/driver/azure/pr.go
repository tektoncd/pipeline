// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type pullService struct {
	*issueService
}

func (s *pullService) Update(ctx context.Context, s2 string, i int, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-request?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=6.0",
		ro.org, ro.project, ro.name, number)
	out := new(pr)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	return convertPullRequest(out), res, err
}

func (s *pullService) List(ctx context.Context, repo string, opts *scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/get-pull-request?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests?api-version=6.0", ro.org, ro.project, ro.name)
	out := new(prList)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	return convertPullRequests(out), res, err
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) ListCommits(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Commit, *scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/pull-request-commits/get-pull-request-commits?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullRequests/%d/commits?api-version=6.0",
		ro.org, ro.project, ro.name, number)
	out := new(commitList)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	return convertCommitList(out.Value), res, err
}

func (s *pullService) Merge(ctx context.Context, repo string, number int, opts *scm.PullRequestMergeOptions) (*scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, err
	}
	// https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/update?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=6.0",
		ro.org, ro.project, ro.name, number)

	out := pr{}
	_, err = s.client.do(ctx, "GET", endpoint, nil, &out)
	if err != nil {
		return nil, fmt.Errorf("could not fetch pr to merge: %v", err)
	}

	in := prUpdateInput{
		Status:                PrCompleted,
		LastMergeSourceCommit: out.LastMergeSourceCommit,
	}
	res, err := s.client.do(ctx, "PATCH", endpoint, in, &out)

	if out.Status != PrCompleted {
		// If you move fast, creating and immediately approving a pr, this often happens.
		return res, fmt.Errorf("patch accepted, but status still %s", out.Status)
	}

	return res, err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, err
	}

	// https://learn.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/update?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=6.0",
		ro.org, ro.project, ro.name, number)
	body := prUpdateInput{
		Status: PrAbandoned,
	}
	res, err := s.client.do(ctx, "PATCH", endpoint, body, nil)
	return res, err
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/pull-requests/create?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pullrequests?api-version=6.0", ro.org, ro.project, ro.name)
	in := &prInput{
		Title:         input.Title,
		Description:   input.Body,
		SourceRefName: scm.ExpandRef(input.Head, "refs/heads"),
		TargetRefName: scm.ExpandRef(input.Base, "refs/heads"),
	}
	out := new(pr)
	res, err := s.client.do(ctx, "POST", endpoint, in, out)
	return convertPullRequest(out), res, err
}

type prInput struct {
	SourceRefName string `json:"sourceRefName"`
	TargetRefName string `json:"targetRefName"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Reviewers     []struct {
		ID string `json:"id"`
	} `json:"reviewers"`
}

type pr struct {
	Repository struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		URL     string `json:"url"`
		WebURL  string `json:"webUrl"`
		Project struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			URL         string `json:"url"`
			State       string `json:"state"`
			Revision    int    `json:"revision"`
		} `json:"project"`
		RemoteURL string `json:"remoteUrl"`
	} `json:"repository"`
	PullRequestID int    `json:"pullRequestId"`
	CodeReviewID  int    `json:"codeReviewId"`
	Status        string `json:"status"`
	CreatedBy     struct {
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
		ImageURL    string `json:"imageUrl"`
	} `json:"createdBy"`
	CreationDate          time.Time   `json:"creationDate"`
	ClosedDate            null.String `json:"closedDate"`
	Title                 string      `json:"title"`
	Description           string      `json:"description"`
	SourceRefName         string      `json:"sourceRefName"`
	TargetRefName         string      `json:"targetRefName"`
	MergeStatus           string      `json:"mergeStatus"`
	MergeID               string      `json:"mergeId"`
	LastMergeSourceCommit *commitRef  `json:"lastMergeSourceCommit,omitempty"`
	LastMergeTargetCommit *commitRef  `json:"lastMergeTargetCommit,omitempty"`
	Reviewers             []struct {
		ReviewerURL string `json:"reviewerUrl"`
		Vote        int    `json:"vote"`
		ID          string `json:"id"`
		DisplayName string `json:"displayName"`
		UniqueName  string `json:"uniqueName"`
		URL         string `json:"url"`
		ImageURL    string `json:"imageUrl"`
	} `json:"reviewers"`
	URL   string `json:"url"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		Repository struct {
			Href string `json:"href"`
		} `json:"repository"`
		WorkItems struct {
			Href string `json:"href"`
		} `json:"workItems"`
		SourceBranch struct {
			Href string `json:"href"`
		} `json:"sourceBranch"`
		TargetBranch struct {
			Href string `json:"href"`
		} `json:"targetBranch"`
		SourceCommit struct {
			Href string `json:"href"`
		} `json:"sourceCommit"`
		TargetCommit struct {
			Href string `json:"href"`
		} `json:"targetCommit"`
		CreatedBy struct {
			Href string `json:"href"`
		} `json:"createdBy"`
		Iterations struct {
			Href string `json:"href"`
		} `json:"iterations"`
	} `json:"_links"`
	SupportsIterations bool   `json:"supportsIterations"`
	ArtifactID         string `json:"artifactId"`
}

type prList struct {
	Values []pr `json:"value"`
}

var (
	PrAbandoned = "abandoned"
	PrCompleted = "completed"
)

type prUpdateInput struct {
	Status                string     `json:"status"`
	LastMergeSourceCommit *commitRef `json:"lastMergeSourceCommit,omitempty"`
}

func convertPullRequests(from *prList) []*scm.PullRequest {
	var prs []*scm.PullRequest
	for index := range from.Values {
		prs = append(prs, convertPullRequest(&from.Values[index]))
	}
	return prs
}

func convertPullRequest(from *pr) *scm.PullRequest {
	return &scm.PullRequest{
		Number: from.PullRequestID,
		Title:  from.Title,
		Body:   from.Description,
		Sha:    from.LastMergeSourceCommit.CommitID,
		Source: scm.TrimRef(from.SourceRefName),
		Target: scm.TrimRef(from.TargetRefName),
		Link:   fmt.Sprintf("%s/pullrequest/%d", from.Repository.WebURL, from.PullRequestID),
		Closed: from.ClosedDate.Valid,
		Merged: from.Status == "completed",
		Ref:    fmt.Sprintf("refs/pull/%d/merge", from.PullRequestID),
		Head: scm.PullRequestBranch{
			Sha: from.LastMergeSourceCommit.CommitID,
		},
		Base: scm.PullRequestBranch{
			Sha: from.LastMergeTargetCommit.CommitID,
		},
		Author: scm.User{
			Login:  from.CreatedBy.UniqueName,
			Avatar: from.CreatedBy.ImageURL,
		},
		Created: from.CreationDate,
	}
}
