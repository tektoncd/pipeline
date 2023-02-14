// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type organizationService struct {
	client *wrapper
}

func (s *organizationService) Create(ctx context.Context, input *scm.OrganizationInput) (*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) Delete(ctx context.Context, s2 string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) ListTeams(ctx context.Context, org string, ops *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) IsMember(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	return false, nil, scm.ErrNotSupported
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	return false, nil, scm.ErrNotSupported
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, ops *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, ops *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListPendingInvitations(ctx context.Context, org string, ops *scm.ListOptions) ([]*scm.OrganizationPendingInvite, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) AcceptOrganizationInvitation(ctx context.Context, org string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) ListMemberships(ctx context.Context, opts *scm.ListOptions) ([]*scm.Membership, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) FindMembership(ctx context.Context, name, username string) (*scm.Membership, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}
