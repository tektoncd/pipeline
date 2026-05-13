// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
)

// GetCommitPullRequest gets the pull request associated with a commit SHA
func (c *Client) GetCommitPullRequest(owner, repo, sha string) (*PullRequest, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &sha); err != nil {
		return nil, nil, err
	}
	pr := new(PullRequest)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/commits/%s/pull", owner, repo, sha),
		jsonHeader, nil, &pr)
	return pr, resp, err
}

// UpdatePullRequest updates a pull request with new commits from the base branch
func (c *Client) UpdatePullRequest(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/update", owner, repo, index),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusOK {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// GetUserTrackedTimes gets all tracked times for a user in a repository
func (c *Client) GetUserTrackedTimes(owner, repo, user string) ([]*TrackedTime, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &user); err != nil {
		return nil, nil, err
	}
	times := make([]*TrackedTime, 0, 10)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/times/%s", owner, repo, user),
		jsonHeader, nil, &times)
	return times, resp, err
}
