// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type gitService struct {
	client *wrapper
}

func (s *gitService) FindBranch(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/branches/%s", repo, name)
	out := new(branch)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertBranch(out), res, err
}

func (s *gitService) FindCommit(ctx context.Context, repo, ref string) (*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/commits/%s", repo, ref)
	out := new(commit)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertCommit(out), res, err
}

// FindRef returns the SHA of the given ref, such as "heads/master".
//
// See https://developer.github.com/v3/git/refs/#get-a-reference
func (s *gitService) FindRef(ctx context.Context, repo, ref string) (string, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/git/refs/%s", repo, ref)
	var out struct {
		Object map[string]string `json:"object"`
	}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return out.Object["sha"], res, err
}

// CreateRef creates a new ref
func (s *gitService) CreateRef(ctx context.Context, repo, ref, sha string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/git/refs", repo)
	in := &struct {
		Ref string `json:"ref"`
		Sha string `json:"sha"`
	}{Ref: ref, Sha: sha}

	out := &struct {
		Ref    string `json:"ref"`
		Object struct {
			Sha string
		} `json:"object"`
	}{}

	res, err := s.client.do(ctx, http.MethodPost, path, in, out)
	scmRef := &scm.Reference{
		Name: out.Ref,
		Sha:  out.Object.Sha,
		Path: out.Ref,
	}
	return scmRef, res, err
}

// DeleteRef deletes the given ref
//
// See https://developer.github.com/v3/git/refs/#delete-a-reference
func (s *gitService) DeleteRef(ctx context.Context, repo, ref string) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/git/refs/%s", repo, ref)
	res, err := s.client.do(ctx, http.MethodDelete, path, nil, nil)
	return res, err
}

func (s *gitService) FindTag(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) ListBranches(ctx context.Context, repo string, opts scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/branches?%s", repo, encodeListOptions(opts))
	out := []*branch{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertBranchList(out), res, err
}

func (s *gitService) ListCommits(ctx context.Context, repo string, opts scm.CommitListOptions) ([]*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/commits?%s", repo, encodeCommitListOptions(opts))
	out := []*commit{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertCommitList(out), res, err
}

func (s *gitService) ListTags(ctx context.Context, repo string, opts scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/tags?%s", repo, encodeListOptions(opts))
	out := []*branch{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertTagList(out), res, err
}

func (s *gitService) ListChanges(ctx context.Context, repo, ref string, _ scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/commits/%s", repo, ref)
	out := new(commit)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertChangeList(out.Files), res, err
}

func (s *gitService) CompareCommits(ctx context.Context, repo, ref1, ref2 string, _ scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("/repos/%s/compare/%s...%s", repo, ref1, ref2)
	out := new(commit)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertChangeList(out.Files), res, err
}

type branch struct {
	Name      string `json:"name"`
	Commit    commit `json:"commit"`
	Protected bool   `json:"protected"`
}

type commit struct {
	Sha    string `json:"sha"`
	URL    string `json:"html_url"`
	Commit struct {
		Tree struct {
			SHA string `json:"sha"`
			URL string `json:"url"`
		} `json:"tree"`
		Author struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"committer"`
		Message string `json:"message"`
	} `json:"commit"`
	Author struct {
		AvatarURL string `json:"avatar_url"`
		Login     string `json:"login"`
	} `json:"author"`
	Committer struct {
		AvatarURL string `json:"avatar_url"`
		Login     string `json:"login"`
	} `json:"committer"`
	Files []*file `json:"files"`
}

func convertCommitList(from []*commit) []*scm.Commit {
	to := []*scm.Commit{}
	for _, v := range from {
		to = append(to, convertCommit(v))
	}
	return to
}

func convertCommit(from *commit) *scm.Commit {
	return &scm.Commit{
		Message: from.Commit.Message,
		Sha:     from.Sha,
		Tree: scm.CommitTree{
			Sha:  from.Commit.Tree.SHA,
			Link: from.Commit.Tree.URL,
		},
		Link: from.URL,
		Author: scm.Signature{
			Name:   from.Commit.Author.Name,
			Email:  from.Commit.Author.Email,
			Date:   from.Commit.Author.Date,
			Login:  from.Author.Login,
			Avatar: from.Author.AvatarURL,
		},
		Committer: scm.Signature{
			Name:   from.Commit.Committer.Name,
			Email:  from.Commit.Committer.Email,
			Date:   from.Commit.Committer.Date,
			Login:  from.Committer.Login,
			Avatar: from.Committer.AvatarURL,
		},
	}
}

func convertBranchList(from []*branch) []*scm.Reference {
	to := []*scm.Reference{}
	for _, v := range from {
		to = append(to, convertBranch(v))
	}
	return to
}

func convertBranch(from *branch) *scm.Reference {
	return &scm.Reference{
		Name: scm.TrimRef(from.Name),
		Path: scm.ExpandRef(from.Name, "refs/heads/"),
		Sha:  from.Commit.Sha,
	}
}

func convertTagList(from []*branch) []*scm.Reference {
	to := []*scm.Reference{}
	for _, v := range from {
		to = append(to, convertTag(v))
	}
	return to
}

func convertTag(from *branch) *scm.Reference {
	return &scm.Reference{
		Name: scm.TrimRef(from.Name),
		Path: scm.ExpandRef(from.Name, "refs/tags/"),
		Sha:  from.Commit.Sha,
	}
}
