// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
)

// IssueTemplate provides metadata and content on an issue template.
// There are two types of issue templates: .Markdown- and .Form-based.
type IssueTemplate struct {
	Name        string   `json:"name"`
	About       string   `json:"about"`
	Filename    string   `json:"file_name"`
	IssueTitle  string   `json:"title"`
	IssueLabels []string `json:"labels"`
	IssueRef    string   `json:"ref"`
	// If non-nil, this is a form-based template
	Form []IssueFormElement `json:"body"`
	// Should only be used when .Form is nil.
	MarkdownContent string `json:"content"`
}

// IssueFormElement describes a part of a IssueTemplate form
type IssueFormElement struct {
	ID          string                      `json:"id"`
	Type        IssueFormElementType        `json:"type"`
	Attributes  IssueFormElementAttributes  `json:"attributes"`
	Validations IssueFormElementValidations `json:"validations"`
}

// IssueFormElementAttributes contains the combined set of attributes available on all element types.
type IssueFormElementAttributes struct {
	// required for all element types.
	// A brief description of the expected user input, which is also displayed in the form.
	Label string `json:"label"`
	// required for element types "dropdown", "checkboxes"
	// for dropdown, contains the available options
	Options []string `json:"options"`
	// for element types "markdown", "textarea", "input"
	// Text that is pre-filled in the input
	Value string `json:"value"`
	// for element types "textarea", "input", "dropdown", "checkboxes"
	// A description of the text area to provide context or guidance, which is displayed in the form.
	Description string `json:"description"`
	// for element types "textarea", "input"
	// A semi-opaque placeholder that renders in the text area when empty.
	Placeholder string `json:"placeholder"`
	// for element types "textarea"
	// A language specifier. If set, the input is rendered as codeblock with syntax highlighting.
	SyntaxHighlighting string `json:"render"`
	// for element types "dropdown"
	Multiple bool `json:"multiple"`
}

// IssueFormElementValidations contains the combined set of validations available on all element types.
type IssueFormElementValidations struct {
	// for all element types
	Required bool `json:"required"`
	// for element types "input"
	IsNumber bool `json:"is_number"`
	// for element types "input"
	Regex string `json:"regex"`
}

// IssueFormElementType is an enum
type IssueFormElementType string

const (
	// IssueFormElementMarkdown is markdown rendered to the form for context, but omitted in the resulting issue
	IssueFormElementMarkdown IssueFormElementType = "markdown"
	// IssueFormElementTextarea is a multi line input
	IssueFormElementTextarea IssueFormElementType = "textarea"
	// IssueFormElementInput is a single line input
	IssueFormElementInput IssueFormElementType = "input"
	// IssueFormElementDropdown is a select form
	IssueFormElementDropdown IssueFormElementType = "dropdown"
	// IssueFormElementCheckboxes are a multi checkbox input
	IssueFormElementCheckboxes IssueFormElementType = "checkboxes"
)

// GetIssueTemplates lists all issue templates of the repository
func (c *Client) GetIssueTemplates(owner, repo string) ([]*IssueTemplate, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	templates := new([]*IssueTemplate)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issue_templates", owner, repo), nil, nil, templates)
	return *templates, resp, err
}

// IsForm tells if this template is a form instead of a markdown-based template.
func (t IssueTemplate) IsForm() bool {
	return t.Form != nil
}
