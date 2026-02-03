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
)

// UpdateOrgAvatar updates the organization's avatar
func (c *Client) UpdateOrgAvatar(org string, opt UpdateUserAvatarOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/orgs/%s/avatar", org),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// DeleteOrgAvatar deletes the organization's avatar
func (c *Client) DeleteOrgAvatar(org string) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE",
		fmt.Sprintf("/orgs/%s/avatar", org),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// RenameOrgOption options for renaming an organization
type RenameOrgOption struct {
	NewName string `json:"new_name"`
}

// RenameOrg renames an organization
func (c *Client) RenameOrg(org string, opt RenameOrgOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/orgs/%s/rename", org),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// ListOrgActivityFeedsOptions options for listing organization activity feeds
type ListOrgActivityFeedsOptions struct {
	ListOptions
	Date string `json:"date,omitempty"`
}

// ListOrgActivityFeeds lists the organization's activity feeds
func (c *Client) ListOrgActivityFeeds(org string, opt ListOrgActivityFeedsOptions) ([]*Activity, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/activities/feeds", org))
	query := opt.getURLQuery()
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	activities := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &activities)
	return activities, resp, err
}

// ListTeamActivityFeedsOptions options for listing team activity feeds
type ListTeamActivityFeedsOptions struct {
	ListOptions
	Date string `json:"date,omitempty"`
}

// ListTeamActivityFeeds lists the team's activity feeds
func (c *Client) ListTeamActivityFeeds(teamID int64, opt ListTeamActivityFeedsOptions) ([]*Activity, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/teams/%d/activities/feeds", teamID))
	query := opt.getURLQuery()
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	activities := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &activities)
	return activities, resp, err
}
