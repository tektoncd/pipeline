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

type bitbucketServerClient struct {
	baseURL      string
	username     string
	password     string
	useBasicAuth bool
	httpClient   *http.Client
}

// newBitbucketServerClient accepts token in two formats:
//   - "username:token": uses HTTP Basic auth for user-level HTTP access tokens.
//   - bare token: uses Bearer auth for project/repository-level tokens,
//     which do not require a username.
func newBitbucketServerClient(serverURL, token string) SCMClient {
	username, password := splitUsernamePassword(token)
	return &bitbucketServerClient{
		baseURL:      strings.TrimRight(serverURL, "/"),
		username:     username,
		password:     password,
		useBasicAuth: strings.Contains(token, ":"),
		httpClient:   &http.Client{},
	}
}

func (c *bitbucketServerClient) newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if c.useBasicAuth {
		// User-level HTTP access token: "username:token" Basic auth
		req.SetBasicAuth(c.username, c.password)
	} else {
		// Project/repository token: Bearer auth only, no username
		req.Header.Set("Authorization", "Bearer "+c.password)
	}
	return req, nil
}

// API docs: https://developer.atlassian.com/server/bitbucket/rest/v1003/api-group-repository/#api-api-latest-projects-projectkey-repos-repositoryslug-raw-path-get
func (c *bitbucketServerClient) GetFileContent(ctx context.Context, org, repo, path, ref string) ([]byte, error) {
	rawURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/raw/%s?at=%s",
		c.baseURL, org, repo, url.PathEscape(path), url.QueryEscape(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("bitbucketserver: GetFileContent: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bitbucketserver: GetFileContent: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("bitbucketserver: GetFileContent: unexpected status %d: %s", resp.StatusCode, body)
		return nil, errors.New(errMsg)
	}
	return io.ReadAll(resp.Body)
}

// API docs: https://developer.atlassian.com/server/bitbucket/rest/v1003/api-group-repository/#api-api-latest-projects-projectkey-repos-repositoryslug-commits-commitid-get
func (c *bitbucketServerClient) GetCommitSHA(ctx context.Context, org, repo, ref string) (string, error) {
	rawURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits/%s",
		c.baseURL, org, repo, url.PathEscape(ref))
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("bitbucketserver: GetCommitSHA: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("bitbucketserver: GetCommitSHA: %w", err)
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("bitbucketserver: GetCommitSHA: failed to parse response: %w", err)
	}
	if resp.ID == "" {
		return "", errors.New("bitbucketserver: GetCommitSHA: empty id in response")
	}
	return resp.ID, nil
}

// API docs: https://developer.atlassian.com/server/bitbucket/rest/v1003/api-group-project/#api-api-latest-projects-projectkey-repos-repositoryslug-get
func (c *bitbucketServerClient) GetCloneURL(ctx context.Context, org, repo string) (string, error) {
	rawURL := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s", c.baseURL, org, repo)
	req, err := c.newRequest(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("bitbucketserver: GetCloneURL: %w", err)
	}
	body, err := doRequest(ctx, c.httpClient, req)
	if err != nil {
		return "", fmt.Errorf("bitbucketserver: GetCloneURL: %w", err)
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
		return "", fmt.Errorf("bitbucketserver: GetCloneURL: failed to parse response: %w", err)
	}
	for _, link := range resp.Links.Clone {
		if link.Name == "http" {
			return link.Href, nil
		}
	}
	return "", errors.New("bitbucketserver: GetCloneURL: no http clone URL found")
}
