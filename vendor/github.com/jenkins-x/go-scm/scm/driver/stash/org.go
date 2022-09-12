// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"context"
	"fmt"

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
	path := fmt.Sprintf("rest/api/1.0/projects/%s/permissions/users?%s", org, encodeListOptions(opts))
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
	path := fmt.Sprintf("rest/api/1.0/projects/%s/permissions/users?%s", org, encodeListOptions(opts))
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
		if participant.User.Name == user || participant.User.Slug == user {
			return true, res, err
		}
	}
	// Retrieve the list of groups attached to the project
	groups, err := getProjectGroups(ctx, org, s, opts)
	if err != nil {
		return false, res, err
	}
	for _, pgroup := range groups {
		// Get list of users in a group
		users, err := usersInGroups(ctx, pgroup.Group.Name, s, opts)
		if err != nil {
			return false, res, err
		}
		for _, member := range users {
			if member.Name == user || member.Slug == user {
				return true, res, err
			}
		}
	}
	return false, res, err
}

// getProjectGroups returns the groups which have some permissions in the project
func getProjectGroups(ctx context.Context, org string, os *organizationService, opts *scm.ListOptions) ([]*projGroup, error) {
	path := fmt.Sprintf("rest/api/1.0/projects/%s/permissions/groups?%s", org, encodeListOptions(opts))
	out := new(projGroups)
	res, err := os.client.do(ctx, "GET", path, nil, out)
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
func usersInGroups(ctx context.Context, group string, os *organizationService, opts *scm.ListOptions) ([]*member, error) {
	path := fmt.Sprintf("rest/api/1.0/admin/groups/more-members?context=%s&%s", group, encodeListOptions(opts))
	out := new(members)
	res, err := os.client.do(ctx, "GET", path, nil, out)
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
	path := fmt.Sprintf("rest/api/1.0/projects/%s/permissions/users?%s", org, encodeListOptions(opts))
	out := new(participants)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if !out.pagination.LastPage.Bool {
		res.Page.First = 1
		res.Page.Next = opts.Page + 1
	}
	for _, participant := range out.Values {
		if (participant.User.Name == user || participant.User.Slug == user) && apiStringToPermission(participant.Permission) == scm.AdminPermission {
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
	var teamMembers []*scm.TeamMember
	for _, f := range from.Values {
		teamMembers = append(teamMembers, &scm.TeamMember{Login: f.User.Name})
	}
	return teamMembers
}

func convertProjectList(from []project) []*scm.Organization {
	var to []*scm.Organization
	for _, v := range from {
		to = append(to, convertProject(v))
	}
	return to
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
