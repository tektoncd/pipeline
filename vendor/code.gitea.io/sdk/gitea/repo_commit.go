// Copyright 2018 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"time"
)

// Identity for a person's identity like an author or committer
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CommitMeta contains meta information of a commit in terms of API.
type CommitMeta struct {
	URL     string    `json:"url"`
	SHA     string    `json:"sha"`
	Created time.Time `json:"created"`
}

// CommitUser contains information of a user in the context of a commit.
type CommitUser struct {
	Identity
	Date string `json:"date"`
}

// RepoCommit contains information of a commit in the context of a repository.
type RepoCommit struct {
	URL          string                     `json:"url"`
	Author       *CommitUser                `json:"author"`
	Committer    *CommitUser                `json:"committer"`
	Message      string                     `json:"message"`
	Tree         *CommitMeta                `json:"tree"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// CommitStats contains stats from a Git commit
type CommitStats struct {
	Total     int `json:"total"`
	Additions int `json:"additions"`
	Deletions int `json:"deletions"`
}

// Commit contains information generated from a Git commit.
type Commit struct {
	*CommitMeta
	HTMLURL    string                 `json:"html_url"`
	RepoCommit *RepoCommit            `json:"commit"`
	Author     *User                  `json:"author"`
	Committer  *User                  `json:"committer"`
	Parents    []*CommitMeta          `json:"parents"`
	Files      []*CommitAffectedFiles `json:"files"`
	Stats      *CommitStats           `json:"stats"`
}

// CommitDateOptions store dates for GIT_AUTHOR_DATE and GIT_COMMITTER_DATE
type CommitDateOptions struct {
	Author    time.Time `json:"author"`
	Committer time.Time `json:"committer"`
}

// CommitAffectedFiles store information about files affected by the commit
type CommitAffectedFiles struct {
	Filename string `json:"filename"`
}

// GetSingleCommit returns a single commit
func (c *Client) GetSingleCommit(user, repo, commitID string) (*Commit, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &commitID); err != nil {
		return nil, nil, err
	}
	commit := new(Commit)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/git/commits/%s", user, repo, commitID), nil, nil, &commit)
	return commit, resp, err
}

// ListCommitOptions list commit options
type ListCommitOptions struct {
	ListOptions
	// SHA or branch to start listing commits from (usually 'master')
	SHA string
	// Path indicates that only commits that include the path's file/dir should be returned.
	Path string
}

// QueryEncode turns options into querystring argument
func (opt *ListCommitOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.SHA != "" {
		query.Add("sha", opt.SHA)
	}
	if opt.Path != "" {
		query.Add("path", opt.Path)
	}
	return query.Encode()
}

// ListRepoCommits return list of commits from a repo
func (c *Client) ListRepoCommits(user, repo string, opt ListCommitOptions) ([]*Commit, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/commits", user, repo))
	opt.setDefaults()
	commits := make([]*Commit, 0, opt.PageSize)
	link.RawQuery = opt.QueryEncode()
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &commits)
	return commits, resp, err
}

// GetCommitDiff returns the commit's raw diff.
func (c *Client) GetCommitDiff(user, repo, commitID string) ([]byte, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_16_0); err != nil {
		return nil, nil, err
	}

	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}

	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/git/commits/%s.%s", user, repo, commitID, pullRequestDiffTypeDiff), nil, nil)
}

// GetCommitPatch returns the commit's raw patch.
func (c *Client) GetCommitPatch(user, repo, commitID string) ([]byte, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_16_0); err != nil {
		return nil, nil, err
	}

	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}

	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/git/commits/%s.%s", user, repo, commitID, pullRequestDiffTypePatch), nil, nil)
}
