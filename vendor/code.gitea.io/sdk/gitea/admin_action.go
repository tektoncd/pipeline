// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import "fmt"

// ListAdminActionJobs lists all admin-scope Actions jobs.
func (c *Client) ListAdminActionJobs(opt ListRepoActionJobsOptions) (*ActionWorkflowJobsResponse, *Response, error) {
	return c.listActionJobs("/admin/actions/jobs", opt)
}

// ListAdminActionRuns lists all admin-scope Actions workflow runs.
func (c *Client) ListAdminActionRuns(opt ListRepoActionRunsOptions) (*ActionWorkflowRunsResponse, *Response, error) {
	return c.listActionRuns("/admin/actions/runs", opt)
}

// ListAdminActionRunners lists all admin-scope Actions runners.
func (c *Client) ListAdminActionRunners(opt ListActionRunnersOptions) (*ActionRunnersResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners("/admin/actions/runners", opt)
}

// GetAdminActionRunner gets one admin-scope Actions runner.
func (c *Client) GetAdminActionRunner(runnerID int64) (*ActionRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(fmt.Sprintf("/admin/actions/runners/%d", runnerID))
}

// DeleteAdminActionRunner deletes one admin-scope Actions runner.
func (c *Client) DeleteAdminActionRunner(runnerID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/admin/actions/runners/%d", runnerID), nil, nil)
}

// UpdateAdminActionRunner updates one admin-scope Actions runner.
func (c *Client) UpdateAdminActionRunner(runnerID int64, opt EditActionRunnerOption) (*ActionRunner, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(fmt.Sprintf("/admin/actions/runners/%d", runnerID), opt)
}

// CreateAdminActionRunnerRegistrationToken creates an admin-scope runner registration token.
func (c *Client) CreateAdminActionRunnerRegistrationToken() (*RegistrationToken, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken("/admin/actions/runners/registration-token")
}
