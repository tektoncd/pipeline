// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// CreateRepoActionRunnerRegistrationToken creates a repository-scope runner registration token.
func (c *Client) CreateRepoActionRunnerRegistrationToken(owner, repo string) (*RegistrationToken, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_22_0); err != nil {
		return nil, nil, err
	}
	return c.createActionRegistrationToken(fmt.Sprintf("/repos/%s/%s/actions/runners/registration-token", owner, repo))
}

// ListRepoActionRunners lists repository-scope Actions runners.
func (c *Client) ListRepoActionRunners(owner, repo string, opt ListActionRunnersOptions) (*ActionRunnersResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionRunners(fmt.Sprintf("/repos/%s/%s/actions/runners", owner, repo), opt)
}

// GetRepoActionRunner gets one repository-scope Actions runner.
func (c *Client) GetRepoActionRunner(owner, repo string, runnerID int64) (*ActionRunner, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getActionRunner(fmt.Sprintf("/repos/%s/%s/actions/runners/%d", owner, repo, runnerID))
}

// DeleteRepoActionRunner deletes one repository-scope Actions runner.
func (c *Client) DeleteRepoActionRunner(owner, repo string, runnerID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/repos/%s/%s/actions/runners/%d", owner, repo, runnerID), nil, nil)
}

// UpdateRepoActionRunner updates one repository-scope Actions runner.
func (c *Client) UpdateRepoActionRunner(owner, repo string, runnerID int64, opt EditActionRunnerOption) (*ActionRunner, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.updateActionRunner(fmt.Sprintf("/repos/%s/%s/actions/runners/%d", owner, repo, runnerID), opt)
}

// ListRepoActionWorkflows lists repository workflows.
func (c *Client) ListRepoActionWorkflows(owner, repo string) (*ActionWorkflowResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	workflows := new(ActionWorkflowResponse)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/workflows", owner, repo), jsonHeader, nil, workflows)
	return workflows, resp, err
}

// GetRepoActionWorkflow gets one repository workflow.
func (c *Client) GetRepoActionWorkflow(owner, repo, workflowID string) (*ActionWorkflow, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	workflow := new(ActionWorkflow)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s", owner, repo, workflowID), jsonHeader, nil, workflow)
	return workflow, resp, err
}

// DisableRepoActionWorkflow disables one repository workflow.
func (c *Client) DisableRepoActionWorkflow(owner, repo, workflowID string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/disable", owner, repo, workflowID), nil, nil)
}

// EnableRepoActionWorkflow enables one repository workflow.
func (c *Client) EnableRepoActionWorkflow(owner, repo, workflowID string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/enable", owner, repo, workflowID), nil, nil)
}

// DispatchRepoActionWorkflow dispatches one repository workflow.
func (c *Client) DispatchRepoActionWorkflow(owner, repo, workflowID string, opt CreateActionWorkflowDispatchOption, returnRunDetails bool) (*RunDetails, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &workflowID); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflowID))
	if returnRunDetails {
		link.RawQuery = url.Values{"return_run_details": []string{"true"}}.Encode()
		details := new(RunDetails)
		resp, err := c.getParsedResponse("POST", link.String(), jsonHeader, bytes.NewReader(body), details)
		return details, resp, err
	}

	resp, err := c.doRequestWithStatusHandle("POST", link.String(), jsonHeader, bytes.NewReader(body))
	return nil, resp, err
}

// ListRepoActionRunArtifacts lists artifacts for one workflow run.
func (c *Client) ListRepoActionRunArtifacts(owner, repo string, runID int64, opt ListActionArtifactsOptions) (*ActionArtifactsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionArtifacts(fmt.Sprintf("/repos/%s/%s/actions/runs/%d/artifacts", owner, repo, runID), opt)
}

// ListRepoActionArtifacts lists repository artifacts.
func (c *Client) ListRepoActionArtifacts(owner, repo string, opt ListActionArtifactsOptions) (*ActionArtifactsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.listActionArtifacts(fmt.Sprintf("/repos/%s/%s/actions/artifacts", owner, repo), opt)
}

// GetRepoActionArtifact gets one repository artifact.
func (c *Client) GetRepoActionArtifact(owner, repo string, artifactID int64) (*ActionArtifact, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	artifact := new(ActionArtifact)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d", owner, repo, artifactID), jsonHeader, nil, artifact)
	return artifact, resp, err
}

// DeleteRepoActionArtifact deletes one repository artifact.
func (c *Client) DeleteRepoActionArtifact(owner, repo string, artifactID int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d", owner, repo, artifactID), nil, nil)
}

// GetRepoActionArtifactArchive downloads one repository artifact zip archive.
func (c *Client) GetRepoActionArtifactArchive(owner, repo string, artifactID int64) ([]byte, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d/zip", owner, repo, artifactID), nil, nil)
}

// GetRepoActionArtifactArchiveReader returns a reader for one repository artifact zip archive.
func (c *Client) GetRepoActionArtifactArchiveReader(owner, repo string, artifactID int64) (io.ReadCloser, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	return c.getResponseReader("GET", fmt.Sprintf("/repos/%s/%s/actions/artifacts/%d/zip", owner, repo, artifactID), nil, nil)
}
