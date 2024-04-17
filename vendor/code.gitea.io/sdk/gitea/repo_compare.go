// Copyright 2024 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import "fmt"

// Compare represents a comparison between two commits.
type Compare struct {
	TotalCommits int       `json:"total_commits"` // Total number of commits in the comparison.
	Commits      []*Commit `json:"commits"`       // List of commits in the comparison.
}

// CompareCommits compares two commits in a repository.
func (c *Client) CompareCommits(user, repo, prev, current string) (*Compare, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &prev, &current); err != nil {
		return nil, nil, err
	}

	basehead := fmt.Sprintf("%s...%s", prev, current)

	apiResp := new(Compare)
	resp, err := c.getParsedResponse(
		"GET",
		fmt.Sprintf("/repos/%s/%s/compare/%s", user, repo, basehead),
		nil, nil, apiResp,
	)
	return apiResp, resp, err
}
