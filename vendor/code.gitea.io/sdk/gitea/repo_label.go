// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Label a label to an issue or a pr
type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// example: 00aabb
	Color       string `json:"color"`
	Description string `json:"description"`
	Exclusive   bool   `json:"exclusive"`
	URL         string `json:"url"`
}

// ListLabelsOptions options for listing repository's labels
type ListLabelsOptions struct {
	ListOptions
}

// ListRepoLabels list labels of one repository
func (c *Client) ListRepoLabels(owner, repo string, opt ListLabelsOptions) ([]*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	labels := make([]*Label, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/labels?%s", owner, repo, opt.getURLQuery().Encode()), nil, nil, &labels)
	return labels, resp, err
}

// GetRepoLabel get one label of repository by repo it
func (c *Client) GetRepoLabel(owner, repo string, id int64) (*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	label := new(Label)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), nil, nil, label)
	return label, resp, err
}

// CreateLabelOption options for creating a label
type CreateLabelOption struct {
	Name string `json:"name"`
	// example: #00aabb
	Color       string `json:"color"`
	Description string `json:"description"`
	Exclusive   bool   `json:"exclusive"`
}

// Validate the CreateLabelOption struct
func (opt CreateLabelOption) Validate() error {
	aw, err := regexp.MatchString("^#?[0-9,a-f,A-F]{6}$", opt.Color)
	if err != nil {
		return err
	}
	if !aw {
		return fmt.Errorf("invalid color format")
	}
	if len(strings.TrimSpace(opt.Name)) == 0 {
		return fmt.Errorf("empty name not allowed")
	}
	return nil
}

// CreateLabel create one label of repository
func (c *Client) CreateLabel(owner, repo string, opt CreateLabelOption) (*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	if len(opt.Color) == 6 {
		if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
			opt.Color = "#" + opt.Color
		}
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	label := new(Label)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/labels", owner, repo),
		jsonHeader, bytes.NewReader(body), label)
	return label, resp, err
}

// EditLabelOption options for editing a label
type EditLabelOption struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Description *string `json:"description"`
	Exclusive   *bool   `json:"exclusive"`
}

// Validate the EditLabelOption struct
func (opt EditLabelOption) Validate() error {
	if opt.Color != nil {
		aw, err := regexp.MatchString("^#?[0-9,a-f,A-F]{6}$", *opt.Color)
		if err != nil {
			return err
		}
		if !aw {
			return fmt.Errorf("invalid color format")
		}
	}
	if opt.Name != nil {
		if len(strings.TrimSpace(*opt.Name)) == 0 {
			return fmt.Errorf("empty name not allowed")
		}
	}
	return nil
}

// EditLabel modify one label with options
func (c *Client) EditLabel(owner, repo string, id int64, opt EditLabelOption) (*Label, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	label := new(Label)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), jsonHeader, bytes.NewReader(body), label)
	return label, resp, err
}

// DeleteLabel delete one label of repository by id
func (c *Client) DeleteLabel(owner, repo string, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/repos/%s/%s/labels/%d", owner, repo, id), nil, nil)
}
