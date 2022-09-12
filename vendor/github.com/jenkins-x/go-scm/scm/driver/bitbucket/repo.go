// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bitbucket

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/jenkins-x/go-scm/scm"
)

type repositoryInput struct {
	SCM     string `json:"scm,omitempty"`
	Project struct {
		Key string `json:"key,omitempty"`
	} `json:"project,omitempty"`
	Private bool `json:"is_private,omitempty"`
}

type repository struct {
	UUID       string    `json:"uuid"`
	SCM        string    `json:"scm"`
	FullName   string    `json:"full_name"`
	IsPrivate  bool      `json:"is_private"`
	CreatedOn  time.Time `json:"created_on"`
	UpdatedOn  time.Time `json:"updated_on"`
	Mainbranch struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"mainbranch"`
}

type perms struct {
	Values []*perm `json:"values"`
}

type perm struct {
	Permissions string `json:"permission"`
}

type hooks struct {
	pagination
	Values []*hook `json:"values"`
}

type hook struct {
	Description          string   `json:"description"`
	URL                  string   `json:"url"`
	SkipCertVerification bool     `json:"skip_cert_verification"`
	Active               bool     `json:"active"`
	Events               []string `json:"events"`
	UUID                 string   `json:"uuid"`
}

type hookInput struct {
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Active      bool     `json:"active"`
	Events      []string `json:"events"`
}

type repositoryService struct {
	client *wrapper
}

type participants struct {
	pagination
	Values []*participant `json:"values"`
}

type participant struct {
	User       user   `json:"user"`
	Permission string `json:"permission"`
}

func (s *repositoryService) Create(ctx context.Context, input *scm.RepositoryInput) (*scm.Repository, *scm.Response, error) {
	workspace := input.Namespace
	if workspace == "" {
		workspace = s.client.Username
	}
	projectKey := ""
	if workspace == "" {
		user, res, err := s.client.Users.Find(ctx)
		if err != nil {
			return nil, res, errors.Wrapf(err, "failed to find current user")
		}
		workspace = user.Login
		if workspace == "" {
			return nil, nil, errors.Errorf("failed to find current user login")
		}
		s.client.Username = workspace
	}

	// lets allow the projectKey to be specified as workspace:projectKey
	words := strings.SplitN(workspace, ":", 2)
	if len(words) > 1 {
		workspace = words[0]
		projectKey = words[1]
	}
	path := fmt.Sprintf("2.0/repositories/%s/%s", workspace, input.Name)
	out := new(repository)
	in := new(repositoryInput)
	in.SCM = "git"
	in.Project.Key = projectKey
	in.Private = input.Private
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRepository(out), res, wrapError(res, err)
}

func wrapError(res *scm.Response, err error) error {
	if res == nil {
		return err
	}
	data, err2 := io.ReadAll(res.Body)
	if err2 != nil {
		return errors.Wrapf(err, "http status %d", res.Status)
	}
	return errors.Wrapf(err, "http status %d mesage %s", res.Status, string(data))
}

func (s *repositoryService) Fork(context.Context, *scm.RepositoryInput, string) (*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	statusList, resp, err := s.ListStatus(ctx, repo, ref, &scm.ListOptions{})
	if err != nil {
		return nil, resp, errors.Wrapf(err, "failed to list statuses")
	}

	combinedState := scm.StateUnknown

	byContext := make(map[string]*scm.Status)
	for _, s := range statusList {
		byContext[s.Target] = s
	}

	keys := make([]string, 0, len(byContext))
	for k := range byContext {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var statuses []*scm.Status
	for _, k := range keys {
		s := byContext[k]
		statuses = append(statuses, s)
	}

	for _, s := range statuses {
		// If we've still got a default state, or the state of the current status is worse than the current state, set it.
		if combinedState == scm.StateUnknown || combinedState > s.State {
			combinedState = s.State
		}
	}

	combined := &scm.CombinedStatus{
		State:    combinedState,
		Sha:      ref,
		Statuses: statuses,
	}
	return combined, resp, nil
}

func (s *repositoryService) FindUserPermission(ctx context.Context, repo, user string) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *repositoryService) AddCollaborator(ctx context.Context, repo, user, permission string) (bool, bool, *scm.Response, error) {
	// TODO lets fake out this method for now
	return true, false, nil, nil
}

func (s *repositoryService) IsCollaborator(ctx context.Context, repo, user string) (bool, *scm.Response, error) {
	// repo format: Workspace-slug/repository-slug
	wsname, reponame := scm.Split(repo)
	path := fmt.Sprintf("/2.0/workspaces/%s/permissions/repositories/%s?q=user.account_id=%q", wsname, reponame, user)

	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return false, res, err
	}

	// assume the response object has single entry
	if len(out.Values) == 1 {
		return (out.Values[0].Permission == "write" || out.Values[0].Permission == "admin"), res, err
	}

	return false, res, err
}

func (s *repositoryService) ListCollaborators(ctx context.Context, repo string, opts *scm.ListOptions) ([]scm.User, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	path := fmt.Sprintf("/2.0/workspaces/%s/permissions/repositories/%s?q=permission!=%q", namespace, name, "read")
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, wrapError(res, err)
	}
	err = copyPagination(out.pagination, res)
	return convertParticipants(out), res, wrapError(res, err)
}

func convertParticipants(participants *participants) []scm.User {
	answer := []scm.User{}
	for _, p := range participants.Values {
		answer = append(answer, *convertUser(&p.User))
	}
	return answer
}

func (s *repositoryService) ListLabels(context.Context, string, *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	return nil, nil, nil
}

func (s *repositoryService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

// Find returns the repository by name.
func (s *repositoryService) Find(ctx context.Context, repo string) (*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s", repo)
	out := new(repository)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepository(out), res, wrapError(res, err)
}

// FindHook returns a repository hook.
func (s *repositoryService) FindHook(ctx context.Context, repo, id string) (*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/hooks/%s", repo, id)
	out := new(hook)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertHook(out), res, wrapError(res, err)
}

// FindPerms returns the repository permissions.
func (s *repositoryService) FindPerms(ctx context.Context, repo string) (*scm.Perm, *scm.Response, error) {
	path := fmt.Sprintf("2.0/user/permissions/repositories?q=repository.full_name=%q", repo)
	out := new(perms)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertPerms(out), res, wrapError(res, err)
}

// List returns the user repository list.
func (s *repositoryService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories?%s", encodeListRoleOptions(opts))
	if opts.URL != "" {
		path = opts.URL
	}
	out := new(repositories)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, wrapError(res, err)
	}
	err = copyPagination(out.pagination, res)
	return convertRepositoryList(out), res, wrapError(res, err)
}

func (s *repositoryService) ListOrganisation(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s?%s", org, encodeListRoleOptions(opts))
	if opts.URL != "" {
		path = opts.URL
	}
	out := new(repositories)
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, wrapError(res, err)
	}
	err = copyPagination(out.pagination, res)
	return convertRepositoryList(out), res, wrapError(res, err)
}

func (s *repositoryService) ListUser(context.Context, string, *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// ListHooks returns a list or repository hooks.
func (s *repositoryService) ListHooks(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/hooks?%s", repo, encodeListOptions(opts))
	out := new(hooks)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, wrapError(res, err)
	}
	err = copyPagination(out.pagination, res)
	return convertHookList(out), res, wrapError(res, err)
}

// ListStatus returns a list of commit statuses.
func (s *repositoryService) ListStatus(ctx context.Context, repo, ref string, opts *scm.ListOptions) ([]*scm.Status, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/commit/%s/statuses?%s", repo, ref, encodeListOptions(opts))
	out := new(statuses)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, wrapError(res, err)
	}
	err = copyPagination(out.pagination, res)
	return convertStatusList(out), res, wrapError(res, err)
}

// CreateHook creates a new repository webhook.
func (s *repositoryService) CreateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	targetText := input.Target
	if input.Secret != "" {
		target, err := url.Parse(input.Target)
		if err != nil {
			return nil, nil, err
		}
		params := target.Query()
		params.Set("secret", input.Secret)
		target.RawQuery = params.Encode()
		targetText = target.String()
	}

	path := fmt.Sprintf("2.0/repositories/%s/hooks", repo)
	in := new(hookInput)
	in.URL = targetText
	in.Active = true
	in.Description = input.Name
	if in.Description == "" {
		in.Description = "my webhook"
	}
	// nolint
	in.Events = append(
		input.NativeEvents,
		convertHookEvents(input.Events)...,
	)
	out := new(hook)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertHook(out), res, wrapError(res, err)
}

func (s *repositoryService) UpdateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// CreateStatus creates a new commit status.
func (s *repositoryService) CreateStatus(ctx context.Context, repo, ref string, input *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/commit/%s/statuses/build", repo, ref)
	in := &status{
		State: convertFromState(input.State),
		Desc:  input.Desc,
		Key:   input.Label,
		Name:  input.Label,
		URL:   input.Target,
	}
	out := new(status)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertStatus(out), res, wrapError(res, err)
}

// DeleteHook deletes a repository webhook.
func (s *repositoryService) DeleteHook(ctx context.Context, repo, id string) (*scm.Response, error) {
	path := fmt.Sprintf("2.0/repositories/%s/hooks/%s", repo, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
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
	namespace, name := scm.Split(from.FullName)
	return &scm.Repository{
		ID:        from.UUID,
		Name:      name,
		Namespace: namespace,
		FullName:  from.FullName,
		Link:      fmt.Sprintf("https://bitbucket.org/%s", from.FullName),
		Branch:    from.Mainbranch.Name,
		Private:   from.IsPrivate,
		Clone:     fmt.Sprintf("https://bitbucket.org/%s.git", from.FullName),
		CloneSSH:  fmt.Sprintf("git@bitbucket.org:%s.git", from.FullName),
		Created:   from.CreatedOn,
		Updated:   from.UpdatedOn,
	}
}

func convertPerms(from *perms) *scm.Perm {
	to := new(scm.Perm)
	if len(from.Values) != 1 {
		return to
	}
	switch from.Values[0].Permissions {
	case "admin":
		to.Pull = true
		to.Push = true
		to.Admin = true
	case "write":
		to.Pull = true
		to.Push = true
	default:
		to.Pull = true
	}
	return to
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
		ID:         from.UUID,
		Name:       from.Description,
		Active:     from.Active,
		Target:     from.URL,
		Events:     from.Events,
		SkipVerify: from.SkipCertVerification,
	}
}

func convertHookEvents(from scm.HookEvents) []string {
	var events []string
	if from.Push {
		events = append(events, "repo:push")
	}
	if from.PullRequest {
		events = append(events, "pullrequest:created", "pullrequest:updated", "pullrequest:changes_request_created", "pullrequest:changes_request_removed", "pullrequest:approved", "pullrequest:unapproved", "pullrequest:fulfilled", "pullrequest:rejected")
	}
	if from.PullRequestComment {
		events = append(events, "pullrequest:comment_created", "pullrequest:comment_updated", "pullrequest:comment_deleted")
	}
	if from.Issue {
		events = append(events, "issue:created", "issue:updated")
	}
	if from.IssueComment {
		events = append(events, "issue:comment_created")
	}
	return events
}

type repositories struct {
	pagination
	Values []*repository `json:"values"`
}

type statuses struct {
	pagination
	Values []*status `json:"values"`
}

type status struct {
	State string `json:"state"`
	Key   string `json:"key"`
	Name  string `json:"name,omitempty"`
	URL   string `json:"url"`
	Desc  string `json:"description,omitempty"`
	Links *struct {
		Commit struct {
			Href string `json:"href,omitempty"`
		} `json:"commit,omitempty"`
	} `json:"links,omitempty"`
}

func convertStatusList(from *statuses) []*scm.Status {
	to := []*scm.Status{}
	for _, v := range from.Values {
		to = append(to, convertStatus(v))
	}
	return to
}

func convertStatus(from *status) *scm.Status {
	link := ""
	if from.Links != nil {
		link = from.Links.Commit.Href
	}
	return &scm.Status{
		State:  convertState(from.State),
		Label:  from.Key,
		Desc:   from.Desc,
		Target: from.URL,
		Link:   link,
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
