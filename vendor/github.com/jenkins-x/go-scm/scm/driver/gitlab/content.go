// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

type contentService struct {
	client *wrapper
}

func (s *contentService) Find(ctx context.Context, repo, path, ref string) (*scm.Content, *scm.Response, error) {
	path = url.QueryEscape(path)
	path = strings.Replace(path, ".", "%2E", -1)
	endpoint := fmt.Sprintf("api/v4/projects/%s/repository/files/%s?ref=%s", encode(repo), path, ref)
	out := new(content)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	raw, berr := base64.StdEncoding.DecodeString(out.Content)
	if berr != nil {
		// samples in the gitlab documentation use RawStdEncoding
		// so we fallback if StdEncoding returns an error.
		raw, berr = base64.RawStdEncoding.DecodeString(out.Content)
		if berr != nil {
			return nil, res, err
		}
	}
	return &scm.Content{
		Path: out.FilePath,
		Data: raw,
	}, res, err
}

func (s *contentService) List(ctx context.Context, repo, path, ref string) ([]*scm.FileEntry, *scm.Response, error) {
	endpoint := fmt.Sprintf("api/v4/projects/%s/repository/tree?path=%s&ref=%s", encode(repo), path, ref)
	out := []*entry{}
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)
	return convertEntryList(out), res, err
}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	endpoint := fmt.Sprintf("api/v4/projects/%s/repository/commits", encode(repo))

	body := &createCommitBody{
		Message: params.Message,
		ID:      encode(repo),
		Branch:  params.Branch,
		Actions: []createCommitAction{
			{Action: "create", Path: path, Content: params.Data, Encoding: "base64"},
		},
	}
	return s.client.do(ctx, "POST", endpoint, &body, nil)
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	path = url.QueryEscape(path)
	path = strings.Replace(path, ".", "%2E", -1)
	endpoint := fmt.Sprintf("api/v4/projects/%s/repository/files/%s", encode(repo), path)

	body := &updateContentBody{
		Message: params.Message,
		Branch:  params.Branch,
		Content: string(params.Data),
	}
	return s.client.do(ctx, "PUT", endpoint, &body, nil)
}

func (s *contentService) Delete(ctx context.Context, repo, path, ref string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type content struct {
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	Size         int    `json:"size"`
	Encoding     string `json:"encoding"`
	Content      string `json:"content"`
	Ref          string `json:"ref"`
	BlobID       string `json:"blob_id"`
	CommitID     string `json:"commit_id"`
	LastCommitID string `json:"last_commit_id"`
}

type createCommitAction struct {
	Action   string `json:"action"`
	Path     string `json:"file_path"`
	Content  []byte `json:"content"`
	Encoding string `json:"encoding"`
}

type createCommitBody struct {
	Branch  string               `json:"branch"`
	ID      string               `json:"id"`
	Message string               `json:"commit_message"`
	Actions []createCommitAction `json:"actions"`
}

type updateContentBody struct {
	Branch  string `json:"branch"`
	Content string `json:"content"`
	Message string `json:"commit_message"`
}

type entry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Mode string `json:"mode"`
}

func convertEntryList(out []*entry) []*scm.FileEntry {
	answer := make([]*scm.FileEntry, 0, len(out))
	for _, o := range out {
		answer = append(answer, convertEntry(o))
	}
	return answer
}

func convertEntry(from *entry) *scm.FileEntry {
	t := "file"
	if from.Type == "tree" {
		t = "dir"
	}
	return &scm.FileEntry{
		Name: from.Name,
		Path: from.Path,
		Type: t,
	}
}
