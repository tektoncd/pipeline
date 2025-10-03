// Copyright 2024 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// TagProtection represents a tag protection for a repository
type TagProtection struct {
	Id                 int64     `json:"id"`
	NamePattern        string    `json:"name_pattern"`
	WhitelistUsernames []string  `json:"whitelist_usernames"`
	WhitelistTeams     []string  `json:"whitelist_teams"`
	Created            time.Time `json:"created_at"`
	Updated            time.Time `json:"updated_at"`
}

// CreateTagProtectionOption options for creating a tag protection
type CreateTagProtectionOption struct {
	NamePattern        string   `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
}

// EditTagProtectionOption options for editing a tag protection
type EditTagProtectionOption struct {
	NamePattern        *string  `json:"name_pattern"`
	WhitelistUsernames []string `json:"whitelist_usernames"`
	WhitelistTeams     []string `json:"whitelist_teams"`
}

// ListRepoTagsOptions options for listing a repository's tags
type ListRepoTagProtectionsOptions struct {
	ListOptions
}

// ListTagProtection list tag protections for a repository
func (c *Client) ListTagProtection(owner, repo string, opt ListRepoTagProtectionsOptions) ([]*TagProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_23_0); err != nil {
		return nil, nil, err
	}

	tps := make([]*TagProtection, 0, opt.PageSize)
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/tag_protections", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &tps)
	return tps, resp, err
}

// GetTagProtection gets a tag protection
func (c *Client) GetTagProtection(owner, repo string, id int64) (*TagProtection, *Response, error) {
	sid := strconv.FormatInt(id, 10)
	if err := escapeValidatePathSegments(&owner, &repo, &sid); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_23_0); err != nil {
		return nil, nil, err
	}
	tp := new(TagProtection)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/tag_protections/%d", owner, repo, id), jsonHeader, nil, tp)
	return tp, resp, err
}

// CreateTagProtection creates a tag protection for a repo
func (c *Client) CreateTagProtection(owner, repo string, opt CreateTagProtectionOption) (*TagProtection, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_23_0); err != nil {
		return nil, nil, err
	}

	tp := new(TagProtection)
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/tag_protections", owner, repo), jsonHeader, bytes.NewReader(body), tp)
	return tp, resp, err
}

// EditTagProtection edits a tag protection for a repo
func (c *Client) EditTagProtection(owner, repo string, id int64, opt EditTagProtectionOption) (*TagProtection, *Response, error) {
	sid := strconv.FormatInt(id, 10)
	if err := escapeValidatePathSegments(&owner, &repo, &sid); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_23_0); err != nil {
		return nil, nil, err
	}

	tp := new(TagProtection)

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/tag_protections/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), tp)
	return tp, resp, err
}

// DeleteTagProtection deletes a tag protection for a repo
func (c *Client) DeleteTagProtection(owner, repo string, id int64) (*Response, error) {
	sid := strconv.FormatInt(id, 10)
	if err := escapeValidatePathSegments(&owner, &repo, &sid); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_23_0); err != nil {
		return nil, err
	}

	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/repos/%s/%s/tag_protections/%d", owner, repo, id), jsonHeader, nil)
}
