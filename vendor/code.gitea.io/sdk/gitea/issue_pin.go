// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
)

// ListRepoPinnedIssues lists a repo's pinned issues
func (c *Client) ListRepoPinnedIssues(owner, repo string) ([]*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	issues := make([]*Issue, 0, 5)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/issues/pinned", owner, repo),
		jsonHeader, nil, &issues)
	return issues, resp, err
}

// PinIssue pins an issue
func (c *Client) PinIssue(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/pin", owner, repo, index),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UnpinIssue unpins an issue
func (c *Client) UnpinIssue(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/pin", owner, repo, index),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// MoveIssuePin moves a pinned issue to the given position
func (c *Client) MoveIssuePin(owner, repo string, index, position int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("PATCH",
		fmt.Sprintf("/repos/%s/%s/issues/%d/pin/%d", owner, repo, index, position),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
