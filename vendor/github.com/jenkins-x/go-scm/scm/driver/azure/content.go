// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

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
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/items/get?view=azure-devops-rest-6.0
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items?path=%s&includeContent=true&$format=json", ro.org, ro.project, ro.name, path)
	endpoint += generateURIFromRef(ref)
	endpoint += "&api-version=6.0"
	out := new(content)
	res, err := s.client.do(ctx, "GET", endpoint, nil, out)
	data := []byte(out.Content)
	return &scm.Content{
		Path:   out.Path,
		Data:   data,
		Sha:    out.CommitID,
		BlobID: out.ObjectID,
	}, res, err
}

func (s *contentService) Create(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pushes?api-version=6.0", ro.org, ro.project, ro.name)
	ref := refUpdate{
		Name:        SanitizeBranchName(params.Branch),
		OldObjectID: params.Ref,
	}
	cha := change{
		ChangeType: "add",
	}
	cha.Item.Path = path
	cha.NewContent.Content = base64.StdEncoding.EncodeToString(params.Data)
	cha.NewContent.ContentType = "base64encoded"

	com := commitRef{
		Comment: params.Message,
		Changes: []change{cha},
	}
	in := &contentCreateUpdate{
		RefUpdates: []refUpdate{ref},
		Commits:    []commitRef{com},
	}

	res, err := s.client.do(ctx, "POST", endpoint, in, nil)
	return res, err
}

func (s *contentService) Update(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pushes?api-version=6.0", ro.org, ro.project, ro.name)
	ref := refUpdate{
		Name:        SanitizeBranchName(params.Branch),
		OldObjectID: params.Sha,
	}
	cha := change{
		ChangeType: "edit",
	}
	cha.Item.Path = path
	cha.NewContent.Content = base64.StdEncoding.EncodeToString(params.Data)
	cha.NewContent.ContentType = "base64encoded"

	com := commitRef{
		Comment: params.Message,
		Changes: []change{cha},
	}
	in := &contentCreateUpdate{
		RefUpdates: []refUpdate{ref},
		Commits:    []commitRef{com},
	}

	res, err := s.client.do(ctx, "POST", endpoint, in, nil)
	return res, err
}

func (s *contentService) Delete(ctx context.Context, repo, path string, params *scm.ContentParams) (*scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/pushes?api-version=6.0", ro.org, ro.project, ro.name)
	ref := refUpdate{
		Name:        SanitizeBranchName(params.Branch),
		OldObjectID: params.Sha,
	}
	change1 := change{
		ChangeType: "delete",
	}
	change1.Item.Path = path
	com := commitRef{
		Comment: params.Message,
		Changes: []change{change1},
	}
	in := &contentCreateUpdate{
		RefUpdates: []refUpdate{ref},
		Commits:    []commitRef{com},
	}

	res, err := s.client.do(ctx, "POST", endpoint, in, nil)
	return res, err
}

func (s *contentService) List(ctx context.Context, repo, path, ref string) ([]*scm.FileEntry, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/items/list?view=azure-devops-rest-6.0
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s/items?path=%s&recursionLevel=Full&$format=json", ro.org, ro.project, ro.name, path)
	endpoint += generateURIFromRef(ref)
	out := new(contentList)
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)
	return convertFileEntryList(out.Value), res, err
}

type content struct {
	ObjectID      string `json:"objectId"`
	GitObjectType string `json:"gitObjectType"`
	CommitID      string `json:"commitId"`
	Path          string `json:"path"`
	Content       string `json:"content"`
	URL           string `json:"url"`
	Links         struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
		Repository struct {
			Href string `json:"href"`
		} `json:"repository"`
		Blob struct {
			Href string `json:"href"`
		} `json:"blob"`
	} `json:"_links"`
}

type contentList struct {
	Count int        `json:"count"`
	Value []*content `json:"value"`
}
type refUpdate struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId,omitempty"`
}
type change struct {
	ChangeType string `json:"changeType"`
	Item       struct {
		Path string `json:"path"`
	} `json:"item"`
	NewContent struct {
		Content     string `json:"content,omitempty"`
		ContentType string `json:"contentType,omitempty"`
	} `json:"newContent,omitempty"`
}
type commitRef struct {
	Comment  string   `json:"comment,omitempty"`
	Changes  []change `json:"changes,omitempty"`
	CommitID string   `json:"commitId"`
	URL      string   `json:"url"`
}
type contentCreateUpdate struct {
	RefUpdates []refUpdate `json:"refUpdates"`
	Commits    []commitRef `json:"commits"`
}

func convertFileEntryList(from []*content) []*scm.FileEntry {
	to := []*scm.FileEntry{}
	for _, v := range from {
		to = append(to, convertFileEntry(v))
	}
	return to
}
func convertFileEntry(from *content) *scm.FileEntry {
	return &scm.FileEntry{Path: from.Path, Sha: from.CommitID}
}

func generateURIFromRef(ref string) (uri string) {
	if ref != "" {
		if len(ref) == 40 {
			return fmt.Sprintf("&versionDescriptor.versionType=commit&versionDescriptor.version=%s", ref)
		}
		return fmt.Sprintf("&versionDescriptor.versionType=branch&versionDescriptor.version=%s", ref)
	}
	return ""
}
