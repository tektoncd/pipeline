// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ActivityPub represents an ActivityPub object
type ActivityPub map[string]interface{}

// GetActivityPubPerson returns the Person actor for a user
func (c *Client) GetActivityPubPerson(userID int64) (ActivityPub, *Response, error) {
	result := make(ActivityPub)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/activitypub/user-id/%d", userID),
		jsonHeader, nil, &result)
	return result, resp, err
}

// SendActivityPubInbox sends an ActivityPub message to a user's inbox
func (c *Client) SendActivityPubInbox(userID int64, activity ActivityPub) (*Response, error) {
	body, err := json.Marshal(activity)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/activitypub/user-id/%d/inbox", userID),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// GetActivityPubPersonResponse returns the raw ActivityPub Person response
func (c *Client) GetActivityPubPersonResponse(userID int64) ([]byte, *Response, error) {
	resp, err := c.doRequest("GET",
		fmt.Sprintf("/activitypub/user-id/%d", userID),
		jsonHeader, nil)
	if err != nil {
		return nil, resp, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	return data, resp, err
}
