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

// UpdateUserAvatarOption options for updating user avatar
type UpdateUserAvatarOption struct {
	Image string `json:"image"` // base64 encoded image
}

// UpdateUserAvatar updates the authenticated user's avatar
func (c *Client) UpdateUserAvatar(opt UpdateUserAvatarOption) (*Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST", "/user/avatar",
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// DeleteUserAvatar deletes the authenticated user's avatar
func (c *Client) DeleteUserAvatar() (*Response, error) {
	status, resp, err := c.getStatusCode("DELETE", "/user/avatar", jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}
