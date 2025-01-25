// Copyright 2024 The Gitea Authors. All rights reserved.
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

// ListRepoActionSecretOption list RepoActionSecret options
type ListRepoActionSecretOption struct {
	ListOptions
}

// CreateActionsVariable represents body for creating a action variable.
type CreateRepoActionsVariable struct {
	Value string `json:"value"`
}

// PutActionsVariable represents body for updating a action variable.
type PutRepoActionsVariable struct {
	Value string `json:"value"`
	Name  string `json:"name"`
}

// ListRepoActionSecret list a repository's secrets
func (c *Client) ListRepoActionSecret(user, repo string, opt ListRepoActionSecretOption) ([]*Secret, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	secrets := make([]*Secret, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/secrets", user, repo))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &secrets)
	return secrets, resp, err
}

// CreateRepoActionSecret creates a secret for the specified repository in the Gitea Actions.
// It takes the organization name and the secret options as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) CreateRepoActionSecret(user, repo string, opt CreateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}

	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", user, repo, opt.Name), jsonHeader, bytes.NewReader(body))
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

// DeleteRepoActionSecret deletes a secret from the Gitea Actions.
// It takes the repository owner, name and the secret name as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) DeleteRepoActionSecret(user, repo, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}

	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/actions/secrets/%s", user, repo, secretName), nil, nil)
	return resp, err
}

// GetRepoActionVariable returns a repository variable in the Gitea Actions.
// It takes the repository owner, name and the variable name as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) GetRepoActionVariable(user, repo, variableName string) (*RepoActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	variable := new(RepoActionVariable)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, repo, variableName), nil, nil, variable)
	return variable, resp, err
}

// CreateRepoActionVariable creates a repository variable in the Gitea Actions.
// It takes the repository owner, name, variable name and the variable value as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) CreateRepoActionVariable(user, repo, variableName, value string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}

	create := CreateRepoActionsVariable{
		Value: value,
	}

	body, err := json.Marshal(&create)
	if err != nil {
		return nil, err
	}

	_, resp, err := c.getResponse("POST", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, repo, variableName), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// UpdateRepoActionVariable updates a repository variable in the Gitea Actions.
// It takes the repository owner, name, variable name and the variable value as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) UpdateRepoActionVariable(user, repo, variableName, value string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}

	update := PutRepoActionsVariable{
		Value: value,
		Name:  variableName,
	}

	body, err := json.Marshal(&update)
	if err != nil {
		return nil, err
	}

	_, resp, err := c.getResponse("PUT", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, repo, variableName), jsonHeader, bytes.NewReader(body))
	return resp, err
}

// DeleteRepoActionVariable deletes a repository variable in the Gitea Actions.
// It takes the repository owner, name and the variable name as parameters.
// The function returns the HTTP response and an error, if any.
func (c *Client) DeleteRepoActionVariable(user, reponame, variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &reponame); err != nil {
		return nil, err
	}

	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/actions/variables/%s", user, reponame, variableName), nil, nil)
	return resp, err
}
