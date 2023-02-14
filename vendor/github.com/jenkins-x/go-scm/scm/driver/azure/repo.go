// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

// RepositoryService implements the repository service for
// the GitHub driver.
type RepositoryService struct {
	client *wrapper
}

// ListOrganisation lists all the repos for an org or specific project in an org
// org can be in the form "<org>" or "<org/project>"
func (s *RepositoryService) ListOrganisation(ctx context.Context, org string, options *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	orgTrimmed := strings.Trim(org, "/")
	orgParts := strings.Split(orgTrimmed, "/")

	if len(orgParts) > 2 || len(orgParts) < 1 {
		return nil, nil, fmt.Errorf("expected org in form <organization> or <organization>/<project>, but got %s", orgTrimmed)
	}

	for _, s := range orgParts {
		if s == "" {
			return nil, nil, fmt.Errorf("expected org in form <organization> or <organization>/<project>, but got %s", orgTrimmed)
		}
	}

	endpoint := fmt.Sprintf("%s/_apis/git/repositories?api-version=6.0", orgTrimmed)

	out := new(repositories)
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)

	return convertRepositoryList(out, options), res, err
}

func (s *RepositoryService) ListUser(ctx context.Context, s2 string, options *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *RepositoryService) ListLabels(ctx context.Context, s2 string, options *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *RepositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *RepositoryService) Create(ctx context.Context, input *scm.RepositoryInput) (*scm.Repository, *scm.Response, error) {
	ro, err := decodeRepo(input.Namespace + "/" + input.Name)
	if err != nil {
		return nil, nil, err
	}

	proj, res, err := s.getProject(ctx, ro)
	if err != nil {
		return nil, res, err
	}

	in := repository{
		Name: input.Name,
		Project: struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			State string `json:"state"`
			URL   string `json:"url"`
		}{
			ID: proj.ID,
		},
	}
	out := new(repository)
	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories?api-version=6.0", ro.org, ro.project)

	res, err = s.client.do(ctx, "POST", endpoint, &in, &out)
	return convertRepository(out), res, err
}

func (s *RepositoryService) Fork(ctx context.Context, input *scm.RepositoryInput, s2 string) (*scm.Repository, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *RepositoryService) IsCollaborator(ctx context.Context, repo, user string) (bool, *scm.Response, error) {
	return false, nil, scm.ErrNotSupported
}

func (s *RepositoryService) AddCollaborator(ctx context.Context, repo, user, permission string) (bool, bool, *scm.Response, error) {
	return false, false, nil, scm.ErrNotSupported
}

func (s *RepositoryService) ListCollaborators(ctx context.Context, repo string, ops *scm.ListOptions) ([]scm.User, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *RepositoryService) FindUserPermission(ctx context.Context, repo, user string) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *RepositoryService) Delete(ctx context.Context, repo string) (*scm.Response, error) {
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, err
	}

	requestRepo, res, err := s.find(ctx, ro)
	if err != nil {
		return res, err
	}

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s?api-version=6.0", ro.org, ro.project, requestRepo.ID)

	res, err = s.client.do(ctx, "DELETE", endpoint, nil, nil)
	return res, err
}

// Find returns the repository by name.
func (s *RepositoryService) Find(ctx context.Context, repo string) (*scm.Repository, *scm.Response, error) {
	// https://docs.microsoft.com/en-us/rest/api/azure/devops/git/repositories/get?view=azure-devops-rest-4.1
	ro, err := decodeRepo(repo)
	if err != nil {
		return nil, nil, err
	}

	return s.find(ctx, ro)
}

func (s *RepositoryService) find(ctx context.Context, ro *repoObj) (*scm.Repository, *scm.Response, error) {

	endpoint := fmt.Sprintf("%s/%s/_apis/git/repositories/%s?api-version=6.0", ro.org, ro.project, ro.name)

	out := new(repository)
	res, err := s.client.do(ctx, "GET", endpoint, nil, &out)
	return convertRepository(out), res, err
}

// FindHook returns a repository hook.
func (s *RepositoryService) FindHook(ctx context.Context, repo, id string) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// FindPerms returns the repository permissions.
func (s *RepositoryService) FindPerms(ctx context.Context, repo string) (*scm.Perm, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// List returns the profile repository list.
func (s *RepositoryService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	// Todo can we correctly implement this? Is there a reasonable list for this request not tied to a specific project?
	return nil, nil, scm.ErrNotSupported
}

// ListHooks returns a list or repository hooks.
func (s *RepositoryService) ListHooks(ctx context.Context, repo string, opts *scm.ListOptions) ([]*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// ListStatus returns a list of commit statuses.
func (s *RepositoryService) ListStatus(ctx context.Context, repo, ref string, opts *scm.ListOptions) ([]*scm.Status, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// CreateHook creates a new repository webhook.
func (s *RepositoryService) CreateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// CreateStatus creates a new commit status.
func (s *RepositoryService) CreateStatus(ctx context.Context, repo, ref string, input *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// CreateDeployStatus creates a new deployment status.
func (s *RepositoryService) CreateDeployStatus(ctx context.Context, repo string, input *scm.DeployStatus) (*scm.DeployStatus, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// UpdateHook updates a repository webhook.
func (s *RepositoryService) UpdateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

// DeleteHook deletes a repository webhook.
func (s *RepositoryService) DeleteHook(ctx context.Context, repo, id string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type project struct {
	ID string `json:"id"`
}

type repositories struct {
	Count int64         `json:"count"`
	Value []*repository `json:"value"`
}

type repository struct {
	DefaultBranch string `json:"defaultBranch"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Project       struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
		URL   string `json:"url"`
	} `json:"project"`
	RemoteURL string `json:"remoteUrl"`
	SSHURL    string `json:"sshUrl"`
	WebURL    string `json:"webUrl"`
	URL       string `json:"url"`
}

// helper function to convert from the gogs repository list to
// the common repository structure.
func convertRepositoryList(from *repositories, options *scm.ListOptions) []*scm.Repository {
	var to []*scm.Repository

	paging := options.Size > 0
	start := uint64(options.Page * options.Size)
	end := start + uint64(options.Size)

	var curr uint64
	for _, v := range from.Value {
		if !paging || (start <= curr && curr < end) {
			to = append(to, convertRepository(v))
		}
		curr++
	}
	return to
}

// helper function to convert from the gogs repository structure
// to the common repository structure.
func convertRepository(from *repository) *scm.Repository {
	projectURL, err := url.Parse(from.Project.URL)
	if err != nil {
		panic(fmt.Sprintf("could not parse url for repository's project: %s", err.Error()))
	}

	var ns string

	pathSegments := strings.Split(projectURL.Path, "/")
	if len(pathSegments) > 1 {
		ns = pathSegments[1] + "/" + from.Project.Name
	}

	return &scm.Repository{
		ID:        from.ID,
		FullName:  ns + "/" + from.Name,
		Name:      from.Name,
		Namespace: ns,
		Link:      from.WebURL,
		Clone:     from.RemoteURL,
		CloneSSH:  from.SSHURL,
		Branch:    scm.TrimRef(from.DefaultBranch),
	}
}

func (s *RepositoryService) getProject(ctx context.Context, ro *repoObj) (*project, *scm.Response, error) {
	proj := new(project)
	projectEndpoint := fmt.Sprintf("%s/_apis/projects/%s?api-version=6.0", ro.org, ro.project)

	res, err := s.client.do(ctx, "GET", projectEndpoint, nil, &proj)
	return proj, res, err
}
