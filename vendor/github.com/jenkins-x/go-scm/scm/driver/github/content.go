// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
)

type contentService struct {
	client *wrapper
}

func (s *contentService) Find(ctx context.Context, repo, path, ref string) (*scm.Content, *scm.Response, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, path, ref)
	out := new(content)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	raw, _ := base64.StdEncoding.DecodeString(out.Content)
	return &scm.Content{
		Path: out.Path,
		Data: raw,
		Sha:  out.Sha,
	}, res, err
}

func (s *contentService) List(ctx context.Context, repo, path, ref string) ([]*scm.FileEntry, *scm.Response, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s?ref=%s", repo, path, ref)
	out := []*entry{}
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)
	return convertEntryList(out), res, err
}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s", repo, path)
	body := &contentBody{
		Message: params.Message,
		Content: params.Data,
		Branch:  params.Branch,
	}

	return s.client.do(ctx, "PUT", endpoint, &body, nil)
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	endpoint := fmt.Sprintf("repos/%s/contents/%s", repo, path)
	body := &contentBody{
		Message: params.Message,
		Content: params.Data,
		Branch:  params.Branch,
		Sha:     params.Sha,
	}

	return s.client.do(ctx, "PUT", endpoint, &body, nil)
}

func (s *contentService) Delete(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type content struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Sha     string `json:"sha"`
	Content string `json:"content"`
}

type entry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Size int    `json:"size"`
	Sha  string `json:"sha"`
	URL  string `json:"url"`
}

type contentBody struct {
	Message string `json:"message"`
	Content []byte `json:"content"`
	Sha     string `json:"sha,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

func convertEntryList(out []*entry) []*scm.FileEntry {
	answer := make([]*scm.FileEntry, 0, len(out))
	for _, o := range out {
		answer = append(answer, convertEntry(o))
	}
	return answer
}

func convertEntry(from *entry) *scm.FileEntry {
	return &scm.FileEntry{
		Name: from.Name,
		Path: from.Path,
		Type: from.Type,
		Size: from.Size,
		Sha:  from.Sha,
		Link: from.URL,
	}
}
