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
)

type gitService struct {
	client *wrapper
}

func (s *gitService) FindRef(ctx context.Context, repo, ref string) (string, *scm.Response, error) {
	ref = strings.TrimPrefix(ref, "heads/")
	path := fmt.Sprintf("api/v4/projects/%s/repository/commits?ref_name=%s", encode(repo), ref)
	out := []*commit{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return "", res, err
	}
	if len(out) > 0 {
		commit := out[0]
		if commit.ID != "" {
			return commit.ID, res, err
		}
	}
	idx := strings.LastIndex(ref, "/")
	if idx >= 0 {
		ref = ref[idx+1:]
	}
	return ref, res, err
}

func (s *gitService) CreateRef(ctx context.Context, repo, ref, sha string) (*scm.Reference, *scm.Response, error) {
	params := url.Values{
		"branch": []string{ref},
		"ref":    []string{sha},
	}
	path := fmt.Sprintf("api/v4/projects/%s/repository/branches?%s", encode(repo), params.Encode())

	out := &struct {
		Name   string `json:"name"`
		Commit struct {
			ID string `json:"id"`
		} `json:"commit"`
	}{}

	res, err := s.client.do(ctx, "POST", path, nil, out)
	scmRef := &scm.Reference{
		Name: out.Name,
		Sha:  out.Commit.ID,
	}
	return scmRef, res, err
}

func (s *gitService) DeleteRef(ctx context.Context, repo, ref string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *gitService) FindBranch(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/branches/%s", encode(repo), encode(name))
	out := new(branch)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertBranch(out), res, err
}

func (s *gitService) FindCommit(ctx context.Context, repo, ref string) (*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/commits/%s", encode(repo), encode(scm.TrimRef(ref)))
	out := new(commit)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertCommit(out), res, err
}

func (s *gitService) FindTag(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/tags/%s", encode(repo), encode(name))
	out := new(branch)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertTag(out), res, err
}

func (s *gitService) ListBranches(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/branches?%s", encode(repo), encodeListOptions(opts))
	out := []*branch{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertBranchList(out), res, err
}

func (s *gitService) ListCommits(ctx context.Context, repo string, opts scm.CommitListOptions) ([]*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/commits?%s", encode(repo), encodeCommitListOptions(opts))
	out := []*commit{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertCommitList(out), res, err
}

func (s *gitService) ListTags(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/tags?%s", encode(repo), encodeListOptions(opts))
	out := []*branch{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertTagList(out), res, err
}

func (s *gitService) ListChanges(ctx context.Context, repo, ref string, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/commits/%s/diff", encode(repo), encode(ref))
	out := []*change{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertChangeList(out), res, err
}

func (s *gitService) CompareCommits(ctx context.Context, repo, ref1, ref2 string, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	opts.From = encode(ref1)
	opts.To = encode(ref2)
	path := fmt.Sprintf("api/v4/projects/%s/repository/compare?%s", encode(repo), encodeListOptions(opts))
	out := compare{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertChangeList(out.Diffs), res, err
}

type compare struct {
	Diffs []*change `json:"diffs"`
}

type branch struct {
	Name   string `json:"name"`
	Commit struct {
		ID string `json:"id"`
	}
}

type commit struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Message        string    `json:"message"`
	AuthorName     string    `json:"author_name"`
	AuthorEmail    string    `json:"author_email"`
	AuthorDate     time.Time `json:"authored_date"`
	CommittedDate  time.Time `json:"committed_date"`
	CommitterName  string    `json:"committer_name"`
	CommitterEmail string    `json:"committer_email"`
	Created        time.Time `json:"created_at"`
	URL            string    `json:"web_url"`
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
		Message: from.Message,
		Sha:     from.ID,
		Link:    from.URL,
		Author: scm.Signature{
			Login: from.AuthorName,
			Name:  from.AuthorName,
			Email: from.AuthorEmail,
			Date:  from.AuthorDate,
		},
		Committer: scm.Signature{
			Login: from.CommitterName,
			Name:  from.CommitterName,
			Email: from.CommitterEmail,
			Date:  from.CommittedDate,
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
		Sha:  from.Commit.ID,
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
		Sha:  from.Commit.ID,
	}
}
