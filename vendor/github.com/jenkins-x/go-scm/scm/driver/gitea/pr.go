// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"context"
	"fmt"

	"code.gitea.io/sdk/gitea"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/jenkins-x/go-scm/scm"
)

type pullService struct {
	*issueService
}

func (s *pullService) Find(ctx context.Context, repo string, index int) (*scm.PullRequest, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.GetPullRequest(namespace, name, int64(index))
	return convertPullRequest(out), toSCMResponse(resp), err
}

func (s *pullService) List(ctx context.Context, repo string, opts *scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) { //nolint:gocritic
	namespace, name := scm.Split(repo)
	in := gitea.ListPullRequestsOptions{
		ListOptions: gitea.ListOptions{
			Page:     opts.Page,
			PageSize: opts.Size,
		},
	}
	if opts.Open && !opts.Closed {
		in.State = gitea.StateOpen
	} else if opts.Closed && !opts.Open {
		in.State = gitea.StateClosed
	}
	out, resp, err := s.client.GiteaClient.ListRepoPullRequests(namespace, name, in)
	return convertPullRequests(out), toSCMResponse(resp), err
}

// TODO: Maybe contribute to gitea/go-sdk with .patch function?
func (s *pullService) ListChanges(ctx context.Context, repo string, number int, _ *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	// Get the patch and then parse it.
	path := fmt.Sprintf("api/v1/repos/%s/pulls/%d.patch", repo, number)
	buf := new(bytes.Buffer)
	res, err := s.client.do(ctx, "GET", path, nil, buf)
	if err != nil {
		return nil, res, err
	}
	changedFiles, _, err := gitdiff.Parse(buf)
	if err != nil {
		return nil, res, err
	}
	var changes []*scm.Change
	for _, c := range changedFiles {
		var linesAdded int64
		var linesDeleted int64

		for _, tf := range c.TextFragments {
			linesAdded += tf.LinesAdded
			linesDeleted += tf.LinesDeleted
		}
		changes = append(changes, &scm.Change{
			Path:         c.NewName,
			PreviousPath: c.OldName,
			Added:        c.IsNew,
			Renamed:      c.IsRename,
			Deleted:      c.IsDelete,
			Additions:    int(linesAdded),
			Deletions:    int(linesDeleted),
		})
	}
	return changes, res, nil
}

func (s *pullService) ListCommits(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Commit, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) Merge(ctx context.Context, repo string, index int, options *scm.PullRequestMergeOptions) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.MergePullRequestOption{}

	if options != nil {
		in.Style = convertMergeMethodToMergeStyle(options.MergeMethod)
		in.Title = options.CommitTitle
	}

	_, resp, err := s.client.GiteaClient.MergePullRequest(namespace, name, int64(index), in)
	return toSCMResponse(resp), err
}

func (s *pullService) Update(ctx context.Context, repo string, number int, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.EditPullRequestOption{
		Title: input.Title,
		Body:  input.Body,
		Base:  input.Base,
	}
	out, resp, err := s.client.GiteaClient.EditPullRequest(namespace, name, int64(number), in)
	return convertPullRequest(out), toSCMResponse(resp), err
}

func (s *pullService) Close(ctx context.Context, repo string, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	closed := gitea.StateClosed
	in := gitea.EditPullRequestOption{
		State: &closed,
	}
	_, resp, err := s.client.GiteaClient.EditPullRequest(namespace, name, int64(number), in)
	return toSCMResponse(resp), err
}

func (s *pullService) Reopen(ctx context.Context, repo string, number int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	reopen := gitea.StateOpen
	in := gitea.EditPullRequestOption{
		State: &reopen,
	}
	_, resp, err := s.client.GiteaClient.EditPullRequest(namespace, name, int64(number), in)
	return toSCMResponse(resp), err
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.CreatePullRequestOption{
		Head:  input.Head,
		Base:  input.Base,
		Title: input.Title,
		Body:  input.Body,
	}
	out, resp, err := s.client.GiteaClient.CreatePullRequest(namespace, name, in)
	return convertPullRequest(out), toSCMResponse(resp), err
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return s.AssignIssue(ctx, repo, number, logins)
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return s.UnassignIssue(ctx, repo, number, logins)
}

//
// native data structure conversion
//

func convertPullRequests(src []*gitea.PullRequest) []*scm.PullRequest {
	dst := []*scm.PullRequest{}
	for _, v := range src {
		dst = append(dst, convertPullRequest(v))
	}
	return dst
}

func convertPullRequest(src *gitea.PullRequest) *scm.PullRequest {
	if src == nil || src.Title == "" {
		return nil
	}
	pr := &scm.PullRequest{
		Number:    int(src.Index),
		Title:     src.Title,
		Body:      src.Body,
		Labels:    convertLabels(src.Labels),
		Sha:       src.Head.Sha,
		Ref:       fmt.Sprintf("refs/pull/%d/head", src.Index),
		State:     string(src.State),
		Source:    src.Head.Name,
		Target:    src.Base.Name,
		Fork:      src.Base.Repository.FullName,
		Base:      *convertPullRequestBranch(src.Base),
		Head:      *convertPullRequestBranch(src.Head),
		DiffLink:  src.DiffURL,
		Link:      src.HTMLURL,
		Closed:    src.State == gitea.StateClosed,
		Author:    *convertUser(src.Poster),
		Assignees: convertUsers(src.Assignees),
		Merged:    src.HasMerged,
		Mergeable: src.Mergeable,
		Created:   *src.Created,
		Updated:   *src.Updated,
	}
	if src.MergedCommitID != nil {
		pr.MergeSha = *src.MergedCommitID
	}
	return pr
}

func convertPullRequestBranch(src *gitea.PRBranchInfo) *scm.PullRequestBranch {
	return &scm.PullRequestBranch{
		Ref:  src.Ref,
		Sha:  src.Sha,
		Repo: *convertRepository(src.Repository),
	}
}

func convertMergeMethodToMergeStyle(mm string) gitea.MergeStyle {
	switch mm {
	case "merge":
		return gitea.MergeStyleMerge
	case "rebase":
		return gitea.MergeStyleRebase
	case "rebase-merge":
		return gitea.MergeStyleRebaseMerge
	case "squash":
		return gitea.MergeStyleSquash
	default:
		return gitea.MergeStyleMerge
	}
}
