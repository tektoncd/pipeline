package fake

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type repositoryService struct {
	client *wrapper
	data   *Data
}

func (s *repositoryService) FindCombinedStatus(ctx context.Context, repo, ref string) (*scm.CombinedStatus, *scm.Response, error) {
	statuses, _, err := s.ListStatus(ctx, repo, ref, &scm.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	return &scm.CombinedStatus{
		Sha:      ref,
		Statuses: statuses,
	}, nil, nil
}

func (s *repositoryService) FindUserPermission(ctx context.Context, repo, user string) (string, *scm.Response, error) {
	f := s.data
	m := f.UserPermissions[repo]
	perm := ""
	if m != nil {
		perm = m[user]
	}
	return perm, nil, nil
}

// NormLogin normalizes login strings
var NormLogin = strings.ToLower

func (s *repositoryService) FindHook(context.Context, string, string) (*scm.Hook, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) FindPerms(context.Context, string) (*scm.Perm, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) ListOrganisation(context.Context, string, *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) ListUser(context.Context, string, *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	panic("implement me")
}

func (s *repositoryService) AddCollaborator(ctx context.Context, repo, user, permission string) (bool, bool, *scm.Response, error) {
	s.data.Collaborators = append(s.data.Collaborators, user)
	if len(s.data.UserPermissions) == 0 {
		s.data.UserPermissions = make(map[string]map[string]string)
	}
	m := s.data.UserPermissions[repo]
	if len(m) == 0 {
		m = make(map[string]string)
	}

	alreadyExists := false
	if _, ok := m[user]; ok {
		alreadyExists = true
	}
	m[user] = permission

	s.data.UserPermissions[repo] = m

	// lets add an invitation if the user isn't already in the repo...
	if !alreadyExists {
		id := int64(len(s.data.Invitations) + 1)
		names := strings.SplitN(repo, "/", 2)
		owner := ""
		repoName := ""
		if len(names) == 2 {
			owner = names[0]
			repoName = names[1]
		}
		s.data.Invitations = append(s.data.Invitations, &scm.Invitation{
			ID: id,
			Repo: &scm.Repository{
				Namespace: owner,
				Name:      repoName,
				FullName:  repo,
			},
			Invitee:     &scm.User{},
			Inviter:     &scm.User{},
			Permissions: permission,
			Created:     time.Now(),
		})
	}

	return true, alreadyExists, nil, nil
}

func (s *repositoryService) IsCollaborator(ctx context.Context, repo, login string) (bool, *scm.Response, error) {
	f := s.data
	normed := NormLogin(login)
	for _, collab := range f.Collaborators {
		if NormLogin(collab) == normed {
			return true, nil, nil
		}
	}
	return false, nil, nil
}

func (s *repositoryService) ListCollaborators(ctx context.Context, repo string, ops *scm.ListOptions) ([]scm.User, *scm.Response, error) {
	f := s.data
	result := make([]scm.User, 0, len(f.Collaborators))
	for _, login := range f.Collaborators {
		result = append(result, scm.User{Login: login})
	}
	return result, nil, nil
}

func (s *repositoryService) Find(ctx context.Context, fullName string) (*scm.Repository, *scm.Response, error) {
	for _, repo := range s.data.Repositories {
		if repo.FullName == fullName {
			return repo, &scm.Response{Status: 200}, nil
		}
	}
	return nil, &scm.Response{Status: 404}, scm.ErrNotFound
}

func (s *repositoryService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Repository, *scm.Response, error) {
	return s.data.Repositories, nil, nil
}

func (s *repositoryService) ListLabels(context.Context, string, *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	f := s.data
	la := []*scm.Label{}
	for _, l := range f.RepoLabelsExisting {
		la = append(la, &scm.Label{Name: l})
	}
	return la, nil, nil
}

func (s *repositoryService) ListStatus(ctx context.Context, repo, ref string, opt *scm.ListOptions) ([]*scm.Status, *scm.Response, error) {
	f := s.data
	result := make([]*scm.Status, 0, len(f.Statuses))
	result = append(result, f.Statuses[ref]...)
	return result, nil, nil
}

func (s *repositoryService) Create(ctx context.Context, input *scm.RepositoryInput) (*scm.Repository, *scm.Response, error) {
	s.data.CreateRepositories = append(s.data.CreateRepositories, input)
	namespace := input.Namespace
	if namespace == "" {
		namespace = s.client.Username
	}
	fullName := scm.Join(namespace, input.Name)
	link := fmt.Sprintf("https://fake.com/%s.git", fullName)
	repo := &scm.Repository{
		Namespace: namespace,
		Name:      input.Name,
		FullName:  fullName,
		Link:      link,
		Clone:     link,
		Created:   time.Now(),
	}
	s.data.Repositories = append(s.data.Repositories, repo)
	return repo, nil, nil
}

func (s *repositoryService) Fork(ctx context.Context, input *scm.RepositoryInput, origRepo string) (*scm.Repository, *scm.Response, error) {
	// TODO: Actually make this fork rather than just duplicate Create.
	return s.Create(ctx, input)
}

func (s *repositoryService) ListHooks(ctx context.Context, fullName string, opts *scm.ListOptions) ([]*scm.Hook, *scm.Response, error) {
	return s.data.Hooks[fullName], nil, nil
}

func (s *repositoryService) CreateHook(ctx context.Context, fullName string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	/* #nosec */
	hook := &scm.Hook{
		ID:     fmt.Sprintf("%d", rand.Int()),
		Name:   input.Name,
		Target: input.Target,
		Active: true,
	}
	s.data.Hooks[fullName] = append(s.data.Hooks[fullName], hook)
	return hook, nil, nil
}

func (s *repositoryService) UpdateHook(ctx context.Context, repo string, input *scm.HookInput) (*scm.Hook, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *repositoryService) DeleteHook(ctx context.Context, fullName, hookID string) (*scm.Response, error) {
	hooks := s.data.Hooks[fullName]
	for i, h := range hooks {
		if h.ID == hookID {
			hooks = append(hooks[0:i], hooks[i+1:]...)
			s.data.Hooks[fullName] = hooks
			break
		}
	}
	return nil, nil
}

func (s *repositoryService) CreateStatus(ctx context.Context, repo, ref string, in *scm.StatusInput) (*scm.Status, *scm.Response, error) {
	statuses := s.data.Statuses[ref]
	if statuses == nil {
		statuses = []*scm.Status{}
	}
	status := scm.ConvertStatusInputToStatus(in)
	for _, existing := range statuses {
		if existing.Label == status.Label {
			*existing = *status
			return status, nil, nil
		}
	}
	statuses = append(statuses, status)
	s.data.Statuses[ref] = statuses
	return status, nil, nil
}

func (s *repositoryService) Delete(context.Context, string) (*scm.Response, error) {
	panic("implement me")
}
