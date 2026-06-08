// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"context"
	"fmt"
	"net/url"

	"github.com/jenkins-x/go-scm/scm"
)

type organizationService struct {
	client *wrapper
}

type project struct {
	ID          int    `json:"id,omitempty"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type projectResponse struct {
	pagination
	Values []project `json:"values"`
}

func (s *organizationService) Create(context.Context, *scm.OrganizationInput) (*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) ListTeams(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	opts.Size = 1000
	path := projectUsersPermissionsPath(org, opts)
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return convertParticipantsToTeamMembers(out), res, err
}

// IsMember checks if the user opening a pull request is part of the org
func (s *organizationService) IsMember(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	opts := &scm.ListOptions{
		Size: 1000,
	}
	// Check if user has permissions in the project
	path := projectUsersPermissionsPath(org, opts)
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return false, res, err
	}
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	for _, participant := range out.Values {
		if isRequestedUser(user, participant.User.Name, participant.User.Slug) {
			return true, res, nil
		}
	}
	// Retrieve the list of groups attached to the project
	groups, err := getProjectGroups(ctx, org, s.client, opts)
	if err != nil {
		return false, res, err
	}
	if isUserInGroups(ctx, user, groups, s.client, opts) {
		return true, res, nil
	}
	return false, res, nil
}

func isUserInGroups(ctx context.Context, requestedUser string, groups []*projGroup, client *wrapper, opts *scm.ListOptions) bool {
	for _, pgroup := range groups {
		users, err := usersInGroups(ctx, pgroup.Group.Name, client, opts)
		if err != nil {
			continue
		}
		for _, member := range users {
			if isRequestedUser(requestedUser, member.Name, member.Slug) {
				return true
			}
		}
	}

	return false
}

func isRequestedUser(requested, name, slug string) bool {
	return name == requested || slug == requested
}

// getProjectGroups returns the groups which have some permissions in the project
func getProjectGroups(ctx context.Context, org string, client *wrapper, opts *scm.ListOptions) ([]*projGroup, error) {
	path := fmt.Sprintf("rest/api/1.0/projects/%s/permissions/groups?%s", org, encodeListOptions(opts))
	out := new(projGroups)
	res, err := client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, err
	}
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return out.Values, nil
}

// usersInGroups returns the members/users in a group
func usersInGroups(ctx context.Context, group string, client *wrapper, opts *scm.ListOptions) ([]*member, error) {
	path := fmt.Sprintf("rest/api/1.0/admin/groups/more-members?context=%s&%s", url.QueryEscape(group), encodeListOptions(opts))
	out := new(members)
	res, err := client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, err
	}
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	return out.Values, nil
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	opts := &scm.ListOptions{
		Size: 1000,
	}
	path := projectUsersPermissionsPath(org, opts)
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	for _, participant := range out.Values {
		if isRequestedUser(user, participant.User.Name, participant.User.Slug) && apiStringToPermission(participant.Permission) == scm.AdminPermission {
			return true, res, err
		}
	}
	return false, res, err
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("rest/api/1.0/projects?%s", encodeListOptions(opts))
	out := new(projectResponse)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertProjectList(out.Values), res, err
}

func (s *organizationService) ListPendingInvitations(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.OrganizationPendingInvite, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) AcceptOrganizationInvitation(ctx context.Context, org string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) ListMemberships(ctx context.Context, opts *scm.ListOptions) ([]*scm.Membership, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func convertParticipantsToTeamMembers(from *participants) []*scm.TeamMember {
	teamMembers := make([]*scm.TeamMember, 0, len(from.Values))
	for _, f := range from.Values {
		teamMembers = append(teamMembers, &scm.TeamMember{Login: f.User.Name})
	}
	return teamMembers
}

func convertProjectList(from []project) []*scm.Organization {
	to := make([]*scm.Organization, 0, len(from))
	for _, v := range from {
		to = append(to, convertProject(v))
	}
	return to
}

func projectUsersPermissionsPath(org string, opts *scm.ListOptions) string {
	return fmt.Sprintf("rest/api/1.0/projects/%s/permissions/users?%s", org, encodeListOptions(opts))
}

func convertProject(from project) *scm.Organization {
	return &scm.Organization{
		ID:     from.ID,
		Name:   from.Key,
		Avatar: "",
		Permissions: scm.Permissions{
			MembersCreateInternal: true,
			MembersCreatePublic:   true,
			MembersCreatePrivate:  true,
		},
	}
}
