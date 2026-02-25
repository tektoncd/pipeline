// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"io"
	"net/url"
)

// ContentsExtResponse contains extended information about a repo's contents
type ContentsExtResponse struct {
	DirContents  []*ContentsResponse `json:"dir_contents,omitempty"`
	FileContents *ContentsResponse   `json:"file_contents,omitempty"`
}

// GetContentsExtOptions options for getting extended contents
type GetContentsExtOptions struct {
	// The name of the commit/branch/tag. Default to the repository's default branch
	Ref string `json:"ref,omitempty"`
	// Comma-separated includes options: file_content, lfs_metadata, commit_metadata, commit_message
	Includes string `json:"includes,omitempty"`
}

// GetContentsExt gets extended file metadata and/or content from a repository
// The extended "contents" API, to get file metadata and/or content, or list a directory
func (c *Client) GetContentsExt(owner, repo, filepath string, opt GetContentsExtOptions) (*ContentsExtResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	// filepath doesn't need escaping since it's already part of the path
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/contents-ext/%s", owner, repo, filepath))
	query := link.Query()

	if opt.Ref != "" {
		query.Add("ref", opt.Ref)
	}
	if opt.Includes != "" {
		query.Add("includes", opt.Includes)
	}

	link.RawQuery = query.Encode()

	result := new(ContentsExtResponse)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, result)
	return result, resp, err
}

// GetEditorConfig gets the EditorConfig definitions of a file in a repository
func (c *Client) GetEditorConfig(owner, repo, filepath string, ref ...string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/editorconfig/%s", owner, repo, filepath))

	if len(ref) > 0 && ref[0] != "" {
		query := link.Query()
		query.Add("ref", ref[0])
		link.RawQuery = query.Encode()
	}

	resp, err := c.doRequest("GET", link.String(), nil, nil)
	if err != nil {
		return nil, resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err := io.ReadAll(resp.Body)
	return data, resp, err
}

// GetRawFileOrLFS gets a file or its LFS object from a repository
// This endpoint resolves LFS pointers and returns actual LFS objects
func (c *Client) GetRawFileOrLFS(owner, repo, filepath string, ref ...string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/media/%s", owner, repo, filepath))

	if len(ref) > 0 && ref[0] != "" {
		query := link.Query()
		query.Add("ref", ref[0])
		link.RawQuery = query.Encode()
	}

	resp, err := c.doRequest("GET", link.String(), nil, nil)
	if err != nil {
		return nil, resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err := io.ReadAll(resp.Body)
	return data, resp, err
}

// GetRawFile gets a file from a repository
// Unlike GetRawFileOrLFS, this does NOT resolve LFS pointers
func (c *Client) GetRawFile(owner, repo, filepath string, ref ...string) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/raw/%s", owner, repo, filepath))

	if len(ref) > 0 && ref[0] != "" {
		query := link.Query()
		query.Add("ref", ref[0])
		link.RawQuery = query.Encode()
	}

	resp, err := c.doRequest("GET", link.String(), nil, nil)
	if err != nil {
		return nil, resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err := io.ReadAll(resp.Body)
	return data, resp, err
}
