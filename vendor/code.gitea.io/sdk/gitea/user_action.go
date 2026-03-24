// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// CreateUserActionSecret creates or updates a user-scope Actions secret.
func (c *Client) CreateUserActionSecret(secretName string, opt CreateOrUpdateSecretOption) (*Response, error) {
	if err := escapeValidatePathSegments(&secretName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_21_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/user/actions/secrets/%s", secretName), jsonHeader, bytes.NewReader(body))
}

// DeleteUserActionSecret deletes a user-scope Actions secret.
func (c *Client) DeleteUserActionSecret(secretName string) (*Response, error) {
	if err := escapeValidatePathSegments(&secretName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_21_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/user/actions/secrets/%s", secretName), nil, nil)
}

// ListUserActionVariable lists user-scope Actions variables.
func (c *Client) ListUserActionVariable(opt ListOptions) ([]*ActionVariable, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	variables := make([]*ActionVariable, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/actions/variables?%s", opt.getURLQuery().Encode()), jsonHeader, nil, &variables)
	return variables, resp, err
}

// GetUserActionVariable gets one user-scope Actions variable.
func (c *Client) GetUserActionVariable(variableName string) (*ActionVariable, *Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	variable := new(ActionVariable)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, nil, variable)
	return variable, resp, err
}

// CreateUserActionVariable creates one user-scope Actions variable.
func (c *Client) CreateUserActionVariable(variableName string, opt CreateActionVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("POST", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, bytes.NewReader(body))
}

// UpdateUserActionVariable updates one user-scope Actions variable.
func (c *Client) UpdateUserActionVariable(variableName string, opt UpdateActionVariableOption) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/user/actions/variables/%s", variableName), jsonHeader, bytes.NewReader(body))
}

// DeleteUserActionVariable deletes one user-scope Actions variable.
func (c *Client) DeleteUserActionVariable(variableName string) (*Response, error) {
	if err := escapeValidatePathSegments(&variableName); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/user/actions/variables/%s", variableName), nil, nil)
}

// ListUserActionRuns lists user-scope Actions workflow runs.
func (c *Client) ListUserActionRuns(opt ListRepoActionRunsOptions) (*ActionWorkflowRunsResponse, *Response, error) {
	return c.listActionRuns("/user/actions/runs", opt)
}

// ListUserActionJobs lists user-scope Actions jobs.
func (c *Client) ListUserActionJobs(opt ListRepoActionJobsOptions) (*ActionWorkflowJobsResponse, *Response, error) {
	return c.listActionJobs("/user/actions/jobs", opt)
}

// CreateUserActionRunnerRegistrationToken creates a user-scope runner registration token.
func (c *Client) CreateUserActionRunnerRegistrationToken() (*RegistrationToken, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken("/user/actions/runners/registration-token")
}

// ListUserActionRunners lists user-scope Actions runners.
func (c *Client) ListUserActionRunners(opt ListActionRunnersOptions) (*ActionRunnersResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners("/user/actions/runners", opt)
}

// GetUserActionRunner gets one user-scope Actions runner.
func (c *Client) GetUserActionRunner(runnerID int64) (*ActionRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(fmt.Sprintf("/user/actions/runners/%d", runnerID))
}

// DeleteUserActionRunner deletes one user-scope Actions runner.
func (c *Client) DeleteUserActionRunner(runnerID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/user/actions/runners/%d", runnerID), nil, nil)
}

// UpdateUserActionRunner updates one user-scope Actions runner.
func (c *Client) UpdateUserActionRunner(runnerID int64, opt EditActionRunnerOption) (*ActionRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(fmt.Sprintf("/user/actions/runners/%d", runnerID), opt)
}
