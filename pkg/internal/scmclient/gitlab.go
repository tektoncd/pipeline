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

const defaultGitLabBaseURL = "https://gitlab.com"

type gitlabClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newGitLabClient(serverURL, token string) SCMClient {
	base := serverURL
	if base == "" {
		base = defaultGitLabBaseURL
	}
	return &gitlabClient{
		baseURL:    strings.TrimRight(base, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *gitlabClient) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Private-Token", c.token)
	return req, nil
}

// encodeProjectID encodes "org/repo" as "org%2Frepo" for use as a GitLab project ID.
func encodeProjectID(org, repo string) string {
	return url.PathEscape(org + "/" + repo)
}

// API docs: https://docs.gitlab.com/api/repository_files/#retrieve-a-file-from-a-repository
func (c *gitlabClient) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	rawURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s?ref=%s",
		c.baseURL, encodeProjectID(org, repo), url.PathEscape(path), url.QueryEscape(ref))

	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("gitlab: GetFileContent: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: GetFileContent: %w", err)
	}

	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("gitlab: GetFileContent: failed to parse response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("gitlab: GetFileContent: failed to decode content: %w", err)
	}
	return decoded, nil
}

// API docs: https://docs.gitlab.com/api/commits/#retrieve-a-commit
func (c *gitlabClient) GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error) {
	rawURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits/%s",
		c.baseURL, encodeProjectID(org, repo), url.PathEscape(ref))

	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("gitlab: GetCommitSHA: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("gitlab: GetCommitSHA: %w", err)
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("gitlab: GetCommitSHA: failed to parse response: %w", err)
	}
	if resp.ID == "" {
		return "", errors.New("gitlab: GetCommitSHA: empty id in response")
	}
	return resp.ID, nil
}

// API docs: https://docs.gitlab.com/api/projects/#retrieve-a-project
func (c *gitlabClient) GetCloneURL(ctx context.Context, org, repo string) (string, error) {
	rawURL := fmt.Sprintf("%s/api/v4/projects/%s", c.baseURL, encodeProjectID(org, repo))

	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("gitlab: GetCloneURL: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("gitlab: GetCloneURL: %w", err)
	}

	var resp struct {
		HTTPURLToRepo string `json:"http_url_to_repo"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("gitlab: GetCloneURL: failed to parse response: %w", err)
	}
	if resp.HTTPURLToRepo == "" {
		return "", errors.New("gitlab: GetCloneURL: empty http_url_to_repo in response")
	}
	return resp.HTTPURLToRepo, nil
}
