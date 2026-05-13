// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
)

// UserHeatmapData represents the data needed to create a heatmap
type UserHeatmapData struct {
	Timestamp     int64 `json:"timestamp"`
	Contributions int64 `json:"contributions"`
}

// GetUserHeatmap gets a user's heatmap data
func (c *Client) GetUserHeatmap(username string) ([]*UserHeatmapData, *Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, nil, err
	}

	heatmap := make([]*UserHeatmapData, 0, 365)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/users/%s/heatmap", username),
		jsonHeader, nil, &heatmap)
	return heatmap, resp, err
}

// ListUserActivityFeedsOptions options for listing user activity feeds
type ListUserActivityFeedsOptions struct {
	ListOptions
	OnlyPerformedBy bool   `json:"only-performed-by,omitempty"`
	Date            string `json:"date,omitempty"`
}

// ListUserActivityFeeds lists a user's activity feeds
func (c *Client) ListUserActivityFeeds(username string, opt ListUserActivityFeedsOptions) ([]*Activity, *Response, error) {
	if err := escapeValidatePathSegments(&username); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/users/%s/activities/feeds", username))
	query := opt.getURLQuery()
	if opt.OnlyPerformedBy {
		query.Add("only-performed-by", "true")
	}
	if opt.Date != "" {
		query.Add("date", opt.Date)
	}
	link.RawQuery = query.Encode()

	activities := make([]*Activity, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &activities)
	return activities, resp, err
}
