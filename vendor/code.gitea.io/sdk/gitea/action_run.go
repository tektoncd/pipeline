// Copyright 2025 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"time"
)

// ActionTask represents a workflow run task (from /actions/tasks endpoint)
// This is the format returned by older Gitea versions
type ActionTask struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"` // Workflow name
	HeadBranch   string    `json:"head_branch"`
	HeadSHA      string    `json:"head_sha"`
	RunNumber    int64     `json:"run_number"`
	Event        string    `json:"event"`
	DisplayTitle string    `json:"display_title"` // PR title or commit message
	Status       string    `json:"status"`
	WorkflowID   string    `json:"workflow_id"` // e.g. "ci.yml"
	URL          string    `json:"url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	RunStartedAt time.Time `json:"run_started_at"`
}

// ActionTaskResponse holds the response for listing action tasks
type ActionTaskResponse struct {
	TotalCount   int64         `json:"total_count"`
	WorkflowRuns []*ActionTask `json:"workflow_runs"`
}

// ActionWorkflowRun represents a workflow run (from /actions/runs endpoint)
// This is the format returned by newer Gitea versions
type ActionWorkflowRun struct {
	ID             int64       `json:"id"`
	DisplayTitle   string      `json:"display_title"`
	Event          string      `json:"event"`
	HeadBranch     string      `json:"head_branch"`
	HeadSha        string      `json:"head_sha"`
	Path           string      `json:"path"`
	RunAttempt     int64       `json:"run_attempt"`
	RunNumber      int64       `json:"run_number"`
	Status         string      `json:"status"`
	Conclusion     string      `json:"conclusion"`
	URL            string      `json:"url"`
	HTMLURL        string      `json:"html_url"`
	StartedAt      time.Time   `json:"started_at"`
	CompletedAt    time.Time   `json:"completed_at"`
	Actor          *User       `json:"actor"`
	TriggerActor   *User       `json:"trigger_actor"`
	Repository     *Repository `json:"repository"`
	HeadRepository *Repository `json:"head_repository"`
	RepositoryID   int64       `json:"repository_id"`
}

// ActionWorkflowRunsResponse holds the response for listing workflow runs
type ActionWorkflowRunsResponse struct {
	TotalCount   int64                `json:"total_count"`
	WorkflowRuns []*ActionWorkflowRun `json:"workflow_runs"`
}

// ActionWorkflowJob represents a job within a workflow run
type ActionWorkflowJob struct {
	ID          int64                 `json:"id"`
	RunID       int64                 `json:"run_id"`
	RunURL      string                `json:"run_url"`
	RunAttempt  int64                 `json:"run_attempt"`
	Name        string                `json:"name"`
	HeadBranch  string                `json:"head_branch"`
	HeadSha     string                `json:"head_sha"`
	Status      string                `json:"status"`
	Conclusion  string                `json:"conclusion"`
	URL         string                `json:"url"`
	HTMLURL     string                `json:"html_url"`
	CreatedAt   time.Time             `json:"created_at"`
	StartedAt   time.Time             `json:"started_at"`
	CompletedAt time.Time             `json:"completed_at"`
	RunnerID    int64                 `json:"runner_id"`
	RunnerName  string                `json:"runner_name"`
	Labels      []string              `json:"labels"`
	Steps       []*ActionWorkflowStep `json:"steps"`
}

// ActionWorkflowJobsResponse holds the response for listing workflow jobs
type ActionWorkflowJobsResponse struct {
	TotalCount int64                `json:"total_count"`
	Jobs       []*ActionWorkflowJob `json:"jobs"`
}

// ActionWorkflowStep represents a step within a job
type ActionWorkflowStep struct {
	Name        string    `json:"name"`
	Number      int64     `json:"number"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// ListRepoActionRunsOptions options for listing repository action runs
type ListRepoActionRunsOptions struct {
	ListOptions
	Branch  string // Filter by branch
	Event   string // Filter by triggering event
	Status  string // Filter by status (pending, queued, in_progress, failure, success, skipped)
	Actor   string // Filter by actor (user who triggered the run)
	HeadSHA string // Filter by the SHA of the head commit
}

// QueryEncode encodes the options to URL query parameters
func (opt *ListRepoActionRunsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Branch != "" {
		query.Add("branch", opt.Branch)
	}
	if opt.Event != "" {
		query.Add("event", opt.Event)
	}
	if opt.Status != "" {
		query.Add("status", opt.Status)
	}
	if opt.Actor != "" {
		query.Add("actor", opt.Actor)
	}
	if opt.HeadSHA != "" {
		query.Add("head_sha", opt.HeadSHA)
	}
	return query.Encode()
}

// ListRepoActionJobsOptions options for listing repository action jobs
type ListRepoActionJobsOptions struct {
	ListOptions
	Status string // Filter by status (pending, queued, in_progress, failure, success, skipped)
}

// QueryEncode encodes the options to URL query parameters
func (opt *ListRepoActionJobsOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if opt.Status != "" {
		query.Add("status", opt.Status)
	}
	return query.Encode()
}

// ListRepoActionRuns lists workflow runs for a repository.
// Requires Gitea 1.25.0 or later. For older versions, use ListRepoActionTasks.
func (c *Client) ListRepoActionRuns(owner, repo string, opt ListRepoActionRunsOptions) (*ActionWorkflowRunsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/runs", owner, repo))
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionWorkflowRunsResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// GetRepoActionRun gets a single workflow run.
// Requires Gitea 1.25.0 or later.
func (c *Client) GetRepoActionRun(owner, repo string, runID int64) (*ActionWorkflowRun, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	run := new(ActionWorkflowRun)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), jsonHeader, nil, run)
	return run, resp, err
}

// ListRepoActionRunJobs lists jobs for a workflow run.
// Requires Gitea 1.25.0 or later.
func (c *Client) ListRepoActionRunJobs(owner, repo string, runID int64, opt ListRepoActionJobsOptions) (*ActionWorkflowJobsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, repo, runID))
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionWorkflowJobsResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// ListRepoActionJobs lists all jobs for a repository.
// Requires Gitea 1.25.0 or later.
func (c *Client) ListRepoActionJobs(owner, repo string, opt ListRepoActionJobsOptions) (*ActionWorkflowJobsResponse, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/jobs", owner, repo))
	link.RawQuery = opt.QueryEncode()

	resp := new(ActionWorkflowJobsResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// GetRepoActionJob gets a single job.
// Requires Gitea 1.25.0 or later.
func (c *Client) GetRepoActionJob(owner, repo string, jobID int64) (*ActionWorkflowJob, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	job := new(ActionWorkflowJob)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/jobs/%d", owner, repo, jobID), jsonHeader, nil, job)
	return job, resp, err
}

// GetRepoActionJobLogs gets the logs for a specific job.
// Requires Gitea 1.25.0 or later.
func (c *Client) GetRepoActionJobLogs(owner, repo string, jobID int64) ([]byte, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}

	return c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/actions/jobs/%d/logs", owner, repo, jobID), nil, nil)
}

// ListRepoActionTasks lists workflow tasks for a repository (Gitea 1.24.x and earlier)
// Use this for older Gitea versions that don't have /actions/runs endpoint
func (c *Client) ListRepoActionTasks(owner, repo string, opt ListOptions) (*ActionTaskResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()

	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/actions/tasks", owner, repo))
	link.RawQuery = opt.getURLQuery().Encode()

	resp := new(ActionTaskResponse)
	response, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, resp)
	return resp, response, err
}

// DeleteRepoActionRun deletes a workflow run.
// Requires Gitea 1.25.0 or later.
func (c *Client) DeleteRepoActionRun(owner, repo string, runID int64) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_25_0); err != nil {
		return nil, err
	}
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}

	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/actions/runs/%d", owner, repo, runID), jsonHeader, nil)
	return resp, err
}
