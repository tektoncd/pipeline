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

// ListAdminHooksOptions options for listing admin hooks
type ListAdminHooksOptions struct {
	ListOptions
	// Type of hooks to list: system, default, or all
	Type string `json:"type,omitempty"`
}

// ListAdminHooks lists all system webhooks
func (c *Client) ListAdminHooks(opt ListAdminHooksOptions) ([]*Hook, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/hooks")
	query := opt.getURLQuery()
	if opt.Type != "" {
		query.Add("type", opt.Type)
	}
	link.RawQuery = query.Encode()

	hooks := make([]*Hook, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &hooks)
	return hooks, resp, err
}

// CreateAdminHook creates a system webhook
func (c *Client) CreateAdminHook(opt CreateHookOption) (*Hook, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	hook := new(Hook)
	resp, err := c.getParsedResponse("POST", "/admin/hooks", jsonHeader, bytes.NewReader(body), hook)
	return hook, resp, err
}

// GetAdminHook gets a system webhook by ID
func (c *Client) GetAdminHook(id int64) (*Hook, *Response, error) {
	hook := new(Hook)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/admin/hooks/%d", id), jsonHeader, nil, hook)
	return hook, resp, err
}

// EditAdminHook edits a system webhook
func (c *Client) EditAdminHook(id int64, opt EditHookOption) (*Hook, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	hook := new(Hook)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/admin/hooks/%d", id), jsonHeader, bytes.NewReader(body), hook)
	return hook, resp, err
}

// DeleteAdminHook deletes a system webhook
func (c *Client) DeleteAdminHook(id int64) (*Response, error) {
	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/admin/hooks/%d", id), jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
