// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"
	"encoding/base64"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

type contentService struct {
	client *wrapper
}

func (s *contentService) Find(ctx context.Context, repo, path, ref string) (*scm.Content, *scm.Response, error) {
	namespace, name := scm.Split(repo)

	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimPrefix(ref, "refs/tags/")

	out, resp, err := s.client.GiteaClient.GetContents(namespace, name, ref, path)
	if err != nil {
		return nil, toSCMResponse(resp), err
	}
	raw, _ := base64.StdEncoding.DecodeString(*out.Content)

	return &scm.Content{
		Path: path,
		Data: raw,
		Sha:  out.SHA,
	}, toSCMResponse(resp), err
}

func (s *contentService) List(ctx context.Context, repo, path, ref string) ([]*scm.FileEntry, *scm.Response, error) {
	namespace, name := scm.Split(repo)

	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimPrefix(ref, "refs/tags/")

	path = strings.TrimPrefix(path, "/")

	c, resp, err := s.client.GiteaClient.ListContents(namespace, name, ref, path)
	if err != nil {
		return nil, toSCMResponse(resp), err
	}
	return convertEntryList(c), toSCMResponse(resp), err

}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {

	namespace, name := scm.Split(repo)
	path = strings.TrimPrefix(path, "/")

	content := base64.StdEncoding.EncodeToString(params.Data)

	o := gitea.CreateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:    params.Message,
			BranchName: params.Branch,
		},
		Content: content,
	}

	_, resp, err := s.client.GiteaClient.CreateFile(namespace, name, path, o)
	return toSCMResponse(resp), err
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {

	namespace, name := scm.Split(repo)
	path = strings.TrimPrefix(path, "/")

	content := base64.StdEncoding.EncodeToString(params.Data)

	o := gitea.UpdateFileOptions{
		FileOptions: gitea.FileOptions{
			Message:    params.Message,
			BranchName: params.Branch,
		},
		Content: content,
		SHA:     params.Sha,
	}

	_, resp, err := s.client.GiteaClient.UpdateFile(namespace, name, path, o)
	return toSCMResponse(resp), err
}

func (s *contentService) Delete(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func convertEntryList(out []*gitea.ContentsResponse) []*scm.FileEntry {
	answer := make([]*scm.FileEntry, 0, len(out))
	for _, o := range out {
		answer = append(answer, convertEntry(o))
	}
	return answer
}

func convertEntry(from *gitea.ContentsResponse) *scm.FileEntry {
	link := ""
	if from.DownloadURL != nil {
		link = *from.DownloadURL
	}
	return &scm.FileEntry{
		Name: from.Name,
		Path: from.Path,
		Type: from.Type,
		Size: int(from.Size),
		Sha:  from.SHA,
		Link: link,
	}
}
