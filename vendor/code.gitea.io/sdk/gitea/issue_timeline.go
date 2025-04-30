// Copyright 2025 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"time"
)

// Comment represents a comment on a commit or issue
type TimelineComment struct {
	ID               int64      `json:"id"`
	HTMLURL          string     `json:"html_url"`
	PRURL            string     `json:"pull_request_url"`
	IssueURL         string     `json:"issue_url"`
	Poster           *User      `json:"user"`
	OriginalAuthor   string     `json:"original_author"`
	OriginalAuthorID int64      `json:"original_author_id"`
	Body             string     `json:"body"`
	Created          time.Time  `json:"created_at"`
	Updated          time.Time  `json:"updated_at"`
	Type             string     `json:"type"`
	Label            []*Label   `json:"label"`
	NewMilestone     *Milestone `json:"milestone"`
	OldMilestone     *Milestone `json:"old_milestone"`
	NewTitle         string     `json:"new_title"`
	OldTitle         string     `json:"old_title"`
}

// ListIssueTimeline list timeline on an issue.
func (c *Client) ListIssueTimeline(owner, repo string, index int64, opt ListIssueCommentOptions) ([]*TimelineComment, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/issues/%d/timeline", owner, repo, index))
	link.RawQuery = opt.QueryEncode()
	timelineComments := make([]*TimelineComment, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &timelineComments)
	return timelineComments, resp, err
}
