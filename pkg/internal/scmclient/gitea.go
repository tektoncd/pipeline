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

type giteaClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func newGiteaClient(serverURL, token string) SCMClient {
	return &giteaClient{
		baseURL:    strings.TrimRight(serverURL, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *giteaClient) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+c.token)
	return req, nil
}

// API docs: https://docs.gitea.com/api/#tag/repository/operation/repoGetContents
func (c *giteaClient) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	rawURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s?ref=%s",
		c.baseURL, org, repo, url.PathEscape(path), url.QueryEscape(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("gitea: GetFileContent: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("gitea: GetFileContent: %w", err)
	}

	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("gitea: GetFileContent: failed to parse response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(
		strings.ReplaceAll(resp.Content, "\n", ""),
	)
	if err != nil {
		return nil, fmt.Errorf("gitea: GetFileContent: failed to decode content: %w", err)
	}
	return decoded, nil
}

// API docs: https://docs.gitea.com/api/#tag/repository/operation/repoGetSingleCommit
func (c *giteaClient) GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error) {
	rawURL := fmt.Sprintf("%s/api/v1/repos/%s/%s/git/commits/%s", c.baseURL, org, repo, url.PathEscape(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("gitea: GetCommitSHA: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("gitea: GetCommitSHA: %w", err)
	}

	var resp struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("gitea: GetCommitSHA: failed to parse response: %w", err)
	}
	if resp.SHA == "" {
		return "", errors.New("gitea: GetCommitSHA: empty sha in response")
	}
	return resp.SHA, nil
}

// API docs: https://docs.gitea.com/api/#tag/repository/operation/repoGet
func (c *giteaClient) GetCloneURL(ctx context.Context, org, repo string) (string, error) {
	rawURL := fmt.Sprintf("%s/api/v1/repos/%s/%s", c.baseURL, org, repo)
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("gitea: GetCloneURL: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("gitea: GetCloneURL: %w", err)
	}

	var resp struct {
		CloneURL string `json:"clone_url"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("gitea: GetCloneURL: failed to parse response: %w", err)
	}
	if resp.CloneURL == "" {
		return "", errors.New("gitea: GetCloneURL: empty clone_url in response")
	}
	return resp.CloneURL, nil
}
