// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// RegistrationToken is returned when creating an Actions runner registration token.
type RegistrationToken struct {
	Token string `json:"token"`
}

// CreateOrUpdateSecretOption contains the data for creating or updating an Actions secret.
type CreateOrUpdateSecretOption struct {
	Data        string `json:"data"`
	Description string `json:"description"`
}

// Validate checks whether a secret payload can be sent to the API.
func (opt CreateOrUpdateSecretOption) Validate() error {
	if len(opt.Data) == 0 {
		return errors.New("empty Data field")
	}
	return nil
}

// ActionVariable represents an Actions variable.
type ActionVariable struct {
	OwnerID     int64  `json:"owner_id"`
	RepoID      int64  `json:"repo_id"`
	Name        string `json:"name"`
	Data        string `json:"data"`
	Description string `json:"description"`
}

// CreateActionVariableOption is used to create an Actions variable.
type CreateActionVariableOption struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

// Validate checks whether the variable create payload is valid.
func (opt CreateActionVariableOption) Validate() error {
	if len(opt.Value) == 0 {
		return errors.New("empty Value field")
	}
	return nil
}

// UpdateActionVariableOption is used to update an Actions variable.
type UpdateActionVariableOption struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// Validate checks whether the variable update payload is valid.
func (opt UpdateActionVariableOption) Validate() error {
	if len(opt.Value) == 0 {
		return errors.New("empty Value field")
	}
	return nil
}

// ListActionRunnersOptions controls runner listing requests.
type ListActionRunnersOptions struct {
	ListOptions
	Disabled *bool
}

// QueryEncode turns the runner list options into a query string.
func (opt *ListActionRunnersOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Disabled != nil {
		query.Add("disabled", fmt.Sprintf("%t", *opt.Disabled))
	}
	return query.Encode()
}

// ActionRunnerLabel represents a runner label.
type ActionRunnerLabel struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ActionRunner represents an Actions runner.
type ActionRunner struct {
	ID        int64                `json:"id"`
	Name      string               `json:"name"`
	Status    string               `json:"status"`
	Busy      bool                 `json:"busy"`
	Disabled  bool                 `json:"disabled"`
	Ephemeral bool                 `json:"ephemeral"`
	Labels    []*ActionRunnerLabel `json:"labels"`
}

// EditActionRunnerOption contains editable runner fields.
type EditActionRunnerOption struct {
	Disabled *bool `json:"disabled"`
}

// Validate checks whether the runner update payload is valid.
func (opt EditActionRunnerOption) Validate() error {
	if opt.Disabled == nil {
		return errors.New("nil Disabled field")
	}
	return nil
}

// ActionRunnersResponse contains a page of runners.
type ActionRunnersResponse struct {
	Runners    []*ActionRunner `json:"runners"`
	TotalCount int64           `json:"total_count"`
}

// ActionWorkflow represents a repository workflow definition.
type ActionWorkflow struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url"`
	HTMLURL   string    `json:"html_url"`
	BadgeURL  string    `json:"badge_url"`
	DeletedAt time.Time `json:"deleted_at"`
}

// ActionWorkflowResponse contains a workflow list response.
type ActionWorkflowResponse struct {
	Workflows  []*ActionWorkflow `json:"workflows"`
	TotalCount int64             `json:"total_count"`
}

// CreateActionWorkflowDispatchOption triggers a workflow_dispatch event.
type CreateActionWorkflowDispatchOption struct {
	Ref    string            `json:"ref"`
	Inputs map[string]string `json:"inputs,omitempty"`
}

// Validate checks whether the dispatch payload is valid.
func (opt CreateActionWorkflowDispatchOption) Validate() error {
	if len(opt.Ref) == 0 {
		return errors.New("empty Ref field")
	}
	return nil
}

// RunDetails contains the workflow run identifiers returned by workflow dispatch.
type RunDetails struct {
	WorkflowRunID int64  `json:"workflow_run_id"`
	RunURL        string `json:"run_url"`
	HTMLURL       string `json:"html_url"`
}

// ActionArtifact represents an Actions artifact.
type ActionArtifact struct {
	ID                 int64              `json:"id"`
	Name               string             `json:"name"`
	SizeInBytes        int64              `json:"size_in_bytes"`
	URL                string             `json:"url"`
	ArchiveDownloadURL string             `json:"archive_download_url"`
	Expired            bool               `json:"expired"`
	WorkflowRun        *ActionWorkflowRun `json:"workflow_run"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	ExpiresAt          time.Time          `json:"expires_at"`
}

// ActionArtifactsResponse contains a page of artifacts.
type ActionArtifactsResponse struct {
	Artifacts  []*ActionArtifact `json:"artifacts"`
	TotalCount int64             `json:"total_count"`
}

// ListActionArtifactsOptions controls artifact listing requests.
type ListActionArtifactsOptions struct {
	ListOptions
	Name string
}

// QueryEncode turns the artifact list options into a query string.
func (opt *ListActionArtifactsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Name != "" {
		query.Add("name", opt.Name)
	}
	return query.Encode()
}

func (c *Client) createActionRegistrationToken(path string) (*RegistrationToken, *Response, error) {
	token := new(RegistrationToken)
	resp, err := c.getParsedResponse("POST", path, nil, nil, token)
	return token, resp, err
}

func (c *Client) listActionRuns(path string, opt ListRepoActionRunsOptions) (*ActionWorkflowRunsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_26_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionWorkflowRunsResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

func (c *Client) listActionJobs(path string, opt ListRepoActionJobsOptions) (*ActionWorkflowJobsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_26_0); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionWorkflowJobsResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

func (c *Client) listActionRunners(path string, opt ListActionRunnersOptions) (*ActionRunnersResponse, *Response, error) {
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionRunnersResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

func (c *Client) getActionRunner(path string) (*ActionRunner, *Response, error) {
	runner := new(ActionRunner)
	resp, err := c.getParsedResponse("GET", path, jsonHeader, nil, runner)
	return runner, resp, err
}

func (c *Client) updateActionRunner(path string, opt EditActionRunnerOption) (*ActionRunner, *Response, error) {
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	runner := new(ActionRunner)
	resp, err := c.getParsedResponse("PATCH", path, jsonHeader, bytes.NewReader(body), runner)
	return runner, resp, err
}

func (c *Client) listActionArtifacts(path string, opt ListActionArtifactsOptions) (*ActionArtifactsResponse, *Response, error) {
	opt.setDefaults()
	link, _ := url.Parse(path)
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionArtifactsResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}
