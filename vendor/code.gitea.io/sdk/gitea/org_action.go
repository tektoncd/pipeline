// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// ListOrgActionVariableOption lists ActionVariable options
type ListOrgActionVariableOption struct {
	ListOptions
}

// ListOrgActionVariable lists an organization's action variables
func (c *Client) ListOrgActionVariable(org string, opt ListOrgActionVariableOption) ([]*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	variables := make([]*ActionVariable, 0, opt.PageSize)

	link, _ := url.Parse(fmt.Sprintf("/orgs/%s/actions/variables", org))
	link.RawQuery = opt.getURLQuery().Encode()
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetOrgActionVariable gets a single organization's action variable by name
func (c *Client) GetOrgActionVariable(org, name string) (*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, nil, err
	}
	var variable ActionVariable
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name),
		jsonHeader, nil, &variable)
	if err != nil {
		return nil, resp, err
	}
	return &variable, resp, nil
}

// CreateOrgActionVariable creates a variable for the specified organization in the Gitea Actions.
func (c *Client) CreateOrgActionVariable(org, name string, opt CreateActionVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("POST", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), jsonHeader, bytes.NewReader(body))
}

// UpdateOrgActionVariable updates a variable for the specified organization in the Gitea Actions.
func (c *Client) UpdateOrgActionVariable(org, name string, opt UpdateActionVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), jsonHeader, bytes.NewReader(body))
}

// CreateOrgActionSecret creates a secret for the specified organization in the Gitea Actions.
func (c *Client) CreateOrgActionSecret(org, secretName string, opt CreateOrUpdateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &secretName); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, secretName), jsonHeader, bytes.NewReader(body))
}

// DeleteOrgActionSecret deletes an organization's Actions secret.
func (c *Client) DeleteOrgActionSecret(org, secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &secretName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/orgs/%s/actions/secrets/%s", org, secretName), nil, nil)
}

// DeleteOrgActionVariable deletes an organization's Actions variable.
func (c *Client) DeleteOrgActionVariable(org, name string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &name); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/orgs/%s/actions/variables/%s", org, name), nil, nil)
}

// CreateOrgActionRunnerRegistrationToken creates an organization runner registration token.
func (c *Client) CreateOrgActionRunnerRegistrationToken(org string) (*RegistrationToken, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken(fmt.Sprintf("/orgs/%s/actions/runners/registration-token", org))
}

// ListOrgActionRunners lists organization-scoped Actions runners.
func (c *Client) ListOrgActionRunners(org string, opt ListActionRunnersOptions) (*ActionRunnersResponse, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners(fmt.Sprintf("/orgs/%s/actions/runners", org), opt)
}

// GetOrgActionRunner gets one organization-scoped Actions runner.
func (c *Client) GetOrgActionRunner(org string, runnerID int64) (*ActionRunner, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(fmt.Sprintf("/orgs/%s/actions/runners/%d", org, runnerID))
}

// DeleteOrgActionRunner deletes one organization-scoped Actions runner.
func (c *Client) DeleteOrgActionRunner(org string, runnerID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/orgs/%s/actions/runners/%d", org, runnerID), nil, nil)
}

// UpdateOrgActionRunner updates one organization-scoped Actions runner.
func (c *Client) UpdateOrgActionRunner(org string, runnerID int64, opt EditActionRunnerOption) (*ActionRunner, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(fmt.Sprintf("/orgs/%s/actions/runners/%d", org, runnerID), opt)
}

// ListOrgActionJobs lists organization-scoped Actions jobs.
func (c *Client) ListOrgActionJobs(org string, opt ListRepoActionJobsOptions) (*ActionWorkflowJobsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	return c.listActionJobs(fmt.Sprintf("/orgs/%s/actions/jobs", org), opt)
}

// ListOrgActionRuns lists organization-scoped Actions workflow runs.
func (c *Client) ListOrgActionRuns(org string, opt ListRepoActionRunsOptions) (*ActionWorkflowRunsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	return c.listActionRuns(fmt.Sprintf("/orgs/%s/actions/runs", org), opt)
}
