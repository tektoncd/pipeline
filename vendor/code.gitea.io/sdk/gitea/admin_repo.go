// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// AdminCreateRepo create a repo
func (c *Client) AdminCreateRepo(user string, opt CreateRepoOption) (*Repository, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	repo := new(Repository)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/admin/users/%s/repos", user), jsonHeader, bytes.NewReader(body), repo)
	return repo, resp, err
}

// ListUnadoptedReposOptions options for listing unadopted repositories
type ListUnadoptedReposOptions struct {
	ListOptions
	Pattern string `json:"pattern,omitempty"`
}

// ListUnadoptedRepos lists unadopted repositories
func (c *Client) ListUnadoptedRepos(opt ListUnadoptedReposOptions) ([]string, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/unadopted")
	query := opt.getURLQuery()
	if opt.Pattern != "" {
		query.Add("pattern", opt.Pattern)
	}
	link.RawQuery = query.Encode()

	repos := make([]string, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &repos)
	return repos, resp, err
}

// AdoptUnadoptedRepo adopts an unadopted repository
func (c *Client) AdoptUnadoptedRepo(owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("POST",
		fmt.Sprintf("/admin/unadopted/%s/%s", owner, repo),
		jsonHeader, nil)
}

// DeleteUnadoptedRepo deletes an unadopted repository
func (c *Client) DeleteUnadoptedRepo(owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE",
		fmt.Sprintf("/admin/unadopted/%s/%s", owner, repo),
		jsonHeader, nil)
}
