// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// --- Pinned pull requests ---

// ListRepoPinnedPullRequests lists a repo's pinned pull requests
func (c *Client) ListRepoPinnedPullRequests(owner, repo string) ([]*PullRequest, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	prs := make([]*PullRequest, 0, 5)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/pulls/pinned", owner, repo),
		jsonHeader, nil, &prs)
	return prs, resp, err
}

// --- Webhooks ---

// TestWebhook tests a webhook
func (c *Client) TestWebhook(owner, repo string, hookID int64, ref string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	opt := map[string]string{"ref": ref}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/repos/%s/%s/hooks/%d/tests", owner, repo, hookID),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// --- Merge upstream ---

// MergeUpstreamRequest options for merging upstream
type MergeUpstreamRequest struct {
	Branch string `json:"branch"`
	FfOnly bool   `json:"ff_only"`
}

// MergeUpstreamResponse represents the response from merging upstream
type MergeUpstreamResponse struct {
	MergeStyle string `json:"merge_type"`
}

// MergeUpstream merges upstream into a forked repository
func (c *Client) MergeUpstream(owner, repo string, opt MergeUpstreamRequest) (*MergeUpstreamResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(MergeUpstreamResponse)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/merge-upstream", owner, repo),
		jsonHeader, bytes.NewReader(body), result)
	return result, resp, err
}

// --- Pin allowed ---

// NewIssuePinsAllowed represents whether new issue/PR pins are allowed
type NewIssuePinsAllowed struct {
	Issues       bool `json:"issues"`
	PullRequests bool `json:"pull_requests"`
}

// CheckPinAllowed checks if the current user can pin issues or PRs
func (c *Client) CheckPinAllowed(owner, repo string) (*NewIssuePinsAllowed, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	result := new(NewIssuePinsAllowed)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/new_pin_allowed", owner, repo),
		jsonHeader, nil, result)
	return result, resp, err
}

// --- Batch file operations ---

// ChangeFilesOptions options for batch file operations
type ChangeFilesOptions struct {
	Files     []*ChangeFileOperation `json:"files"`
	Message   string                 `json:"message"`
	Branch    string                 `json:"branch,omitempty"`
	NewBranch string                 `json:"new_branch,omitempty"`
	ForcePush bool                   `json:"force_push,omitempty"`
	Author    Identity               `json:"author"`
	Committer Identity               `json:"committer"`
	Dates     CommitDateOptions      `json:"dates"`
	Signoff   bool                   `json:"signoff,omitempty"`
}

// ChangeFileOperation represents a file operation in batch
type ChangeFileOperation struct {
	Operation string `json:"operation"` // create, update, upload, rename, delete
	Path      string `json:"path"`
	Content   string `json:"content"`             // base64 encoded for create/update
	SHA       string `json:"sha,omitempty"`       // required for update/delete
	FromPath  string `json:"from_path,omitempty"` // for rename
}

// ChangeFiles creates, updates, or deletes multiple files
func (c *Client) ChangeFiles(owner, repo string, opt ChangeFilesOptions) (*FileResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(FileResponse)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/contents", owner, repo),
		jsonHeader, bytes.NewReader(body), &result)
	return result, resp, err
}

// --- Branch protection priorities ---

// UpdateBranchProtectionPriorities updates the priorities of branch protection rules
func (c *Client) UpdateBranchProtectionPriorities(owner, repo string, ids []int64) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	opt := map[string][]int64{"ids": ids}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("POST",
		fmt.Sprintf("/repos/%s/%s/branch_protections/priority", owner, repo),
		jsonHeader, bytes.NewReader(body))
	if err != nil {
		return resp, err
	}
	if status != http.StatusNoContent {
		return resp, fmt.Errorf("unexpected status: %d", status)
	}
	return resp, nil
}

// --- File contents (batch lookup) ---

// GetFilesOptions controls batch file-content lookup requests.
type GetFilesOptions struct {
	Files []string `json:"files"`
}

// Validate checks whether the batch file lookup request is valid.
func (opt GetFilesOptions) Validate() error {
	if len(opt.Files) == 0 {
		return errors.New("empty Files field")
	}
	return nil
}

// GetRepoFileContents fetches metadata and contents for multiple files through the GET endpoint.
// The file list is JSON-encoded in the "body" query parameter; for large file lists prefer
// PostRepoFileContents to avoid URL length limitations.
func (c *Client) GetRepoFileContents(owner, repo, ref string, opt GetFilesOptions) ([]*ContentsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/file-contents", owner, repo))
	query := link.Query()
	if ref != "" {
		query.Add("ref", ref)
	}
	query.Add("body", string(body))
	link.RawQuery = query.Encode()

	contents := make([]*ContentsResponse, 0, len(opt.Files))
	resp, err := c.getParsedResponse("GET", link.String(), jsonHeader, nil, &contents)
	return contents, resp, err
}

// PostRepoFileContents fetches metadata and contents for multiple files through the POST endpoint.
func (c *Client) PostRepoFileContents(owner, repo, ref string, opt GetFilesOptions) ([]*ContentsResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/file-contents", owner, repo))
	if ref != "" {
		link.RawQuery = url.Values{"ref": []string{ref}}.Encode()
	}

	contents := make([]*ContentsResponse, 0, len(opt.Files))
	resp, err := c.getParsedResponse("POST", link.String(), jsonHeader, bytes.NewReader(body), &contents)
	return contents, resp, err
}

// --- Issue config ---

// IssueConfigContactLink represents an issue config contact link.
type IssueConfigContactLink struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	About string `json:"about"`
}

// IssueConfig represents the parsed issue config for a repository.
type IssueConfig struct {
	BlankIssuesEnabled bool                     `json:"blank_issues_enabled"`
	ContactLinks       []IssueConfigContactLink `json:"contact_links"`
}

// GetIssueConfig gets the issue config for a repository.
func (c *Client) GetIssueConfig(owner, repo string) (*IssueConfig, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	config := new(IssueConfig)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issue_config", owner, repo), jsonHeader, nil, config)
	return config, resp, err
}

// --- Licenses ---

// GetRepoLicenses gets detected licenses for a repository.
func (c *Client) GetRepoLicenses(owner, repo string) ([]string, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	licenses := make([]string, 0, 2)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/licenses", owner, repo), jsonHeader, nil, &licenses)
	return licenses, resp, err
}

// --- Signing keys ---

// GetRepoSigningKeyGPG gets the repository signing GPG public key.
func (c *Client) GetRepoSigningKeyGPG(owner, repo string) (string, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return "", nil, err
	}
	key, resp, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/signing-key.gpg", owner, repo), nil, nil)
	return string(key), resp, err
}

// GetRepoSigningKeySSH gets the repository signing SSH public key.
func (c *Client) GetRepoSigningKeySSH(owner, repo string) (string, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return "", nil, err
	}
	key, resp, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/signing-key.pub", owner, repo), nil, nil)
	return string(key), resp, err
}

// --- Subscribers ---

// ListRepoSubscribers lists repository watchers.
func (c *Client) ListRepoSubscribers(owner, repo string, opt ListOptions) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	subscribers := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/subscribers?%s", owner, repo, opt.getURLQuery().Encode()), jsonHeader, nil, &subscribers)
	return subscribers, resp, err
}

// --- Diff patch ---

// ApplyDiffPatchFileOptions applies a patch against repository contents.
type ApplyDiffPatchFileOptions struct {
	FileOptions
	Content string `json:"content"`
}

// Validate checks whether the patch payload is valid.
func (opt ApplyDiffPatchFileOptions) Validate() error {
	if len(opt.Content) == 0 {
		return errors.New("empty Content field")
	}
	return nil
}

// ApplyRepoDiffPatch applies a patch to repository contents.
func (c *Client) ApplyRepoDiffPatch(owner, repo string, opt ApplyDiffPatchFileOptions) (*FileResponse, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	result := new(FileResponse)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/diffpatch", owner, repo), jsonHeader, bytes.NewReader(body), result)
	return result, resp, err
}

// --- Push mirrors ---

// TriggerPushMirrorsSync triggers push-mirror syncing for a repository.
func (c *Client) TriggerPushMirrorsSync(owner, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("POST", fmt.Sprintf("/repos/%s/%s/push_mirrors-sync", owner, repo), nil, nil)
}
