// Copyright 2023 The Gitea Authors. All rights reserved.
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

// ListOrgActionSecretOption list OrgActionSecret options
type ListOrgActionSecretOption struct {
	ListOptions
}

// ListOrgActionSecret list an organization's secrets
func (c *Client) ListOrgActionSecret(org string, opt ListOrgActionSecretOption) ([]*Secret, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	secrets := make([]*Secret, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/secrets", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &secrets)
	return secrets, resp, err
}

// CreateSecretOption represents the options for creating a secret.
type CreateSecretOption struct {
	Name string `json:"name"` // Name is the name of the secret.
	Data string `json:"data"` // Data is the data of the secret.
}

// Validate checks if the CreateSecretOption is valid.
// It returns an error if any of the validation checks fail.
func (opt *CreateSecretOption) Validate() error {
	if len(opt.Name) == 0 {
		return fmt.Errorf("name required")
	}
	if len(opt.Name) > 30 {
		return fmt.Errorf("name to long")
	}
	if len(opt.Data) == 0 {
		return fmt.Errorf("data required")
	}
	return nil
}

// CreateOrgActionSecret creates a secret for the specified organization in the Gitea Actions.
// It takes the organization name and the secret options as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) CreateOrgActionSecret(org string, opt CreateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, opt.Name), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusCreated:
		return resp, nil
	case http.StatusNoContent:
		return resp, nil
	case http.StatusNotFound:
		return resp, fmt.Errorf("forbidden")
	case http.StatusBadRequest:
		return resp, fmt.Errorf("bad request")
	default:
		return resp, fmt.Errorf("unexpected Status: %d", status)
	}
}
