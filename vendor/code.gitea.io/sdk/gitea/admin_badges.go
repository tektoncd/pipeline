// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Badge represents a user badge
type Badge struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

// ListUserBadges lists badges of a user
func (c *Client) ListUserBadges(username string) ([]*Badge, *Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, nil, err
	}
	badges := make([]*Badge, 0, 5)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/admin/users/%s/badges", username),
		jsonHeader, nil, &badges)
	return badges, resp, err
}

// UserBadgeOption represents options for adding badges to a user
type UserBadgeOption struct {
	BadgeSlugs []string `json:"badge_slugs"`
}

// AddUserBadges adds badges to a user by their slugs
func (c *Client) AddUserBadges(username string, opt UserBadgeOption) (*Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/admin/users/%s/badges", username),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent && status != http.StatusCreated {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// DeleteUserBadge deletes a user's badge
func (c *Client) DeleteUserBadge(username string, opt UserBadgeOption) (*Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE",
		fmt.Sprintf("/admin/users/%s/badges", username),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
