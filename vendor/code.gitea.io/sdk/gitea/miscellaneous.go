// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// GitignoreTemplateInfo represents a gitignore template
type GitignoreTemplateInfo struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

// LabelTemplate represents a label template
type LabelTemplate struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Exclusive   bool   `json:"exclusive"`
}

// LicensesTemplateListEntry represents a license in the list
type LicensesTemplateListEntry struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// LicenseTemplateInfo represents a license template
type LicenseTemplateInfo struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	URL            string `json:"url"`
	Body           string `json:"body"`
	Implementation string `json:"implementation"`
}

// MarkdownOption represents options for rendering markdown
type MarkdownOption struct {
	Text    string `json:"Text"`
	Mode    string `json:"Mode"`
	Context string `json:"Context"`
	Wiki    bool   `json:"Wiki"`
}

// MarkupOption represents options for rendering markup
type MarkupOption struct {
	Text     string `json:"Text"`
	Mode     string `json:"Mode"`
	Context  string `json:"Context"`
	FilePath string `json:"FilePath"`
	Wiki     bool   `json:"Wiki"`
}

// NodeInfo represents nodeinfo about the server
type NodeInfo struct {
	Version           string                 `json:"version"`
	Software          NodeInfoSoftware       `json:"software"`
	Protocols         []string               `json:"protocols"`
	Services          NodeInfoServices       `json:"services"`
	OpenRegistrations bool                   `json:"openRegistrations"`
	Usage             NodeInfoUsage          `json:"usage"`
	Metadata          map[string]interface{} `json:"metadata"`
}

// NodeInfoSoftware represents software information
type NodeInfoSoftware struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Homepage   string `json:"homepage"`
}

// NodeInfoServices represents third party services
type NodeInfoServices struct {
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

// NodeInfoUsage represents usage statistics
type NodeInfoUsage struct {
	Users         NodeInfoUsageUsers `json:"users"`
	LocalPosts    int64              `json:"localPosts"`
	LocalComments int64              `json:"localComments"`
}

// NodeInfoUsageUsers represents user statistics
type NodeInfoUsageUsers struct {
	Total          int64 `json:"total"`
	ActiveHalfyear int64 `json:"activeHalfyear"`
	ActiveMonth    int64 `json:"activeMonth"`
}

// ListGitignoresTemplates lists all gitignore templates
func (c *Client) ListGitignoresTemplates() ([]string, *Response, error) {
	templates := make([]string, 0, 10)
	resp, err := c.getParsedResponse("GET", "/gitignore/templates", jsonHeader, nil, &templates)
	return templates, resp, err
}

// GetGitignoreTemplateInfo gets information about a gitignore template
func (c *Client) GetGitignoreTemplateInfo(name string) (*GitignoreTemplateInfo, *Response, error) {
	if err := escapeValidatePathSegments(&name); err != nil {
		return nil, nil, err
	}
	template := new(GitignoreTemplateInfo)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/gitignore/templates/%s", name),
		jsonHeader, nil, &template)
	return template, resp, err
}

// ListLabelTemplates lists all label templates
func (c *Client) ListLabelTemplates() ([]string, *Response, error) {
	templates := make([]string, 0, 10)
	resp, err := c.getParsedResponse("GET", "/label/templates", jsonHeader, nil, &templates)
	return templates, resp, err
}

// GetLabelTemplate gets all labels in a template
func (c *Client) GetLabelTemplate(name string) ([]*LabelTemplate, *Response, error) {
	if err := escapeValidatePathSegments(&name); err != nil {
		return nil, nil, err
	}
	labels := make([]*LabelTemplate, 0, 10)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/label/templates/%s", name),
		jsonHeader, nil, &labels)
	return labels, resp, err
}

// ListLicenseTemplates lists all license templates
func (c *Client) ListLicenseTemplates() ([]*LicensesTemplateListEntry, *Response, error) {
	licenses := make([]*LicensesTemplateListEntry, 0, 10)
	resp, err := c.getParsedResponse("GET", "/licenses", jsonHeader, nil, &licenses)
	return licenses, resp, err
}

// GetLicenseTemplateInfo gets information about a license template
func (c *Client) GetLicenseTemplateInfo(name string) (*LicenseTemplateInfo, *Response, error) {
	if err := escapeValidatePathSegments(&name); err != nil {
		return nil, nil, err
	}
	license := new(LicenseTemplateInfo)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/licenses/%s", name),
		jsonHeader, nil, &license)
	return license, resp, err
}

// RenderMarkdown renders a markdown document as HTML
func (c *Client) RenderMarkdown(opt MarkdownOption) (string, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return "", nil, err
	}

	resp, err := c.doRequest("POST", "/markdown", jsonHeader, bytes.NewReader(body))
	if err != nil {
		return "", resp, err
	}
	defer func() { _ = resp.Body.Close() }()

	html, err := io.ReadAll(resp.Body)
	return string(html), resp, err
}

// RenderMarkdownRaw renders raw markdown as HTML
func (c *Client) RenderMarkdownRaw(markdown string) (string, *Response, error) {
	resp, err := c.doRequest("POST", "/markdown/raw",
		map[string][]string{"Content-Type": {"text/plain"}},
		bytes.NewReader([]byte(markdown)))
	if err != nil {
		return "", resp, err
	}
	defer func() { _ = resp.Body.Close() }()

	html, err := io.ReadAll(resp.Body)
	return string(html), resp, err
}

// RenderMarkup renders a markup document as HTML
func (c *Client) RenderMarkup(opt MarkupOption) (string, *Response, error) {
	body, err := json.Marshal(&opt)
	if err != nil {
		return "", nil, err
	}

	resp, err := c.doRequest("POST", "/markup", jsonHeader, bytes.NewReader(body))
	if err != nil {
		return "", resp, err
	}
	defer func() { _ = resp.Body.Close() }()

	html, err := io.ReadAll(resp.Body)
	return string(html), resp, err
}

// GetNodeInfo gets the nodeinfo of the Gitea application
func (c *Client) GetNodeInfo() (*NodeInfo, *Response, error) {
	nodeInfo := new(NodeInfo)
	resp, err := c.getParsedResponse("GET", "/nodeinfo", jsonHeader, nil, &nodeInfo)
	return nodeInfo, resp, err
}

// GetSigningKeyGPG gets the default GPG signing key
func (c *Client) GetSigningKeyGPG() (string, *Response, error) {
	resp, err := c.doRequest("GET", "/signing-key.gpg", nil, nil)
	if err != nil {
		return "", resp, err
	}
	defer func() { _ = resp.Body.Close() }()

	key, err := io.ReadAll(resp.Body)
	return string(key), resp, err
}

// GetSigningKeySSH gets the default SSH signing key
func (c *Client) GetSigningKeySSH() (string, *Response, error) {
	resp, err := c.doRequest("GET", "/signing-key.pub", nil, nil)
	if err != nil {
		return "", resp, err
	}
	defer func() { _ = resp.Body.Close() }()

	key, err := io.ReadAll(resp.Body)
	return string(key), resp, err
}
