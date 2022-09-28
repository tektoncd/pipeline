// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/jenkins-x/go-scm/scm"
)

type contentService struct {
	client *wrapper
}

func (s *contentService) Find(ctx context.Context, repo, path, ref string) (*scm.Content, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	endpoint := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/raw/%s?at=%s", namespace, name, path, url.QueryEscape(ref))
	buf := new(bytes.Buffer)
	res, err := s.client.do(ctx, "GET", endpoint, nil, buf)
	return &scm.Content{
		Path: path,
		Data: buf.Bytes(),
	}, res, err
}

func (s *contentService) List(ctx context.Context, repo, path, ref string) ([]*scm.FileEntry, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *contentService) Delete(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}
