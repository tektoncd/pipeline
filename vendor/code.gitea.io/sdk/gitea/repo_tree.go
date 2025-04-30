// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// GitEntry represents a git tree
type GitEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GitTreeResponse returns a git tree
type GitTreeResponse struct {
	SHA        string     `json:"sha"`
	URL        string     `json:"url"`
	Entries    []GitEntry `json:"tree"`
	Truncated  bool       `json:"truncated"`
	Page       int        `json:"page"`
	TotalCount int        `json:"total_count"`
}

type ListTreeOptions struct {
	ListOptions
	// Ref can be branch/tag/commit. required
	// e.g.: "master"
	Ref string
	// Recursive if true will return the tree in a recursive fashion
	Recursive bool
}

// GetTrees get trees of repository,
func (c *Client) GetTrees(user, repo string, opt ListTreeOptions) (*GitTreeResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &opt.Ref); err != nil {
		return nil, nil, err
	}
	trees := new(GitTreeResponse)
	opt.setDefaults()
	path := fmt.Sprintf("/repos/%s/%s/git/trees/%s?%s", user, repo, opt.Ref, opt.getURLQuery().Encode())

	if opt.Recursive {
		path += "&recursive=1"
	}
	resp, err := c.getParsedResponse("GET", path, nil, nil, trees)
	return trees, resp, err
}
