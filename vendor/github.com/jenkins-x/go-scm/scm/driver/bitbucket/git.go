// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bitbucket

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type gitService struct {
	client *wrapper
}

func (s *gitService) FindRef(ctx context.Context, repo, ref string) (string, *scm.Response, error) {
	ref = strings.TrimPrefix(ref, "heads/")

	if strings.Index(ref, "/") > 0 {
		// ref is of pattern "foo/bar".  FindCommit method fails to work properly (Open ticket BCLOUD-9969) in this case.  Use FindBranch instead
		branchRef, res, err := s.FindBranch(ctx, repo, ref)

		if err == nil {
			return branchRef.Sha, res, nil
		}

		return ref, res, err
	}

	// here when ref pattern is not foo/bar
	commit, res, err := s.FindCommit(ctx, repo, ref)
	if err != nil && res == nil || (res.Status != 404 && res.Status >= 300) {
		return "", res, err
	}
	if commit != nil {
		if commit.Sha != "" {
			return commit.Sha, res, nil
		}
	}
	idx := strings.LastIndex(ref, "/")
	if idx >= 0 {
		ref = ref[idx+1:]
	}
	return ref, nil, nil
}

func (s *gitService) CreateRef(ctx context.Context, repo, ref, sha string) (*scm.Reference, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *gitService) DeleteRef(ctx context.Context, repo, ref string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *gitService) FindBranch(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/refs/branches/%s", repo, name)
	out := new(branch)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertBranch(out), res, err
}

func (s *gitService) FindCommit(ctx context.Context, repo, ref string) (*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/commit/%s", repo, ref)
	out := new(commit)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertCommit(out), res, err
}

func (s *gitService) FindTag(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/refs/tags/%s", repo, name)
	out := new(branch)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertTag(out), res, err
}

func (s *gitService) ListBranches(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/refs/branches?%s", repo, encodeListOptions(opts))
	out := new(branches)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertBranchList(out), res, err
}

func (s *gitService) ListCommits(ctx context.Context, repo string, opts scm.CommitListOptions) ([]*scm.Commit, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/commits/%s?%s", repo, opts.Ref, encodeCommitListOptions(opts))
	out := new(commits)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertCommitList(out), res, err
}

func (s *gitService) ListTags(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/refs/tags?%s", repo, encodeListOptions(opts))
	out := new(branches)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertTagList(out), res, err
}

func (s *gitService) ListChanges(ctx context.Context, repo, ref string, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/diffstat/%s?%s", repo, ref, encodeListOptions(opts))
	out := new(diffstats)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertDiffstats(out), res, err
}

func (s *gitService) CompareCommits(ctx context.Context, repo, ref1, ref2 string, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/diffstat/%s..%s?%s", repo, ref1, ref2, encodeListOptions(opts))
	out := new(diffstats)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
	return convertDiffstats(out), res, err
}

type branch struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Target struct {
		Hash string `json:"hash"`
	} `json:"target"`
}

type commits struct {
	pagination
	Values []*commit `json:"values"`
}

type branches struct {
	pagination
	Values []*branch `json:"values"`
}

type diffstats struct {
	pagination
	Values []*diffstat
}

type diffstat struct {
	Status string `json:"status"`
	Old    struct {
		Path  string `json:"path"`
		Type  string `json:"type"`
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"old"`
	LinesRemoved int `json:"lines_removed"`
	LinesAdded   int `json:"lines_added"`
	New          struct {
		Path  string `json:"path"`
		Type  string `json:"type"`
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	} `json:"new"`
}

type commit struct {
	Hash  string `json:"hash"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		Comments struct {
			Href string `json:"href"`
		} `json:"comments"`
		Patch struct {
			Href string `json:"href"`
		} `json:"patch"`
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
		Diff struct {
			Href string `json:"href"`
		} `json:"diff"`
		Approve struct {
			Href string `json:"href"`
		} `json:"approve"`
		Statuses struct {
			Href string `json:"href"`
		} `json:"statuses"`
	} `json:"links"`
	Author struct {
		Raw  string `json:"raw"`
		User struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
			AccountID   string `json:"account_id"`
			Links       struct {
				Self struct {
					Href string `json:"href"`
				} `json:"self"`
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
				Avatar struct {
					Href string `json:"href"`
				} `json:"avatar"`
			} `json:"links"`
			Type string `json:"type"`
			UUID string `json:"uuid"`
		} `json:"user"`
	} `json:"author"`
	Summary struct {
		Raw    string `json:"raw"`
		Markup string `json:"markup"`
		HTML   string `json:"html"`
		Type   string `json:"type"`
	} `json:"summary"`
	Date    time.Time `json:"date"`
	Message string    `json:"message"`
	Type    string    `json:"type"`
}

func convertDiffstats(from *diffstats) []*scm.Change {
	to := []*scm.Change{}
	for _, v := range from.Values {
		to = append(to, convertDiffstat(v))
	}
	return to
}

func convertDiffstat(from *diffstat) *scm.Change {
	return &scm.Change{
		Path:    from.New.Path,
		Added:   from.Status == "added",
		Renamed: from.Status == "renamed",
		Deleted: from.Status == "removed",
	}
}

func convertCommitList(from *commits) []*scm.Commit {
	to := []*scm.Commit{}
	for _, v := range from.Values {
		to = append(to, convertCommit(v))
	}
	return to
}

func convertCommit(from *commit) *scm.Commit {
	return &scm.Commit{
		Message: from.Message,
		Sha:     from.Hash,
		Link:    from.Links.HTML.Href,
		Author: scm.Signature{
			Name:   from.Author.User.DisplayName,
			Email:  extractEmail(from.Author.Raw),
			Date:   from.Date,
			Login:  from.Author.User.Username,
			Avatar: from.Author.User.Links.Avatar.Href,
		},
		Committer: scm.Signature{
			Name:   from.Author.User.DisplayName,
			Email:  extractEmail(from.Author.Raw),
			Date:   from.Date,
			Login:  from.Author.User.Username,
			Avatar: from.Author.User.Links.Avatar.Href,
		},
	}
}

func convertBranchList(from *branches) []*scm.Reference {
	to := []*scm.Reference{}
	for _, v := range from.Values {
		to = append(to, convertBranch(v))
	}
	return to
}

func convertBranch(from *branch) *scm.Reference {
	return &scm.Reference{
		Name: scm.TrimRef(from.Name),
		Path: scm.ExpandRef(from.Name, "refs/heads/"),
		Sha:  from.Target.Hash,
	}
}

func convertTagList(from *branches) []*scm.Reference {
	to := []*scm.Reference{}
	for _, v := range from.Values {
		to = append(to, convertTag(v))
	}
	return to
}

func convertTag(from *branch) *scm.Reference {
	return &scm.Reference{
		Name: scm.TrimRef(from.Name),
		Path: scm.ExpandRef(from.Name, "refs/tags/"),
		Sha:  from.Target.Hash,
	}
}
