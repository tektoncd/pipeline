// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"net/url"
)

// ListAdminEmailsOptions options for listing all emails
type ListAdminEmailsOptions struct {
	ListOptions
}

// ListAdminEmails lists all email addresses
func (c *Client) ListAdminEmails(opt ListAdminEmailsOptions) ([]*Email, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/emails")
	link.RawQuery = opt.getURLQuery().Encode()

	emails := make([]*Email, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &emails)
	return emails, resp, err
}

// SearchAdminEmailsOptions options for searching emails
type SearchAdminEmailsOptions struct {
	ListOptions
	Query string `json:"q,omitempty"`
}

// SearchAdminEmails searches email addresses
func (c *Client) SearchAdminEmails(opt SearchAdminEmailsOptions) ([]*Email, *Response, error) {
	opt.setDefaults()

	link, _ := url.Parse("/admin/emails/search")
	query := opt.getURLQuery()
	if opt.Query != "" {
		query.Add("q", opt.Query)
	}
	link.RawQuery = query.Encode()

	emails := make([]*Email, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &emails)
	return emails, resp, err
}
