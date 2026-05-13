// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"time"
)

// ListRepoActivityFeedsOptions options for listing repository activity feeds
type ListRepoActivityFeedsOptions struct {
	ListOptions
	Date string `json:"date"` // the date of the activities to be found (format: YYYY-MM-DD)
}

// ListRepoActivityFeeds lists activity feeds for a repository
func (c *Client) ListRepoActivityFeeds(owner, repo string, opt ListRepoActivityFeedsOptions) ([]*Activity, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/activities/feeds", owner, repo))
	query := opt.getURLQuery()
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	feeds := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &feeds)
	return feeds, resp, err
}

// IssueConfigValidation represents the validation result for issue config
type IssueConfigValidation struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ValidateIssueConfig validates the issue config file for a repository
func (c *Client) ValidateIssueConfig(owner, repo string) (*IssueConfigValidation, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	result := new(IssueConfigValidation)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/issue_config/validate", owner, repo),
		jsonHeader, nil, &result)
	return result, resp, err
}

// TopicSearchOptions options for searching topics
type TopicSearchOptions struct {
	ListOptions
	Query string `json:"q"` // query string
}

// TopicSearchResult represents a topic search result
type TopicSearchResult struct {
	Topics []*TopicResponse `json:"topics"`
}

// TopicResponse represents a topic
type TopicResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"topic_name"`
	RepoCount int       `json:"repo_count"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
}

// SearchTopics searches for topics
func (c *Client) SearchTopics(opt TopicSearchOptions) (*TopicSearchResult, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/topics/search")
	query := opt.getURLQuery()
	if opt.Query != "" {
		query.Add("q", opt.Query)
	}
	link.RawQuery = query.Encode()

	result := new(TopicSearchResult)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &result)
	return result, resp, err
}
