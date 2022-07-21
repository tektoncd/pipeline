// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type organizationService struct {
	client *wrapper
}

func (s *organizationService) Create(context.Context, *scm.OrganizationInput) (*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) IsMember(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	var resp *scm.Response
	var users []scm.User
	var err error
	firstRun := false
	opts := &scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (resp != nil && opts.Page <= resp.Page.Last) {
		users, resp, err = s.ListMemberUsers(ctx, org, opts)
		if err != nil {
			return false, resp, err
		}
		firstRun = true
		for k := range users {
			if users[k].Login == user {
				return true, resp, nil
			}
		}
		opts.Page++
	}
	return false, resp, err
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	// TODO implement me
	return false, nil, nil
}

func (s *organizationService) ListTeams(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	// TODO implement me
	return nil, nil, nil
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	// TODO implement me
	return nil, nil, nil
}

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	users, res, err := s.ListMemberUsers(ctx, org, opts)
	if err != nil {
		return nil, res, err
	}
	var members []*scm.TeamMember
	for k := range users {
		members = append(members, &scm.TeamMember{Login: users[k].Login})
	}
	return members, res, nil
}

func (s *organizationService) ListMemberUsers(ctx context.Context, org string, opts *scm.ListOptions) ([]scm.User, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/groups/%s/members/all?%s", org, encodeListOptions(opts))
	out := []*user{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertUserList(out), res, err
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/groups/%s", name)
	out := new(organization)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertOrganization(out), res, err
}

func (s *organizationService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/groups?%s", encodeListOptions(opts))
	out := []*organization{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertOrganizationList(out), res, err
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

type organization struct {
	ID     int         `json:"id"`
	Name   string      `json:"name"`
	Path   string      `json:"path"`
	Avatar null.String `json:"avatar_url"`
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
		Name:   from.Path,
		Avatar: from.Avatar.String,
	}
}
