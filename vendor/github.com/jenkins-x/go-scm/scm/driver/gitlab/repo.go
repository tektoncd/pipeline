// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type repository struct {
	ID            int         `json:"id"`
	Path          string      `json:"path"`
	PathNamespace string      `json:"path_with_namespace"`
	DefaultBranch string      `json:"default_branch"`
	Visibility    string      `json:"visibility"`
	WebURL        string      `json:"web_url"`
	SSHURL        string      `json:"ssh_url_to_repo"`
	HTTPURL       string      `json:"http_url_to_repo"`
	Namespace     namespace   `json:"namespace"`
	Permissions   permissions `json:"permissions"`
}

type namespace struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type permissions struct {
	ProjectAccess access `json:"project_access"`
	GroupAccess   access `json:"group_access"`
}

type access struct {
	AccessLevel       int `json:"access_level"`
	NotificationLevel int `json:"notification_level"`
}

type hook struct {
	ID                    int       `json:"id"`
	URL                   string    `json:"url"`
	ProjectID             int       `json:"project_id"`
	PushEvents            bool      `json:"push_events"`
	IssuesEvents          bool      `json:"issues_events"`
	MergeRequestsEvents   bool      `json:"merge_requests_events"`
	TagPushEvents         bool      `json:"tag_push_events"`
	NoteEvents            bool      `json:"note_events"`
	JobEvents             bool      `json:"job_events"`
	PipelineEvents        bool      `json:"pipeline_events"`
	WikiPageEvents        bool      `json:"wiki_page_events"`
	EnableSslVerification bool      `json:"enable_ssl_verification"`
	CreatedAt             time.Time `json:"created_at"`
}

type label struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type repositoryService struct {
	client *wrapper
}

func (s *repositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) FindUserPermission(ctx context.Context, repo string, user string) (string, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) IsCollaborator(ctx context.Context, repo, user string) (bool, *scm.Response, error) {
	users, resp, err := s.ListCollaborators(ctx, repo)
	if err != nil {
		return false, resp, err
	}
	for _, u := range users {
		if u.Name == user || u.Login == user {
			return true, resp, err
		}
	}
	return false, resp, err
}

func (s *repositoryService) ListCollaborators(ctx context.Context, repo string) ([]scm.User, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/members/all", encode(repo))
	out := []*user{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertUserList(out), res, err
}

func (s *repositoryService) ListLabels(context.Context, string, scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) Find(ctx context.Context, repo string) (*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s", encode(repo))
	out := new(repository)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepository(out), res, err
}

func (s *repositoryService) FindHook(ctx context.Context, repo string, id string) (*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/hooks/%s", encode(repo), id)
	out := new(hook)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertHook(out), res, err
}

func (s *repositoryService) FindPerms(ctx context.Context, repo string) (*scm.Perm, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s", encode(repo))
	out := new(repository)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertRepository(out).Perm, res, err
}

func (s *repositoryService) List(ctx context.Context, opts scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects?%s", encodeMemberListOptions(opts))
	out := []*repository{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertRepositoryList(out), res, err
}

func (s *repositoryService) ListHooks(ctx context.Context, repo string, opts scm.ListOptions) ([]*scm.Hook, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/hooks?%s", encode(repo), encodeListOptions(opts))
	out := []*hook{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertHookList(out), res, err
}

func (s *repositoryService) ListStatus(ctx context.Context, repo, ref string, opts scm.ListOptions) ([]*scm.Status, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/repository/commits/%s/statuses?%s", encode(repo), ref, encodeListOptions(opts))
	out := []*status{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertStatusList(out), res, err
}

func (s *repositoryService) CreateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	params := url.Values{}
	params.Set("url", input.Target)
	if input.Secret != "" {
		params.Set("token", input.Secret)
	}
	if input.SkipVerify {
		params.Set("enable_ssl_verification", "true")
	}
	if input.Events.Branch {
		// no-op
	}
	if input.Events.Issue {
		params.Set("issues_events", "true")
	}
	if input.Events.IssueComment ||
		input.Events.PullRequestComment {
		params.Set("note_events", "true")
	}
	if input.Events.PullRequest {
		params.Set("merge_requests_events", "true")
	}
	if input.Events.Push || input.Events.Branch {
		params.Set("push_events", "true")
	}
	if input.Events.Tag {
		params.Set("tag_push_events", "true")
	}

	path := fmt.Sprintf("api/v4/projects/%s/hooks?%s", encode(repo), params.Encode())
	out := new(hook)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertHook(out), res, err
}

func (s *repositoryService) CreateStatus(ctx context.Context, repo, ref string, input *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	params := url.Values{}
	params.Set("state", convertFromState(input.State))
	params.Set("name", input.Label)
	params.Set("target_url", input.Target)
	path := fmt.Sprintf("api/v4/projects/%s/statuses/%s?%s", encode(repo), ref, params.Encode())
	out := new(status)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertStatus(out), res, err
}

func (s *repositoryService) DeleteHook(ctx context.Context, repo string, id string) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/hooks/%s", encode(repo), id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

// helper function to convert from the gogs repository list to
// the common repository structure.
func convertRepositoryList(from []*repository) []*scm.Repository {
	to := []*scm.Repository{}
	for _, v := range from {
		to = append(to, convertRepository(v))
	}
	return to
}

// helper function to convert from the gogs repository structure
// to the common repository structure.
func convertRepository(from *repository) *scm.Repository {
	to := &scm.Repository{
		ID:        strconv.Itoa(from.ID),
		Namespace: from.Namespace.Path,
		Name:      from.Path,
		Branch:    from.DefaultBranch,
		Private:   convertPrivate(from.Visibility),
		Clone:     from.HTTPURL,
		CloneSSH:  from.SSHURL,
		Perm: &scm.Perm{
			Pull:  true,
			Push:  canPush(from),
			Admin: canAdmin(from),
		},
	}
	if to.Namespace == "" {
		if parts := strings.SplitN(from.PathNamespace, "/", 2); len(parts) == 2 {
			to.Namespace = parts[1]
		}
	}
	return to
}

func convertHookList(from []*hook) []*scm.Hook {
	to := []*scm.Hook{}
	for _, v := range from {
		to = append(to, convertHook(v))
	}
	return to
}

func convertHook(from *hook) *scm.Hook {
	return &scm.Hook{
		ID:         strconv.Itoa(from.ID),
		Active:     true,
		Target:     from.URL,
		Events:     convertEvents(from),
		SkipVerify: !from.EnableSslVerification,
	}
}

type status struct {
	Name    string      `json:"name"`
	Desc    null.String `json:"description"`
	Status  string      `json:"status"`
	Sha     string      `json:"sha"`
	Ref     string      `json:"ref"`
	Target  null.String `json:"target_url"`
	Created time.Time   `json:"created_at"`
	Updated time.Time   `json:"updated_at"`
}

func convertStatusList(from []*status) []*scm.Status {
	to := []*scm.Status{}
	for _, v := range from {
		to = append(to, convertStatus(v))
	}
	return to
}

func convertStatus(from *status) *scm.Status {
	return &scm.Status{
		State:  convertState(from.Status),
		Label:  from.Name,
		Desc:   from.Desc.String,
		Target: from.Target.String,
	}
}

func convertEvents(from *hook) []string {
	var events []string
	if from.IssuesEvents {
		events = append(events, "issues")
	}
	if from.TagPushEvents {
		events = append(events, "tag")
	}
	if from.PushEvents {
		events = append(events, "push")
	}
	if from.NoteEvents {
		events = append(events, "comment")
	}
	if from.MergeRequestsEvents {
		events = append(events, "merge")
	}
	return events
}

func convertState(from string) scm.State {
	switch from {
	case "canceled":
		return scm.StateCanceled
	case "failed":
		return scm.StateFailure
	case "pending":
		return scm.StatePending
	case "running":
		return scm.StateRunning
	case "success":
		return scm.StateSuccess
	default:
		return scm.StateUnknown
	}
}

func convertFromState(from scm.State) string {
	switch from {
	case scm.StatePending:
		return "pending"
	case scm.StateRunning:
		return "running"
	case scm.StateSuccess:
		return "success"
	case scm.StateCanceled:
		return "canceled"
	default:
		return "failed"
	}
}

func convertPrivate(from string) bool {
	switch from {
	case "public", "":
		return false
	default:
		return true
	}
}

func convertLabelObjects(from []*label) []*scm.Label {
	var labels []*scm.Label
	for _, label := range from {
		labels = append(labels, &scm.Label{
			Name:        label.Name,
			Description: label.Description,
			Color:       label.Color,
		})
	}
	return labels
}

func canPush(proj *repository) bool {
	switch {
	case proj.Permissions.ProjectAccess.AccessLevel >= 30:
		return true
	case proj.Permissions.GroupAccess.AccessLevel >= 30:
		return true
	default:
		return false
	}
}

func canAdmin(proj *repository) bool {
	switch {
	case proj.Permissions.ProjectAccess.AccessLevel >= 40:
		return true
	case proj.Permissions.GroupAccess.AccessLevel >= 40:
		return true
	default:
		return false
	}
}
