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

const defaultBitbucketCloudBaseURL = "https://api.bitbucket.org"

type bitbucketCloudClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// newBitbucketCloudClient accepts bitbucket access tokens, which use
// bearer auth for repository/workspace/project access tokens,
// not requiring a username
// api_token with username (Basic auth) is yet to be implemented (#7189, #9484)
func newBitbucketCloudClient(serverURL, token string) SCMClient {
	base := serverURL
	if base == "" {
		base = defaultBitbucketCloudBaseURL
	}
	return &bitbucketCloudClient{
		baseURL:    strings.TrimRight(base, "/"),
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *bitbucketCloudClient) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return req, nil
}

// API docs: https://developer.atlassian.com/cloud/bitbucket/rest/api-group-source/#api-repositories-workspace-repo-slug-src-commit-path-get
func (c *bitbucketCloudClient) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	rawURL := fmt.Sprintf("%s/2.0/repositories/%s/%s/src/%s/%s",
		c.baseURL, org, repo, url.PathEscape(ref), url.PathEscape(path))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("bitbucketcloud: GetFileContent: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucketcloud: GetFileContent: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bitbucketcloud: GetFileContent: unexpected status %d: %s", resp.StatusCode, body)
	}
	return io.ReadAll(resp.Body)
}

// API docs: https://developer.atlassian.com/cloud/bitbucket/rest/api-group-commits/#api-repositories-workspace-repo-slug-commit-commit-get
func (c *bitbucketCloudClient) GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error) {
	rawURL := fmt.Sprintf("%s/2.0/repositories/%s/%s/commit/%s",
		c.baseURL, org, repo, url.PathEscape(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("bitbucketcloud: GetCommitSHA: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("bitbucketcloud: GetCommitSHA: %w", err)
	}
	var resp struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("bitbucketcloud: GetCommitSHA: failed to parse response: %w", err)
	}
	if resp.Hash == "" {
		return "", errors.New("bitbucketcloud: GetCommitSHA: empty sha in response")
	}
	return resp.Hash, nil
}

// API docs: https://developer.atlassian.com/cloud/bitbucket/rest/api-group-repositories/#api-repositories-workspace-repo-slug-get
func (c *bitbucketCloudClient) GetCloneURL(ctx context.Context, org, repo string) (string, error) {
	rawURL := fmt.Sprintf("%s/2.0/repositories/%s/%s",
		c.baseURL, org, repo)
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("bitbucketcloud: GetCloneURL: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("bitbucketcloud: GetCloneURL: %w", err)
	}
	var resp struct {
		Links struct {
			Clone []struct {
				Name string `json:"name"`
				Href string `json:"href"`
			} `json:"clone"`
		} `json:"links"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("bitbucketcloud: GetCloneURL: failed to parse response: %w", err)
	}
	for _, link := range resp.Links.Clone {
		if link.Name == "https" {
			return link.Href, nil
		}
	}
	return "", errors.New("bitbucketcloud: GetCloneURL: no https clone URL found")
}
