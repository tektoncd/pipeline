// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/jenkins-x/go-scm/scm"
)

type repository struct {
	Slug          string `json:"slug"`
	ID            int    `json:"id"`
	Name          string `json:"name"`
	ScmID         string `json:"scmId"`
	State         string `json:"state"`
	StatusMessage string `json:"statusMessage"`
	Forkable      bool   `json:"forkable"`
	Project       struct {
		Key    string `json:"key"`
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Public bool   `json:"public"`
		Type   string `json:"type"`
		Links  struct {
			Self []link `json:"self"`
		} `json:"links"`
	} `json:"project"`
	Public bool `json:"public"`
	Links  struct {
		Clone []link `json:"clone"`
		Self  []link `json:"self"`
	} `json:"links"`
}

type repositories struct {
	pagination
	Values []*repository `json:"values"`
}

type link struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type hooks struct {
	pagination
	Values []*hook `json:"values"`
}

type hook struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	CreatedDate int64    `json:"createdDate"`
	UpdatedDate int64    `json:"updatedDate"`
	Events      []string `json:"events"`
	URL         string   `json:"url"`
	Active      bool     `json:"active"`
	Config      struct {
		Secret string `json:"secret"`
	} `json:"configuration"`
}

type hookInput struct {
	Name   string   `json:"name"`
	Events []string `json:"events"`
	URL    string   `json:"url"`
	Active bool     `json:"active"`
	Config struct {
		Secret string `json:"secret"`
	} `json:"configuration"`
}

type status struct {
	State string `json:"state"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Desc  string `json:"description"`
}

type statuses struct {
	pagination
	Values []*status `json:"values"`
}

type participants struct {
	pagination
	Values []*participant `json:"values"`
}

type participant struct {
	User       user   `json:"user"`
	Permission string `json:"permission"`
}

type projGroups struct {
	pagination
	Values []*projGroup
}

type projGroup struct {
	Group      group  `json:"group"`
	Permission string `json:"permission"`
}

type group struct {
	Name string `json:"name"`
}

type members struct {
	pagination
	Values []*member `json:"values"`
}

type member struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type repositoryService struct {
	client *wrapper
}

type repoInput struct {
	Name   string `json:"name"`
	ScmID  string `json:"scmId"`
	Public bool   `json:"public"`
}

type addCollaboratorInput struct {
	Name       string `json:"name"`
	Permission string `json:"permission"`
}

func (s *repositoryService) Create(ctx context.Context, input *scm.RepositoryInput) (*scm.Repository, *scm.Response, error) {
	in := &repoInput{
		Name:   input.Name,
		ScmID:  "git",
		Public: !input.Private,
	}
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos", input.Namespace)

	out := new(repository)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRepository(out), res, err
}

type forkProjectInput struct {
	Key string `json:"key,omitempty"`
}

type forkInput struct {
	Slug    string            `json:"slug,omitempty"`
	Project *forkProjectInput `json:"project,omitempty"`
}

func (s *repositoryService) Fork(ctx context.Context, input *scm.RepositoryInput, origRepo string) (*scm.Repository, *scm.Response, error) {
	namespace, name := scm.Split(origRepo)
	in := new(forkInput)
	if input.Name != "" {
		in.Slug = input.Name
	}
	if input.Namespace != "" {
		in.Project = &forkProjectInput{Key: input.Namespace}
	}
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s", namespace, name)

	out := new(repository)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRepository(out), res, err
}

func (s *repositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	path := fmt.Sprintf("rest/build-status/1.0/commits/%s?orderBy=oldest", url.PathEscape(ref))
	out := new(statuses)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, err
	}

	combinedState := scm.StateUnknown

	byContext := make(map[string]*status)
	for _, s := range out.Values {
		byContext[s.Key] = s
	}

	keys := make([]string, 0, len(byContext))
	for k := range byContext {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var statuses []*scm.Status
	for _, k := range keys {
		s := byContext[k]
		statuses = append(statuses, convertStatus(s))
	}

	for _, s := range statuses {
		// If we've still got a default state, or the state of the current status is worse than the current state, set it.
		if combinedState == scm.StateUnknown || combinedState > s.State {
			combinedState = s.State
		}
	}

	return &scm.CombinedStatus{
		State:    combinedState,
		Sha:      ref,
		Statuses: statuses,
	}, res, err
}

func (s *repositoryService) FindUserPermission(ctx context.Context, repo, user string) (string, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/permissions/users?filter=%s", namespace, name, url.QueryEscape(user))
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return "", res, err
	}
	for _, p := range out.Values {
		if p.User.Name == user {
			return apiStringToPermission(p.Permission), res, nil
		}
	}
	return "", res, nil
}

func (s *repositoryService) AddCollaborator(ctx context.Context, repo, user, permission string) (bool, bool, *scm.Response, error) {
	existingPerm, res, err := s.FindUserPermission(ctx, repo, user)
	if err != nil {
		return false, false, res, err
	}
	if existingPerm == permission {
		return false, true, res, nil
	}

	apiPerm := permissionToAPIString(permission, false)
	if apiPerm == "" {
		return false, false, nil, fmt.Errorf("unknown permission '%s'", permission)
	}
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/permissions/users?&name=%s&permission=%s", namespace, name, user, apiPerm)
	input := &addCollaboratorInput{
		Name:       user,
		Permission: apiPerm,
	}
	res, err = s.client.do(ctx, "PUT", path, input, nil)
	if err != nil {
		return false, false, res, err
	}
	return true, false, res, nil
}

func (s *repositoryService) IsCollaborator(ctx context.Context, repo, user string) (bool, *scm.Response, error) {
	users, resp, err := s.ListCollaborators(ctx, repo, &scm.ListOptions{})
	if err != nil {
		return false, resp, err
	}
	for k := range users {
		if users[k].Name == user || users[k].Login == user {
			return true, resp, err
		}
	}
	return false, resp, err
}

func (s *repositoryService) ListCollaborators(ctx context.Context, repo string, opts *scm.ListOptions) ([]scm.User, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	opts.Size = 1000
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/permissions/users?%s", namespace, name, encodeListOptions(opts))
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertParticipants(out), res, err
}

func (s *repositoryService) ListLabels(context.Context, string, *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	// TODO implement me!
	return nil, nil, nil
}

// Find returns the repository by name.
func (s *repositoryService) Find(ctx context.Context, repo string) (*scm.Repository, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s", namespace, name)
	out := new(repository)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepository(out), res, err
}

// FindHook returns a repository hook.
func (s *repositoryService) FindHook(ctx context.Context, repo, id string) (*scm.Hook, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/webhooks/%s", namespace, name, id)
	out := new(hook)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertHook(out), res, err
}

// FindPerms returns the repository permissions.
func (s *repositoryService) FindPerms(ctx context.Context, repo string) (*scm.Perm, *scm.Response, error) {
	// HACK: test if the user has read access to the repository.
	_, _, err := s.Find(ctx, repo)
	if err != nil {
		return &scm.Perm{
			Pull:  false,
			Push:  false,
			Admin: false,
		}, nil, nil
	}

	// HACK: test if the user has admin access to the repository.
	_, _, err = s.ListHooks(ctx, repo, &scm.ListOptions{})
	if err == nil {
		return &scm.Perm{
			Pull:  true,
			Push:  true,
			Admin: true,
		}, nil, nil
	}
	// HACK: test if the user has write access to the repository.
	_, name := scm.Split(repo)
	repos, _, _ := s.listWrite(ctx, repo)
	for _, repo := range repos {
		if repo.Name == name {
			return &scm.Perm{
				Pull:  true,
				Push:  true,
				Admin: false,
			}, nil, nil
		}
	}

	return &scm.Perm{
		Pull:  true,
		Push:  false,
		Admin: false,
	}, nil, nil
}

// List returns the user repository list.
func (s *repositoryService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("rest/api/1.0/repos?%s", encodeListRoleOptions(opts))
	out := new(repositories)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertRepositoryList(out), res, err
}

func (s *repositoryService) ListOrganisation(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos?%s", org, encodeListRoleOptions(opts))
	out := new(repositories)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertRepositoryList(out), res, err
}

func (s *repositoryService) ListUser(context.Context, string, *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// listWrite returns the user repository list.
func (s *repositoryService) listWrite(ctx context.Context, repo string) ([]*scm.Repository, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/repos?size=1000&permission=REPO_WRITE&project=%s&name=%s", namespace, name)
	out := new(repositories)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepositoryList(out), res, err
}

// ListHooks returns a list or repository hooks.
func (s *repositoryService) ListHooks(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Hook, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/webhooks?%s", namespace, name, encodeListOptions(opts))
	out := new(hooks)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertHookList(out), res, err
}

// ListStatus returns a list of commit statuses.
func (s *repositoryService) ListStatus(ctx context.Context, _, ref string, opts *scm.ListOptions) ([]*scm.Status, *scm.Response, error) {
	path := fmt.Sprintf("rest/build-status/1.0/commits/%s?%s", url.PathEscape(ref), encodeListOptions(opts))
	out := new(statuses)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertStatusList(out), res, err
}

// CreateHook creates a new repository webhook.
func (s *repositoryService) CreateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/webhooks", namespace, name)
	in := new(hookInput)
	in.URL = input.Target
	in.Active = true
	in.Name = input.Name
	in.Config.Secret = input.Secret
	// nolint
	in.Events = append(
		input.NativeEvents,
		convertHookEvents(input.Events)...,
	)
	out := new(hook)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertHook(out), res, err
}

func (s *repositoryService) UpdateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// CreateStatus creates a new commit status.
// reference: https://developer.atlassian.com/server/bitbucket/how-tos/updating-build-status-for-commits/
func (s *repositoryService) CreateStatus(ctx context.Context, repo, ref string, input *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	path := fmt.Sprintf("rest/build-status/1.0/commits/%s", url.PathEscape(ref))
	in := status{
		State: convertFromState(input.State),
		Key:   input.Label,
		Name:  input.Label,
		URL:   input.Target,
		Desc:  input.Desc,
	}
	res, err := s.client.do(ctx, "POST", path, in, nil)
	return &scm.Status{
		State:  input.State,
		Label:  input.Label,
		Desc:   input.Desc,
		Target: input.Target,
	}, res, err
}

// DeleteHook deletes a repository webhook.
func (s *repositoryService) DeleteHook(ctx context.Context, repo, id string) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("rest/api/1.0/projects/%s/repos/%s/webhooks/%s", namespace, name, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *repositoryService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

// helper function to convert from the gogs repository list to
// the common repository structure.
func convertRepositoryList(from *repositories) []*scm.Repository {
	to := []*scm.Repository{}
	for _, v := range from.Values {
		to = append(to, convertRepository(v))
	}
	return to
}

// helper function to convert from the gogs repository structure
// to the common repository structure.
func convertRepository(from *repository) *scm.Repository {
	return &scm.Repository{
		ID:        strconv.Itoa(from.ID),
		Name:      from.Slug,
		Namespace: from.Project.Key,
		FullName:  fmt.Sprintf("%s/%s", from.Project.Key, from.Slug),
		Link:      extractSelfLink(from.Links.Self),
		Branch:    "master",
		Private:   !from.Public,
		CloneSSH:  extractLink(from.Links.Clone, "ssh"),
		Clone:     anonymizeLink(extractLink(from.Links.Clone, "http")),
	}
}

func extractLink(links []link, name string) (href string) {
	for _, link := range links {
		if link.Name == name {
			return link.Href
		}
	}
	return
}

func extractSelfLink(links []link) (href string) {
	for _, link := range links {
		return link.Href
	}
	return
}

func anonymizeLink(link string) (href string) {
	parsed, err := url.Parse(link)
	if err != nil {
		return link
	}
	parsed.User = nil
	return parsed.String()
}

func convertHookList(from *hooks) []*scm.Hook {
	to := []*scm.Hook{}
	for _, v := range from.Values {
		to = append(to, convertHook(v))
	}
	return to
}

func convertHook(from *hook) *scm.Hook {
	return &scm.Hook{
		ID:     strconv.Itoa(from.ID),
		Name:   from.Name,
		Active: from.Active,
		Target: from.URL,
		Events: from.Events,
	}
}

func convertHookEvents(from scm.HookEvents) []string {
	var events []string
	if from.Push || from.Branch || from.Tag {
		events = append(events, "repo:refs_changed")
	}
	if from.PullRequest {
		events = append(events, "pr:declined", "pr:modified", "pr:deleted", "pr:opened", "pr:merged", "pr:from_ref_updated")
	}
	if from.PullRequestComment {
		events = append(events, "pr:comment:added", "pr:comment:deleted", "pr:comment:edited")
	}
	return events
}

func convertStatusList(from *statuses) []*scm.Status {
	to := []*scm.Status{}
	for _, v := range from.Values {
		to = append(to, convertStatus(v))
	}
	return to
}

func convertStatus(from *status) *scm.Status {
	return &scm.Status{
		State:  convertState(from.State),
		Label:  from.Key,
		Desc:   from.Desc,
		Target: from.URL,
	}
}

func convertFromState(from scm.State) string {
	switch from {
	case scm.StatePending, scm.StateRunning:
		return "INPROGRESS"
	case scm.StateSuccess:
		return "SUCCESSFUL"
	default:
		return "FAILED"
	}
}

func convertState(from string) scm.State {
	switch from {
	case "FAILED":
		return scm.StateFailure
	case "INPROGRESS":
		return scm.StatePending
	case "SUCCESSFUL":
		return scm.StateSuccess
	default:
		return scm.StateUnknown
	}
}

func convertParticipants(participants *participants) []scm.User {
	answer := []scm.User{}
	for _, p := range participants.Values {
		answer = append(answer, *convertUser(&p.User))
	}
	return answer
}

func apiStringToPermission(apiString string) string {
	switch apiString {
	case "ADMIN", "PROJECT_ADMIN", "REPO_ADMIN":
		return scm.AdminPermission
	case "PROJECT_CREATE", "PROJECT_WRITE", "REPO_WRITE":
		return scm.WritePermission
	case "PROJECT_READ", "REPO_READ":
		return scm.ReadPermission
	default:
		return scm.NoPermission
	}
}

func permissionToAPIString(perm string, isProject bool) string {
	prefix := "REPO"
	if isProject {
		prefix = "PROJECT"
	}
	switch perm {
	case scm.AdminPermission:
		return prefix + "_ADMIN"
	case scm.WritePermission:
		return prefix + "_WRITE"
	case scm.ReadPermission:
		return prefix + "_READ"
	default:
		return ""
	}
}
