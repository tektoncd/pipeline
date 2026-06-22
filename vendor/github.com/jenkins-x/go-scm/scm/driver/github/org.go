// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/jenkins-x/go-scm/scm"
)

type organizationService struct {
	client *wrapper
}
type plan struct {
	Name         string `json:"name"`
	PrivateRepos int    `json:"private_repos"`
}
type organization struct {
	ID                    int    `json:"id,omitempty"`
	Login                 string `json:"login"`
	Avatar                string `json:"avatar_url"`
	MembersCreatePublic   bool   `json:"members_can_create_public_repositories"`
	MembersCreatePrivate  bool   `json:"members_can_create_private_repositories"`
	MembersCreateInternal bool   `json:"members_can_create_internal_repositories"`
	Plan                  plan
}

type team struct {
	ID           int    `json:"id,omitempty"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description,omitempty"`
	Privacy      string `json:"privacy,omitempty"`
	Parent       *team  `json:"parent,omitempty"`         // Only present in responses
	ParentTeamID *int   `json:"parent_team_id,omitempty"` // Only valid in creates/edits
}

type pendingInvitations struct {
	ID      int     `json:"id"`
	Login   string  `json:"login"`
	Role    string  `json:"role"`
	Inviter inviter `json:"inviter"`
}

type inviter struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
}

type teamMember struct {
	Login string `json:"login"`
}

type membership struct {
	Role         string       `json:"role"`
	State        string       `json:"state"`
	User         user         `json:"user"`
	Organization organization `json:"organization"`
}

func (s *organizationService) Create(context.Context, *scm.OrganizationInput) (*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) IsMember(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	path := fmt.Sprintf("orgs/%s/members/%s", org, user)
	res, err := s.client.do(ctx, "GET", path, nil, nil)
	if err != nil && res == nil {
		return false, res, err
	}
	code := res.Status
	if code == 204 {
		return true, res, nil
	} else if code == 404 {
		return false, res, nil
	} else if code == 302 {
		return false, res, fmt.Errorf("requester is not %s org member", org)
	}
	// Should be unreachable.
	return false, res, fmt.Errorf("unexpected status: %d", code)
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	path := fmt.Sprintf("orgs/%s/memberships/%s", org, user)
	out := membership{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return false, res, err
	}
	isAdmin := out.Role == "admin"
	return isAdmin, res, nil
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("orgs/%s", name)
	out := new(organization)
	req := &scm.Request{
		Method: http.MethodGet,
		Path:   path,
		Header: map[string][]string{
			// This accept header adds member create repo permissions
			"Accept": {"application/vnd.github.surtur-preview+json"},
		},
	}
	res, err := s.client.doRequest(ctx, req, nil, out)

	return convertOrganization(out), res, err
}

func (s *organizationService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("user/orgs?%s", encodeListOptions(opts))
	out := []*organization{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertOrganizationList(out), res, err
}

func (s *organizationService) ListTeams(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	path := fmt.Sprintf("orgs/%s/teams?%s", org, encodeListOptions(opts))
	out := []*team{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertTeams(out), res, err
}

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	params := encodeListOptions(opts)

	req := &scm.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("orgs/%s/members?%s", org, params),
		Header: map[string][]string{
			// This accept header enables the nested teams preview.
			// https://developer.github.com/changes/2017-08-30-preview-nested-teams/
			"Accept": {"application/vnd.github.hellcat-preview+json"},
		},
	}
	out := []*teamMember{}
	res, err := s.client.doRequest(ctx, req, nil, &out)
	return convertTeamMembers(out), res, err
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	params := encodeListOptionsWith(opts, url.Values{
		"role": []string{role},
	})

	req := &scm.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("teams/%d/members?%s", id, params),
		Header: map[string][]string{
			// This accept header enables the nested teams preview.
			// https://developer.github.com/changes/2017-08-30-preview-nested-teams/
			"Accept": {"application/vnd.github.hellcat-preview+json"},
		},
	}
	out := []*teamMember{}
	res, err := s.client.doRequest(ctx, req, nil, &out)
	return convertTeamMembers(out), res, err
}

// ListPendingInvitations lists the pending invitations for an organisation
// see https://developer.github.com/v3/orgs/members/#list-pending-organization-invitations
func (s *organizationService) ListPendingInvitations(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.OrganizationPendingInvite, *scm.Response, error) {
	req := &scm.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("orgs/%s/invitations?%s", org, encodeListOptions(opts)),
	}
	out := []*pendingInvitations{}
	res, err := s.client.doRequest(ctx, req, nil, &out)
	return convertOrganisationPendingInvites(out), res, err
}

// ListMemberships lists organisation memberships for the authenticated user
// see https://developer.github.com/v3/orgs/members/#list-organization-memberships-for-the-authenticated-user
func (s *organizationService) ListMemberships(ctx context.Context, opts *scm.ListOptions) ([]*scm.Membership, *scm.Response, error) {
	req := &scm.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("/user/memberships/orgs?%s", encodeListOptions(opts)),
	}
	out := []*membership{}
	res, err := s.client.doRequest(ctx, req, nil, &out)
	return convertMemberships(out), res, err
}

// AcceptOrganizationInvitation accepts an invitation for an organisation
func (s *organizationService) AcceptOrganizationInvitation(ctx context.Context, org string) (*scm.Response, error) {
	req := &scm.Request{
		Method: http.MethodPatch,
		Path:   fmt.Sprintf("/user/memberships/orgs/%s", org),
	}
	values := map[string]string{"state": "active"}
	return s.client.doRequest(ctx, req, values, nil)
}

func convertOrganisationPendingInvites(from []*pendingInvitations) []*scm.OrganizationPendingInvite {
	to := []*scm.OrganizationPendingInvite{}
	for _, v := range from {
		to = append(to, &scm.OrganizationPendingInvite{
			ID:           v.ID,
			Login:        v.Login,
			InviterLogin: v.Inviter.Login,
		})
	}
	return to
}

func convertMemberships(from []*membership) []*scm.Membership {
	to := []*scm.Membership{}
	for _, v := range from {
		to = append(to, &scm.Membership{
			OrganizationName: v.Organization.Login,
			State:            v.State,
			Role:             v.Role,
		})
	}
	return to
}

func convertOrganizationList(from []*organization) []*scm.Organization {
	to := []*scm.Organization{}
	for _, v := range from {
		to = append(to, convertOrganization(v))
	}
	return to
}

func convertOrganization(from *organization) *scm.Organization {
	return &scm.Organization{
		ID:     from.ID,
		Name:   from.Login,
		Avatar: from.Avatar,
		Permissions: scm.Permissions{
			MembersCreateInternal: from.MembersCreateInternal,
			MembersCreatePublic:   from.MembersCreatePublic,
			MembersCreatePrivate:  from.MembersCreatePrivate,
		},
	}
}

func convertTeams(from []*team) []*scm.Team {
	to := []*scm.Team{}
	for _, v := range from {
		team := convertTeam(v)
		if team != nil {
			to = append(to, team)
		}
	}
	return to
}

func convertTeam(from *team) *scm.Team {
	if from == nil {
		return nil
	}
	to := &scm.Team{
		Description: from.Description,
		ID:          from.ID,
		Name:        from.Name,
		Parent:      convertTeam(from.Parent),
		Privacy:     from.Privacy,
		Slug:        from.Slug,
	}
	if from.ParentTeamID != nil {
		to.ParentTeamID = *from.ParentTeamID
	}
	return to
}

func convertTeamMembers(from []*teamMember) []*scm.TeamMember {
	to := []*scm.TeamMember{}
	for _, v := range from {
		member := convertTeamMember(v)
		if member != nil {
			to = append(to, member)
		}
	}
	return to
}

func convertTeamMember(from *teamMember) *scm.TeamMember {
	if from == nil {
		return nil
	}
	return &scm.TeamMember{
		Login: from.Login,
	}
}
