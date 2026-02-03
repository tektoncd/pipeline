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

type OrgActionVariable struct {
	OwnerID     int64  `json:"owner_id"`
	RepoID      int64  `json:"repo_id"`
	Name        string `json:"name"`
	Data        string `json:"data"`
	Description string `json:"description"`
}

// ListOrgActionVariableOption lists OrgActionVariable options
type ListOrgActionVariableOption struct {
	ListOptions
}

// ListOrgActionVariable lists an organization's action variables
func (c *Client) ListOrgActionVariable(org string, opt ListOrgActionVariableOption) ([]*OrgActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	variables := make([]*OrgActionVariable, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/variables", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetOrgActionVariable gets a single organization's action variable by name
func (c *Client) GetOrgActionVariable(org, name string) (*OrgActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, nil, err
	}
	var variable OrgActionVariable
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name),
		jsonHeader, nil, &variable)
	if err != nil {
		return nil, resp, err
	}
	return &variable, resp, nil
}

// CreateOrgActionVariableOption represents the options for creating an org action variable.
type CreateOrgActionVariableOption struct {
	Name        string `json:"name"`        // Name is the name of the variable.
	Value       string `json:"value"`       // Value is the value of the variable.
	Description string `json:"description"` // Description is the description of the variable.
}

// Validate checks if the CreateOrgActionVariableOption is valid.
func (opt *CreateOrgActionVariableOption) Validate() error {
	if len(opt.Name) == 0 {
		return fmt.Errorf("name required")
	}
	if len(opt.Name) > 30 {
		return fmt.Errorf("name too long")
	}
	if len(opt.Value) == 0 {
		return fmt.Errorf("value required")
	}
	return nil
}

// CreateOrgActionVariable creates a variable for the specified organization in the Gitea Actions.
func (c *Client) CreateOrgActionVariable(org string, opt CreateOrgActionVariableOption) (*Response, error) {
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

	status, resp, err := c.getStatusCode("POST", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, opt.Name), jsonHeader, bytes.NewReader(body))
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

// UpdateOrgActionVariableOption represents the options for updating an org action variable.
type UpdateOrgActionVariableOption struct {
	Value       string `json:"value"`       // Value is the new value of the variable.
	Description string `json:"description"` // Description is the new description of the variable.
}

// Validate checks if the UpdateOrgActionVariableOption is valid.
func (opt *UpdateOrgActionVariableOption) Validate() error {
	if len(opt.Value) == 0 {
		return fmt.Errorf("value required")
	}
	return nil
}

// UpdateOrgActionVariable updates a variable for the specified organization in the Gitea Actions.
func (c *Client) UpdateOrgActionVariable(org, name string, opt UpdateOrgActionVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), jsonHeader, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	switch status {
	case http.StatusOK:
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

// CreateSecretOption represents the options for creating a secret.
type CreateSecretOption struct {
	Name        string `json:"name"`        // Name is the name of the secret.
	Data        string `json:"data"`        // Data is the data of the secret.
	Description string `json:"description"` // Description is the description of the secret.
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
