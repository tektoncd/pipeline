// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
)

// Note represents a git note
type Note struct {
	Message string  `json:"message"`
	Commit  *Commit `json:"commit"`
}

// GetRepoNoteOptions options for getting a note
type GetRepoNoteOptions struct {
	// include verification for every commit (disable for speedup, default 'true')
	Verification *bool `json:"verification,omitempty"`
	// include a list of affected files for every commit (disable for speedup, default 'true')
	Files *bool `json:"files,omitempty"`
}

// GetRepoNote gets a note corresponding to a single commit from a repository
func (c *Client) GetRepoNote(owner, repo, sha string, opt GetRepoNoteOptions) (*Note, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &sha); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/git/notes/%s", owner, repo, sha))
	query := link.Query()

	if opt.Verification != nil {
		if *opt.Verification {
			query.Add("verification", "true")
		} else {
			query.Add("verification", "false")
		}
	}

	if opt.Files != nil {
		if *opt.Files {
			query.Add("files", "true")
		} else {
			query.Add("files", "false")
		}
	}

	link.RawQuery = query.Encode()

	note := new(Note)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &note)
	return note, resp, err
}
