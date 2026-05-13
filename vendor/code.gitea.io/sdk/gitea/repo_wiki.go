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

// WikiPage represents a wiki page
type WikiPage struct {
	Title         string      `json:"title"`
	ContentBase64 string      `json:"content_base64"`
	CommitCount   int64       `json:"commit_count"`
	Sidebar       string      `json:"sidebar"`
	Footer        string      `json:"footer"`
	HTMLURL       string      `json:"html_url"`
	SubURL        string      `json:"sub_url"`
	LastCommit    *WikiCommit `json:"last_commit,omitempty"`
}

// WikiPageMetaData represents metadata for a wiki page (without content)
type WikiPageMetaData struct {
	Title      string      `json:"title"`
	HTMLURL    string      `json:"html_url"`
	SubURL     string      `json:"sub_url"`
	LastCommit *WikiCommit `json:"last_commit,omitempty"`
}

// WikiCommit represents a wiki commit/revision
type WikiCommit struct {
	ID       string      `json:"sha"`
	Message  string      `json:"message"`
	Author   *CommitUser `json:"author,omitempty"`
	Commiter *CommitUser `json:"commiter,omitempty"`
}

// WikiCommitList represents a list of wiki commits
type WikiCommitList struct {
	WikiCommits []*WikiCommit `json:"commits"`
	Count       int64         `json:"count"`
}

// CreateWikiPageOptions options for creating or editing a wiki page
type CreateWikiPageOptions struct {
	Title         string `json:"title,omitempty"`
	ContentBase64 string `json:"content_base64,omitempty"`
	Message       string `json:"message,omitempty"`
}

// ListWikiPagesOptions options for listing wiki pages
type ListWikiPagesOptions struct {
	ListOptions
}

// ListWikiPageRevisionsOptions options for listing wiki page revisions
type ListWikiPageRevisionsOptions struct {
	Page int `json:"page,omitempty"`
}

// CreateWikiPage creates a new wiki page
func (c *Client) CreateWikiPage(owner, repo string, opt CreateWikiPageOptions) (*WikiPage, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}

	page := new(WikiPage)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/wiki/new", owner, repo),
		jsonHeader, bytes.NewReader(body), &page)
	return page, resp, err
}

// GetWikiPage gets a wiki page by name
func (c *Client) GetWikiPage(owner, repo, pageName string) (*WikiPage, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &pageName); err != nil {
		return nil, nil, err
	}

	page := new(WikiPage)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/wiki/page/%s", owner, repo, pageName),
		jsonHeader, nil, &page)
	return page, resp, err
}

// EditWikiPage edits an existing wiki page
func (c *Client) EditWikiPage(owner, repo, pageName string, opt CreateWikiPageOptions) (*WikiPage, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &pageName); err != nil {
		return nil, nil, err
	}

	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}

	page := new(WikiPage)
	resp, err := c.getParsedResponse("PATCH",
		fmt.Sprintf("/repos/%s/%s/wiki/page/%s", owner, repo, pageName),
		jsonHeader, bytes.NewReader(body), &page)
	return page, resp, err
}

// DeleteWikiPage deletes a wiki page
func (c *Client) DeleteWikiPage(owner, repo, pageName string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &pageName); err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("DELETE",
		fmt.Sprintf("/repos/%s/%s/wiki/page/%s", owner, repo, pageName),
		jsonHeader, nil)
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// ListWikiPages lists all wiki pages in a repository
func (c *Client) ListWikiPages(owner, repo string, opt ListWikiPagesOptions) ([]*WikiPageMetaData, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/wiki/pages", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()

	pages := make([]*WikiPageMetaData, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &pages)
	return pages, resp, err
}

// GetWikiPageRevisions gets all revisions of a wiki page
func (c *Client) GetWikiPageRevisions(owner, repo, pageName string, opt ListWikiPageRevisionsOptions) (*WikiCommitList, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &pageName); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/wiki/revisions/%s", owner, repo, pageName))
	if opt.Page > 0 {
		query := link.Query()
		query.Add("page", fmt.Sprintf("%d", opt.Page))
		link.RawQuery = query.Encode()
	}

	commitList := new(WikiCommitList)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &commitList)
	return commitList, resp, err
}
