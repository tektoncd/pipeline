/*
Copyright 2026 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scmclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const defaultAzureBaseURL = "https://dev.azure.com"

type azureClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newAzureClient(serverURL, token string) SCMClient {
	base := serverURL
	if base == "" {
		base = defaultAzureBaseURL
	}
	return &azureClient{
		baseURL:    strings.TrimRight(base, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *azureClient) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// Azure PAT auth: Basic with empty username and PAT as password.
	req.SetBasicAuth("", c.token)
	return req, nil
}

// API docs: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/items/get?view=azure-devops-rest-7.1&tabs=HTTP
func (c *azureClient) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	organization, project, err := decodeAzureOrgProject(org)
	if err != nil {
		return nil, err
	}
	rawURL := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/items?path=%s&%s&includeContent=true&api-version=7.1",
		c.baseURL, organization, project, repo, url.QueryEscape(path), generateVersionDescriptor(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("azure: GetFileContent: %w", err)
	}
	// Request plain text response so we get raw file content directly.
	req.Header.Set("Accept", "text/plain")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure: GetFileContent: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure: GetFileContent: unexpected status %d: %s", resp.StatusCode, body)
	}
	return io.ReadAll(resp.Body)
}

// API docs: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/commits/get?view=azure-devops-rest-7.1&tabs=HTTP
func (c *azureClient) GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error) {
	organization, project, err := decodeAzureOrgProject(org)
	if err != nil {
		return "", err
	}
	rawURL := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s/commits/%s?api-version=7.1",
		c.baseURL, organization, project, repo, url.PathEscape(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("azure: GetCommitSHA: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("azure: GetCommitSHA: %w", err)
	}
	var resp struct {
		CommitID string `json:"commitId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("azure: GetCommitSHA: failed to parse response: %w", err)
	}
	if resp.CommitID == "" {
		return "", errors.New("azure: GetCommitSHA: empty commitId in response")
	}
	return resp.CommitID, nil
}

// API docs: https://learn.microsoft.com/en-us/rest/api/azure/devops/git/repositories/get-repository?view=azure-devops-rest-7.1&tabs=HTTP
func (c *azureClient) GetCloneURL(ctx context.Context, org, repo string) (string, error) {
	organization, project, err := decodeAzureOrgProject(org)
	if err != nil {
		return "", err
	}
	rawURL := fmt.Sprintf("%s/%s/%s/_apis/git/repositories/%s?api-version=7.1",
		c.baseURL, organization, project, repo)
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("azure: GetCloneURL: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("azure: GetCloneURL: %w", err)
	}
	var resp struct {
		RemoteURL string `json:"remoteUrl"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("azure: GetCloneURL: failed to parse response: %w", err)
	}
	if resp.RemoteURL == "" {
		return "", errors.New("azure: GetCloneURL: empty remoteUrl in response")
	}
	return resp.RemoteURL, nil
}

// decodeAzureRepo splits "org/project" into its two components.
func decodeAzureOrgProject(org string) (string, string, error) {
	parts := strings.SplitN(org, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("azure: org must be in 'org/project' format, got %q", org)
	}
	return parts[0], parts[1], nil
}

// generateVersionDescriptor returns the versionDescriptor query params for a ref.
// Handles refs/heads/, refs/tags/, and bare branch/tag names or commit SHAs.
func generateVersionDescriptor(ref string) string {
	if branch, ok := strings.CutPrefix(ref, "refs/heads/"); ok {
		return fmt.Sprintf("versionDescriptor.version=%s&versionDescriptor.versionType=branch", branch)
	}
	if tag, ok := strings.CutPrefix(ref, "refs/tags/"); ok {
		return fmt.Sprintf("versionDescriptor.version=%s&versionDescriptor.versionType=tag", tag)
	}
	// Bare SHA (40 hex chars) - treat as commit
	if len(ref) == 40 {
		return fmt.Sprintf("versionDescriptor.version=%s&versionDescriptor.versionType=commit", ref)
	}
	// Bare branch name
	return fmt.Sprintf("versionDescriptor.version=%s&versionDescriptor.versionType=branch", ref)
}
