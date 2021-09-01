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
	"github.com/pkg/errors"
)

const (
	noPermissions         = 0
	guestPermissions      = 10
	reporterPermissions   = 20
	developerPermissions  = 30
	maintainerPermissions = 40
	ownerPermissions      = 50

	privateVisibility  = "private"
	internalVisibility = "internal"
	publicVisibility   = "public"
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

type memberPermissions struct {
	UserID      int `json:"user_id"`
	AccessLevel int `json:"access_level"`
}

func stringToAccessLevel(perm string) int {
	switch perm {
	case scm.AdminPermission:
		// owner permission only applies to groups, so fails for projects
		return maintainerPermissions
	case scm.WritePermission:
		return developerPermissions
	case scm.ReadPermission:
		return guestPermissions
	default:
		return noPermissions
	}
}

func accessLevelToString(level int) string {
	switch level {
	case 50:
		return scm.AdminPermission
	case 40, 30:
		return scm.WritePermission
	case 20, 10:
		return scm.ReadPermission
	default:
		return scm.NoPermission
	}
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

type member struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	Name        string `json:"name"`
	State       string `json:"state"`
	AccessLevel int    `json:"access_level"`
	WebURL      string `json:"web_url"`
	AvatarURL   string `json:"avatar_url"`
	// Fields to be figured out later
	// expires_at - a yyyy-mm-dd date
	// group_saml_identity

}
type repositoryService struct {
	client *wrapper
}

type repositoryInput struct {
	Name        string `json:"name"`
	NamespaceID int    `json:"namespace_id"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility"`
}

func (s *repositoryService) Create(ctx context.Context, input *scm.RepositoryInput) (*scm.Repository, *scm.Response, error) {
	namespace, err := s.client.findNamespaceByName(ctx, input.Namespace)
	if err != nil {
		return nil, nil, err
	}
	if namespace == nil {
		return nil, nil, fmt.Errorf("no namespace found for %s", input.Namespace)
	}
	in := new(repositoryInput)
	in.Name = input.Name
	in.Description = input.Description
	in.NamespaceID = namespace.ID

	if input.Private {
		in.Visibility = privateVisibility
	} else {
		in.Visibility = publicVisibility
	}

	path := "/api/v4/projects"
	out := new(repository)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRepository(out), res, err
}

type forkInput struct {
	Namespace string `json:"namespace_path,omitempty"`
	Name      string `json:"name,omitempty"`
}

func (s *repositoryService) Fork(ctx context.Context, input *scm.RepositoryInput, origRepo string) (*scm.Repository, *scm.Response, error) {
	in := new(forkInput)
	in.Name = input.Name
	in.Namespace = input.Namespace

	path := fmt.Sprintf("/api/v4/projects/%s/fork", encode(origRepo))
	out := new(repository)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertRepository(out), res, err
}

func (s *repositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	var resp *scm.Response
	var statuses []*scm.Status
	var statusesByPage []*scm.Status
	var err error
	firstRun := false
	opts := scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (resp != nil && opts.Page <= resp.Page.Last) {
		statusesByPage, resp, err = s.ListStatus(ctx, repo, ref, opts)
		if err != nil {
			return nil, resp, err
		}
		statuses = append(statuses, statusesByPage...)
		firstRun = true
		opts.Page++
	}

	combinedState := scm.StateUnknown

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
	}, resp, err
}

func (s *repositoryService) FindUserPermission(ctx context.Context, repo string, user string) (string, *scm.Response, error) {
	var resp *scm.Response
	var err error
	firstRun := false
	opts := scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (resp != nil && opts.Page <= resp.Page.Last) {
		path := fmt.Sprintf("api/v4/projects/%s/members/all?%s", encode(repo), encodeListOptions(opts))
		out := []*member{}
		resp, err = s.client.do(ctx, "GET", path, nil, &out)
		if err != nil {
			return scm.NoPermission, resp, err
		}
		firstRun = true
		for _, u := range out {
			if u.Username == user {
				return accessLevelToString(u.AccessLevel), resp, nil
			}
		}
		opts.Page++
	}
	return scm.NoPermission, resp, nil
}

func (s *repositoryService) AddCollaborator(ctx context.Context, repo, username, permission string) (bool, bool, *scm.Response, error) {
	userData, _, err := s.client.Users.FindLogin(ctx, username)
	if err != nil {
		return false, false, nil, errors.Wrapf(err, "couldn't look up ID for user %s", username)
	}
	if userData == nil {
		return false, false, nil, fmt.Errorf("no user for %s found", username)
	}
	path := fmt.Sprintf("api/v4/projects/%s/members", encode(repo))
	out := new(user)
	in := &memberPermissions{
		UserID:      userData.ID,
		AccessLevel: stringToAccessLevel(permission),
	}
	res, err := s.client.do(ctx, "POST", path, in, &out)
	if err != nil {
		return false, false, res, err
	}
	return true, false, res, nil
}

func (s *repositoryService) IsCollaborator(ctx context.Context, repo, user string) (bool, *scm.Response, error) {
	var resp *scm.Response
	var users []scm.User
	var err error
	firstRun := false
	opts := scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (resp != nil && opts.Page <= resp.Page.Last) {
		users, resp, err = s.ListCollaborators(ctx, repo, opts)
		if err != nil {
			return false, resp, err
		}
		firstRun = true
		for _, u := range users {
			if u.Name == user || u.Login == user {
				return true, resp, err
			}
		}
		opts.Page++
	}
	return false, resp, err
}

func (s *repositoryService) ListCollaborators(ctx context.Context, repo string, ops scm.ListOptions) ([]scm.User, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/members/all?%s", encode(repo), encodeListOptions(ops))
	out := []*user{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertUserList(out), res, err
}

func (s *repositoryService) ListLabels(ctx context.Context, repo string, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/labels?%s", encode(repo), encodeListOptions(opts))
	out := []*label{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertLabelObjects(out), res, err
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

func (s *repositoryService) ListOrganisation(context.Context, string, scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) ListUser(context.Context, string, scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
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
	hasStarEvents := false
	for _, event := range input.NativeEvents {
		if event == "*" {
			hasStarEvents = true
		}
	}
	if input.Events.Branch {
		// no-op
	}
	if input.Events.Issue || hasStarEvents {
		params.Set("issues_events", "true")
	}
	if input.Events.IssueComment ||
		input.Events.PullRequestComment || hasStarEvents {
		params.Set("note_events", "true")
	}
	if input.Events.PullRequest || hasStarEvents {
		params.Set("merge_requests_events", "true")
	}
	if input.Events.Push || input.Events.Branch || hasStarEvents {
		params.Set("push_events", "true")
	}
	if input.Events.Tag || hasStarEvents {
		params.Set("tag_push_events", "true")
	}
	if input.Events.Release || hasStarEvents {
		params.Set("releases_events", "true")
	}

	path := fmt.Sprintf("api/v4/projects/%s/hooks?%s", encode(repo), params.Encode())
	out := new(hook)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertHook(out), res, err
}

func (s *repositoryService) UpdateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	params := url.Values{}
	hookID := input.Name
	params.Set("url", input.Target)
	if input.Secret != "" {
		params.Set("token", input.Secret)
	}
	if input.SkipVerify {
		params.Set("enable_ssl_verification", "true")
	}
	hasStarEvents := false
	for _, event := range input.NativeEvents {
		if event == "*" {
			hasStarEvents = true
		}
	}
	if input.Events.Branch {
		// no-op
	}
	if input.Events.Issue || hasStarEvents {
		params.Set("issues_events", "true")
	} else {
		params.Set("issues_events", "false")
	}
	if input.Events.IssueComment ||
		input.Events.PullRequestComment || hasStarEvents {
		params.Set("note_events", "true")
	} else {
		params.Set("note_events", "false")
	}
	if input.Events.PullRequest || hasStarEvents {
		params.Set("merge_requests_events", "true")
	} else {
		params.Set("merge_requests_events", "false")
	}
	if input.Events.Push || input.Events.Branch || hasStarEvents {
		params.Set("push_events", "true")
	} else {
		params.Set("push_events", "false")
	}
	if input.Events.Tag || hasStarEvents {
		params.Set("tag_push_events", "true")
	} else {
		params.Set("tag_push_events", "false")
	}

	path := fmt.Sprintf("api/v4/projects/%s/hooks/%s?%s", encode(repo), hookID, params.Encode())
	out := new(hook)
	res, err := s.client.do(ctx, "PUT", path, nil, out)
	return convertHook(out), res, err
}

func (s *repositoryService) CreateStatus(ctx context.Context, repo, ref string, input *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	params := url.Values{}
	params.Set("state", convertFromState(input.State))
	params.Set("name", input.Label)
	params.Set("target_url", input.Target)
	params.Set("description", input.Desc)
	var repoID string
	// TODO to add a cache to reduce unnecessary network request
	if repository, response, err := s.Find(ctx, repo); err == nil {
		repoID = repository.ID
	} else {
		return nil, response, errors.Errorf("cannot found repository %s, %v", repo, err)
	}
	path := fmt.Sprintf("api/v4/projects/%s/statuses/%s?%s", repoID, ref, params.Encode())
	out := new(status)
	res, err := s.client.do(ctx, "POST", path, nil, out)
	return convertStatus(out), res, err
}

func (s *repositoryService) DeleteHook(ctx context.Context, repo string, id string) (*scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/hooks/%s", encode(repo), id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *repositoryService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
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
		FullName:  from.PathNamespace,
		Branch:    from.DefaultBranch,
		Private:   convertPrivate(from.Visibility),
		Clone:     from.HTTPURL,
		CloneSSH:  from.SSHURL,
		Link:      from.WebURL,
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
		labels = append(labels, convertLabel(label))
	}
	return labels
}

func convertLabel(from *label) *scm.Label {
	return &scm.Label{
		ID:          int64(from.ID),
		Name:        from.Name,
		Description: from.Description,
		Color:       from.Color,
	}
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
