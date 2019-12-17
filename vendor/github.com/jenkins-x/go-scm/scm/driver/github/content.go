// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

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
	}, res, err
}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *contentService) Delete(ctx context.Context, repo, path, ref string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type content struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Sha     string `json:"sha"`
	Content string `json:"content"`
}

type contentUpdate struct {
	Sha     string `json:"sha"`
	Message string `json:"message"`
	HTMLURL string `json:"html_url"`
	Author  struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"author"`
	Committer struct {
		Name  string    `json:"name"`
		Email string    `json:"email"`
		Date  time.Time `json:"date"`
	} `json:"committer"`
}
