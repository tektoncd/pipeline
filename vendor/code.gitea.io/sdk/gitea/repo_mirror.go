// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type CreatePushMirrorOption struct {
	Interval       string `json:"interval"`
	RemoteAddress  string `json:"remote_address"`
	RemotePassword string `json:"remote_password"`
	RemoteUsername string `json:"remote_username"`
	SyncONCommit   bool   `json:"sync_on_commit"`
}

// PushMirrorResponse returns a git push mirror
type PushMirrorResponse struct {
	Created       string `json:"created"`
	Interval      string `json:"interval"`
	LastError     string `json:"last_error"`
	LastUpdate    string `json:"last_update"`
	RemoteAddress string `json:"remote_address"`
	RemoteName    string `json:"remote_name"`
	RepoName      string `json:"repo_name"`
	SyncONCommit  bool   `json:"sync_on_commit"`
}

// PushMirrors add a push mirror to the repository
func (c *Client) PushMirrors(user, repo string, opt CreatePushMirrorOption) (*PushMirrorResponse, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(opt)
	if err != nil {
		return nil, nil, err
	}
	pm := new(PushMirrorResponse)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/push_mirrors", user, repo), jsonHeader, bytes.NewReader(body), &pm)
	return pm, resp, err
}
