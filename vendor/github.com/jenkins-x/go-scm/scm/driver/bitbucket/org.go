// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bitbucket

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
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
	path := fmt.Sprintf("2.0/workspaces/%s/permissions?q=user.account_id=%q", org, user)
	result := new(organizationMemberships)
	res, err := s.client.do(ctx, "GET", path, nil, result)
	if err != nil {
		return false, res, err
	}

	if len(result.Values) == 0 {
		return false, res, nil
	}

	permission := result.Values[0].Permission

	if permission == "admin" || permission == "owner" {
		return true, res, nil
	}

	return false, res, nil
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	return false, nil, scm.ErrNotSupported
}

func (s *organizationService) ListTeams(ctx context.Context, org string, ops *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, ops *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, ops *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("2.0/workspaces/%s", name)
	out := new(organization)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertOrganization(out), res, err
}

func (s *organizationService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("2.0/workspaces?%s", encodeListRoleOptions(opts))
	out := new(organizationList)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, err
	}
	err = copyPagination(out.pagination, res)
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

func convertOrganizationList(from *organizationList) []*scm.Organization {
	to := []*scm.Organization{}
	for _, v := range from.Values {
		to = append(to, convertOrganization(v))
	}
	return to
}

type organizationMemberships struct {
	pagination
	Values []*orgMemberPermission `json:"values"`
}

type organizationList struct {
	pagination
	Values []*organization `json:"values"`
}

type organization struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type orgMemberPermission struct {
	Permission string `json:"permission"`
}

func convertOrganization(from *organization) *scm.Organization {
	return &scm.Organization{
		Name:   from.Slug,
		Avatar: fmt.Sprintf("https://bitbucket.org/workspaces/%s/avatar", from.Slug),
	}
}
