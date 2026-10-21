// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ListOrgLabelsOptions options for listing organization labels
type ListOrgLabelsOptions struct {
	ListOptions
}

// ListOrgLabels returns the labels defined at the org level
func (c *Client) ListOrgLabels(orgName string, opt ListOrgLabelsOptions) ([]*Label, *Response, error) {
	if err := escapeValidatePathSegments(&orgName); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	labels := make([]*Label, 0, opt.PageSize)
	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/labels", orgName))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &labels)
	return labels, resp, err
}

type CreateOrgLabelOption struct {
	// Name of the label
	Name string `json:"name"`
	// Color of the label in hex format without #
	Color string `json:"color"`
	// Description of the label
	Description string `json:"description"`
	// Whether this is an exclusive label
	Exclusive bool `json:"exclusive"`
}

// Validate the CreateLabelOption struct
func (opt CreateOrgLabelOption) Validate() error {
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

// CreateOrgLabel creates a new label under an organization
func (c *Client) CreateOrgLabel(orgName string, opt CreateOrgLabelOption) (*Label, *Response, error) {
	if err := escapeValidatePathSegments(&orgName); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	label := new(Label)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/orgs/%s/labels", orgName), jsonHeader, bytes.NewReader(body), label)
	return label, resp, err
}

// GetOrgLabel get one label of organization by org it
func (c *Client) GetOrgLabel(orgName string, labelID int64) (*Label, *Response, error) {
	if err := escapeValidatePathSegments(&orgName); err != nil {
		return nil, nil, err
	}
	label := new(Label)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/labels/%d", orgName, labelID), nil, nil, label)
	return label, resp, err
}

type EditOrgLabelOption struct {
	// New name of the label
	Name *string `json:"name"`
	// New color of the label in hex format without #
	Color *string `json:"color"`
	// New description of the label
	Description *string `json:"description"`
	// Whether this is an exclusive label
	Exclusive *bool `json:"exclusive,omitempty"`
}

// EditOrgLabel edits an existing org-level label by ID
func (c *Client) EditOrgLabel(orgName string, labelID int64, opt EditOrgLabelOption) (*Label, *Response, error) {
	if err := escapeValidatePathSegments(&orgName); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	label := new(Label)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/orgs/%s/labels/%d", orgName, labelID), jsonHeader, bytes.NewReader(body), label)
	return label, resp, err
}

// DeleteOrgLabel deletes a org label by ID
func (c *Client) DeleteOrgLabel(orgName string, labelID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&orgName); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/orgs/%s/labels/%d", orgName, labelID), jsonHeader, nil)
	return resp, err
}
