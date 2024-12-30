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

func (s *contentService) List(ctx context.Context, repo, path, ref string, opts *scm.ListOptions) ([]*scm.FileEntry, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	endpoint := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/files/%s?at=%s&%s", namespace, name, path, ref, encodeListOptions(opts))
	out := new(contents)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertFileEntryList(out), res, err
}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	namespace, repoName := scm.Split(repo)
	endpoint := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/browse/%s", namespace, repoName, path)
	message := params.Message
	if params.Signature.Name != "" && params.Signature.Email != "" {
		message = fmt.Sprintf("%s\nSigned-off-by: %s <%s>", params.Message, params.Signature.Name, params.Signature.Email)
	}
	in := &contentCreateUpdate{
		Message: message,
		Branch:  params.Branch,
		Content: params.Data,
	}
	return s.client.do(ctx, "PUT", endpoint, in, nil)
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *contentService) Delete(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type contents struct {
	pagination
	Values []string `json:"values"`
}

type contentCreateUpdate struct {
	Branch  string `json:"branch"`
	Message string `json:"message"`
	Content []byte `json:"content"`
	Sha     string `json:"sourceCommitId"`
}

func convertFileEntryList(from *contents) []*scm.FileEntry {
	var to []*scm.FileEntry
	for _, v := range from.Values {
		to = append(to, &scm.FileEntry{
			Path: v,
		})
	}
	return to
}
