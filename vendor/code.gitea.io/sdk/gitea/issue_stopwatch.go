// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"time"
)

// StopWatch represents a running stopwatch of an issue / pr
type StopWatch struct {
	Created       time.Time `json:"created"`
	Seconds       int64     `json:"seconds"`
	Duration      string    `json:"duration"`
	IssueIndex    int64     `json:"issue_index"`
	IssueTitle    string    `json:"issue_title"`
	RepoOwnerName string    `json:"repo_owner_name"`
	RepoName      string    `json:"repo_name"`
}

// ListStopwatchesOptions options for listing stopwatches
type ListStopwatchesOptions struct {
	ListOptions
}

// GetMyStopwatches list all stopwatches
//
// Deprecated: Use ListMyStopwatches instead, which supports pagination.
func (c *Client) GetMyStopwatches() ([]*StopWatch, *Response, error) {
	return c.ListMyStopwatches(ListStopwatchesOptions{})
}

// ListMyStopwatches list all stopwatches with pagination
func (c *Client) ListMyStopwatches(opt ListStopwatchesOptions) ([]*StopWatch, *Response, error) {
	link, _ := url.Parse("/user/stopwatches")
	opt.setDefaults()
	link.RawQuery = opt.getURLQuery().Encode()
	stopwatches := make([]*StopWatch, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &stopwatches)
	return stopwatches, resp, err
}

// DeleteIssueStopwatch delete / cancel a specific stopwatch
func (c *Client) DeleteIssueStopwatch(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/delete", owner, repo, index), nil, nil)
}

// StartIssueStopWatch starts a stopwatch for an existing issue for a given
// repository
func (c *Client) StartIssueStopWatch(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/start", owner, repo, index), nil, nil)
}

// StopIssueStopWatch stops an existing stopwatch for an issue in a given
// repository
func (c *Client) StopIssueStopWatch(owner, repo string, index int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("POST", fmt.Sprintf("/repos/%s/%s/issues/%d/stopwatch/stop", owner, repo, index), nil, nil)
}
