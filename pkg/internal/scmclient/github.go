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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const defaultGitHubBaseURL = "https://api.github.com"

type githubClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newGitHubClient(serverURL, token string) SCMClient {
	base := serverURL
	if base == "" {
		base = defaultGitHubBaseURL
	}
	return &githubClient{
		baseURL:    strings.TrimRight(base, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *githubClient) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-Github-Api-Version", "2026-03-10")
	return req, nil
}

// API docs: https://docs.github.com/en/rest/repos/contents?apiVersion=2026-03-10#get-repository-content
func (c *githubClient) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		c.baseURL, org, repo, url.PathEscape(path), url.QueryEscape(ref))
	req, err := c.newRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("github: GetFileContent: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("github: GetFileContent: %w", err)
	}

	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("github: GetFileContent: failed to parse response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("github: GetFileContent: failed to decode content: %w", err)
	}
	return decoded, nil
}

// API docs: https://docs.github.com/en/rest/commits/commits?apiVersion=2026-03-10#get-a-commit
func (c *githubClient) GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.baseURL, org, repo, url.PathEscape(ref))
	req, err := c.newRequest(ctx, url)
	if err != nil {
		return "", fmt.Errorf("github: GetCommitSHA: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("github: GetCommitSHA: %w", err)
	}

	var resp struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("github: GetCommitSHA: failed to parse response: %w", err)
	}
	if resp.SHA == "" {
		return "", errors.New("github: GetCommitSHA: empty sha in response")
	}
	return resp.SHA, nil
}

// API docs: https://docs.github.com/en/rest/repos/repos?apiVersion=2026-03-10#get-a-repository
func (c *githubClient) GetCloneURL(ctx context.Context, org, repo string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", c.baseURL, org, repo)
	req, err := c.newRequest(ctx, url)
	if err != nil {
		return "", fmt.Errorf("github: GetCloneURL: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("github: GetCloneURL: %w", err)
	}

	var resp struct {
		CloneURL string `json:"clone_url"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("github: GetCloneURL: failed to parse response: %w", err)
	}
	if resp.CloneURL == "" {
		return "", errors.New("github: GetCloneURL: empty clone_url in response")
	}
	return resp.CloneURL, nil
}
