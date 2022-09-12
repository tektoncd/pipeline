// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gogs

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type gitService struct {
	client *wrapper
}

func (s *gitService) FindRef(ctx context.Context, repo, ref string) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *gitService) CreateRef(ctx context.Context, repo, ref, sha string) (*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) DeleteRef(ctx context.Context, repo, ref string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *gitService) FindBranch(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/branches/%s", repo, name)
	out := new(branch)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertBranch(out), res, err
}

func (s *gitService) FindCommit(ctx context.Context, repo, ref string) (*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/commits/%s", repo, ref)
	out := new(commitDetail)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertCommit(out), res, err
}

func (s *gitService) FindTag(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) ListBranches(ctx context.Context, repo string, _ *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/branches", repo)
	out := []*branch{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertBranchList(out), res, err
}

func (s *gitService) ListCommits(ctx context.Context, repo string, _ scm.CommitListOptions) ([]*scm.Commit, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) ListTags(ctx context.Context, repo string, _ *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) ListChanges(ctx context.Context, repo, ref string, _ *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) CompareCommits(ctx context.Context, repo, ref1, ref2 string, _ *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

//
// native data structures
//

type (
	// gogs branch object.
	branch struct {
		Name   string `json:"name"`
		Commit commit `json:"commit"`
	}

	// gogs commit object.
	commit struct {
		ID        string    `json:"id"`
		Message   string    `json:"message"`
		URL       string    `json:"url"`
		Author    signature `json:"author"`
		Committer signature `json:"committer"`
		Timestamp time.Time `json:"timestamp"`
	}

	// gogs commit detail object.
	commitDetail struct {
		Sha       string    `json:"sha"`
		Commit    commit    `json:"commit"`
		Committer committer `json:"committer"`
	}

	// gogs committer object.
	committer struct {
		AvatarURL string `json:"avatar_url"`
		Login     string `json:"login"`
		Email     string `json:"email"`
		FullName  string `json:"full_name"`
	}

	// gogs signature object.
	signature struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Username string `json:"username"`
	}
)

//
// native data structure conversion
//

func convertBranchList(src []*branch) []*scm.Reference {
	dst := []*scm.Reference{}
	for _, v := range src {
		dst = append(dst, convertBranch(v))
	}
	return dst
}

func convertBranch(src *branch) *scm.Reference {
	return &scm.Reference{
		Name: scm.TrimRef(src.Name),
		Path: scm.ExpandRef(src.Name, "refs/heads/"),
		Sha:  src.Commit.ID,
	}
}

// func convertCommitList(src []*commit) []*scm.Commit {
// 	dst := []*scm.Commit{}
// 	for _, v := range src {
// 		dst = append(dst, convertCommit(v))
// 	}
// 	return dst
// }

func convertCommit(src *commitDetail) *scm.Commit {
	return &scm.Commit{
		Sha:       src.Sha,
		Link:      src.Commit.URL,
		Message:   src.Commit.Message,
		Author:    convertSignature(src.Commit.Author),
		Committer: convertCommitter(src.Committer),
	}
}

func convertSignature(src signature) scm.Signature {
	return scm.Signature{
		Login: src.Username,
		Email: src.Email,
		Name:  src.Name,
	}
}

func convertCommitter(src committer) scm.Signature {
	return scm.Signature{
		Avatar: src.AvatarURL,
		Login:  src.Login,
		Email:  src.Email,
		Name:   src.FullName,
	}
}
