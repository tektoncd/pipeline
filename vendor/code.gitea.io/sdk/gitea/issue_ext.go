// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// IssueBlockedBy represents an issue that blocks another issue
type IssueBlockedBy struct {
	Index     int64     `json:"index"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}

// ListIssueBlocksOptions options for listing issue blocks
type ListIssueBlocksOptions struct {
	ListOptions
}

// ListIssueBlocks lists issues that are blocked by the specified issue with pagination
func (c *Client) ListIssueBlocks(owner, repo string, index int64, opt ListIssueBlocksOptions) ([]*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/blocks", owner, repo, index))
	opt.setDefaults()
	link.RawQuery = opt.getURLQuery().Encode()
	issues := make([]*Issue, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &issues)
	return issues, resp, err
}

// IssueMeta represents issue reference for blocking/dependency operations
type IssueMeta struct {
	Index int64 `json:"index"`
}

// CreateIssueBlocking blocks an issue with another issue
func (c *Client) CreateIssueBlocking(owner, repo string, index int64, opt IssueMeta) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/blocks", owner, repo, index),
		jsonHeader, bytes.NewReader(body), &issue)
	return issue, resp, err
}

// RemoveIssueBlocking removes an issue block
func (c *Client) RemoveIssueBlocking(owner, repo string, index int64, opt IssueMeta) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/blocks", owner, repo, index),
		jsonHeader, bytes.NewReader(body), &issue)
	return issue, resp, err
}

// ListIssueDependenciesOptions options for listing issue dependencies
type ListIssueDependenciesOptions struct {
	ListOptions
}

// ListIssueDependencies lists issues that block the specified issue (its dependencies) with pagination
func (c *Client) ListIssueDependencies(owner, repo string, index int64, opt ListIssueDependenciesOptions) ([]*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies", owner, repo, index))
	opt.setDefaults()
	link.RawQuery = opt.getURLQuery().Encode()
	issues := make([]*Issue, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &issues)
	return issues, resp, err
}

// CreateIssueDependency creates a new issue dependency
func (c *Client) CreateIssueDependency(owner, repo string, index int64, opt IssueMeta) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies", owner, repo, index),
		jsonHeader, bytes.NewReader(body), &issue)
	return issue, resp, err
}

// RemoveIssueDependency removes an issue dependency
func (c *Client) RemoveIssueDependency(owner, repo string, index int64, opt IssueMeta) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/dependencies", owner, repo, index),
		jsonHeader, bytes.NewReader(body), &issue)
	return issue, resp, err
}

// LockIssueOption represents options for locking an issue
type LockIssueOption struct {
	LockReason string `json:"lock_reason"`
}

// LockIssue locks an issue
func (c *Client) LockIssue(owner, repo string, index int64, opt LockIssueOption) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("PUT",
		fmt.Sprintf("/repos/%s/%s/issues/%d/lock", owner, repo, index),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UnlockIssue unlocks an issue
func (c *Client) UnlockIssue(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE",
		fmt.Sprintf("/repos/%s/%s/issues/%d/lock", owner, repo, index),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// EditDeadlineOption represents options for updating issue deadline
type EditDeadlineOption struct {
	Deadline *time.Time `json:"due_date"`
}

// UpdateIssueDeadline updates an issue's deadline
func (c *Client) UpdateIssueDeadline(owner, repo string, index int64, opt EditDeadlineOption) (*Issue, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	issue := new(Issue)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/issues/%d/deadline", owner, repo, index),
		jsonHeader, bytes.NewReader(body), &issue)
	return issue, resp, err
}
