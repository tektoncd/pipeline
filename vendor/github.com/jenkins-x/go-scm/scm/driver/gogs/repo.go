// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gogs

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type repositoryService struct {
	client *wrapper
}

func (s *repositoryService) Create(context.Context, *scm.RepositoryInput) (*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) Fork(context.Context, *scm.RepositoryInput, string) (*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) FindUserPermission(ctx context.Context, repo, user string) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *repositoryService) AddCollaborator(ctx context.Context, repo, user, permission string) (bool, bool, *scm.Response, error) {
	return false, false, nil, scm.ErrNotSupported
}

func (s *repositoryService) IsCollaborator(ctx context.Context, repo, user string) (bool, *scm.Response, error) {
	return false, nil, scm.ErrNotSupported
}

func (s *repositoryService) ListCollaborators(ctx context.Context, repo string, opts *scm.ListOptions) ([]scm.User, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) ListLabels(context.Context, string, *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) Find(ctx context.Context, repo string) (*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s", repo)
	out := new(repository)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepository(out), res, err
}

func (s *repositoryService) FindHook(ctx context.Context, repo, id string) (*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/hooks/%s", repo, id)
	out := new(hook)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertHook(out), res, err
}

func (s *repositoryService) FindPerms(ctx context.Context, repo string) (*scm.Perm, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s", repo)
	out := new(repository)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepository(out).Perm, res, err
}

func (s *repositoryService) List(ctx context.Context, _ *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := "api/v1/user/repos"
	out := []*repository{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertRepositoryList(out), res, err
}

func (s *repositoryService) ListOrganisation(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/orgs/%s/repos", org)
	out := []*repository{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertRepositoryList(out), res, err
}

func (s *repositoryService) ListUser(ctx context.Context, username string, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/users/%s/repos", username)
	out := []*repository{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertRepositoryList(out), res, err
}

func (s *repositoryService) ListHooks(ctx context.Context, repo string, _ *scm.ListOptions) ([]*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/hooks", repo)
	out := []*hook{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertHookList(out), res, err
}

func (s *repositoryService) ListStatus(context.Context, string, string, *scm.ListOptions) ([]*scm.Status, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) CreateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/hooks", repo)
	in := new(hook)
	in.Type = "gogs"
	in.Active = true
	in.Config.Secret = input.Secret
	in.Config.ContentType = "json"
	in.Config.URL = input.Target
	// nolint
	in.Events = append(
		input.NativeEvents,
		convertHookEvent(input.Events)...,
	)
	out := new(hook)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertHook(out), res, err
}

func (s *repositoryService) UpdateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) CreateStatus(context.Context, string, string, *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) DeleteHook(ctx context.Context, repo, id string) (*scm.Response, error) {
	path := fmt.Sprintf("api/v1/repos/%s/hooks/%s", repo, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *repositoryService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

//
// native data structures
//

type (
	// gogs repository resource.
	repository struct {
		ID            int       `json:"id"`
		Owner         user      `json:"owner"`
		Name          string    `json:"name"`
		FullName      string    `json:"full_name"`
		Private       bool      `json:"private"`
		Fork          bool      `json:"fork"`
		HTMLURL       string    `json:"html_url"`
		SSHURL        string    `json:"ssh_url"`
		CloneURL      string    `json:"clone_url"`
		DefaultBranch string    `json:"default_branch"`
		CreatedAt     time.Time `json:"created_at"`
		UpdatedAt     time.Time `json:"updated_at"`
		Permissions   perm      `json:"permissions"`
	}

	// gogs permissions details.
	perm struct {
		Admin bool `json:"admin"`
		Push  bool `json:"push"`
		Pull  bool `json:"pull"`
	}

	// gogs hook resource.
	hook struct {
		ID     int        `json:"id"`
		Type   string     `json:"type"`
		Events []string   `json:"events"`
		Active bool       `json:"active"`
		Config hookConfig `json:"config"`
	}

	// gogs hook configuration details.
	hookConfig struct {
		URL         string `json:"url"`
		ContentType string `json:"content_type"`
		Secret      string `json:"secret"`
	}
)

//
// native data structure conversion
//

func convertRepositoryList(src []*repository) []*scm.Repository {
	var dst []*scm.Repository
	for _, v := range src {
		dst = append(dst, convertRepository(v))
	}
	return dst
}

func convertRepository(src *repository) *scm.Repository {
	return &scm.Repository{
		ID:        strconv.Itoa(src.ID),
		Namespace: userLogin(&src.Owner),
		Name:      src.Name,
		FullName:  src.FullName,
		Perm:      convertPerm(src.Permissions),
		Branch:    src.DefaultBranch,
		Private:   src.Private,
		Clone:     src.CloneURL,
		CloneSSH:  src.SSHURL,
	}
}

func convertPerm(src perm) *scm.Perm {
	return &scm.Perm{
		Push:  src.Push,
		Pull:  src.Pull,
		Admin: src.Admin,
	}
}

func convertHookList(src []*hook) []*scm.Hook {
	var dst []*scm.Hook
	for _, v := range src {
		dst = append(dst, convertHook(v))
	}
	return dst
}

func convertHook(from *hook) *scm.Hook {
	return &scm.Hook{
		ID:     strconv.Itoa(from.ID),
		Active: from.Active,
		Target: from.Config.URL,
		Events: from.Events,
	}
}

func convertHookEvent(from scm.HookEvents) []string {
	var events []string
	if from.PullRequest {
		events = append(events, "pull_request")
	}
	if from.Issue {
		events = append(events, "issues")
	}
	if from.IssueComment || from.PullRequestComment {
		events = append(events, "issue_comment")
	}
	if from.Branch || from.Tag {
		events = append(events, "create", "delete")
	}
	if from.Push {
		events = append(events, "push")
	}
	if from.Release {
		events = append(events, "release")
	}
	return events
}
