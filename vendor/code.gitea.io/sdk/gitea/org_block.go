// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
	"net/url"
)

// ListOrgBlocksOptions options for listing organization blocks
type ListOrgBlocksOptions struct {
	ListOptions
}

// ListOrgBlocks lists users blocked by the organization
func (c *Client) ListOrgBlocks(org string, opt ListOrgBlocksOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/blocks", org))
	link.RawQuery = opt.getURLQuery().Encode()

	users := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &users)
	return users, resp, err
}

// CheckOrgBlock checks if a user is blocked by the organization
func (c *Client) CheckOrgBlock(org, username string) (bool, *Response, error) {
	if err := escapeValidatePathSegments(&org, &username); err != nil {
		return false, nil, err
	}
	status, resp, err := c.getStatusCode("GET",
		fmt.Sprintf("/orgs/%s/blocks/%s", org, username),
		jsonHeader, nil)
	if err != nil {
		return false, resp, err
	}
	return status == http.StatusNoContent, resp, nil
}

// BlockOrgUser blocks a user from the organization
func (c *Client) BlockOrgUser(org, username string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &username); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("PUT",
		fmt.Sprintf("/orgs/%s/blocks/%s", org, username),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// UnblockOrgUser unblocks a user from the organization
func (c *Client) UnblockOrgUser(org, username string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &username); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE",
		fmt.Sprintf("/orgs/%s/blocks/%s", org, username),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
