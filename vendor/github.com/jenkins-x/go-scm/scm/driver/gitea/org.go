// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

type organizationService struct {
	client *wrapper
}

func (s *organizationService) Create(_ context.Context, org *scm.OrganizationInput) (*scm.Organization, *scm.Response, error) {
	visibility := gitea.VisibleTypePublic
	if org.Private {
		visibility = gitea.VisibleTypePrivate
	}
	out, resp, err := s.client.GiteaClient.CreateOrg(gitea.CreateOrgOption{
		Name:        org.Name,
		FullName:    org.Name,
		Description: org.Description,
		Website:     org.Homepage,
		Visibility:  visibility,
	})
	return convertOrg(out), toSCMResponse(resp), err
}

func (s *organizationService) Delete(_ context.Context, org string) (*scm.Response, error) {
	resp, err := s.client.GiteaClient.DeleteOrg(org)
	return toSCMResponse(resp), err
}

func (s *organizationService) IsMember(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	isMember, resp, err := s.client.GiteaClient.CheckOrgMembership(org, user)
	return isMember, toSCMResponse(resp), err
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	var members []*scm.TeamMember
	var res *scm.Response
	var membersPage []*scm.TeamMember
	var err error
	firstRun := false
	opts := &scm.ListOptions{
		Page: 1,
	}
	for !firstRun || (res != nil && opts.Page <= res.Page.Last) {
		membersPage, res, err = s.ListOrgMembers(ctx, org, opts)
		if err != nil {
			return false, res, err
		}
		firstRun = true
		members = append(members, membersPage...)
		opts.Page++
	}
	for _, m := range members {
		if m.Login == user && m.IsAdmin {
			return true, res, nil
		}
	}
	return false, res, nil
}

func (s *organizationService) ListTeams(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.ListOrgTeams(org, gitea.ListTeamsOptions{ListOptions: toGiteaListOptions(opts)})
	return convertTeamList(out), toSCMResponse(resp), err
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.ListTeamMembers(int64(id), gitea.ListTeamMembersOptions{
		ListOptions: toGiteaListOptions(opts),
	})
	return convertMemberList(out), toSCMResponse(resp), err
}

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.ListOrgMembership(org, gitea.ListOrgMembershipOption{ListOptions: toGiteaListOptions(opts)})
	return convertMemberList(out), toSCMResponse(resp), err
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.GetOrg(name)
	return convertOrg(out), toSCMResponse(resp), err
}

func (s *organizationService) List(ctx context.Context, opts *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.ListMyOrgs(gitea.ListOrgsOptions{ListOptions: toGiteaListOptions(opts)})
	return convertOrgList(out), toSCMResponse(resp), err
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

//
// native data structure conversion
//

func convertOrgList(from []*gitea.Organization) []*scm.Organization {
	to := []*scm.Organization{}
	for _, v := range from {
		to = append(to, convertOrg(v))
	}
	return to
}

func convertOrg(from *gitea.Organization) *scm.Organization {
	if from == nil || from.UserName == "" {
		return nil
	}
	return &scm.Organization{
		Name:   from.UserName,
		Avatar: from.AvatarURL,
	}
}

func convertMemberList(from []*gitea.User) []*scm.TeamMember {
	var to []*scm.TeamMember
	for _, v := range from {
		to = append(to, convertMember(v))
	}
	return to
}

func convertMember(from *gitea.User) *scm.TeamMember {
	return &scm.TeamMember{
		Login:   from.UserName,
		IsAdmin: from.IsAdmin,
	}
}

func convertTeamList(from []*gitea.Team) []*scm.Team {
	var to []*scm.Team
	for _, v := range from {
		to = append(to, convertTeam(v))
	}
	return to
}

func convertTeam(from *gitea.Team) *scm.Team {
	if from == nil {
		return nil
	}
	return &scm.Team{
		ID:          int(from.ID),
		Name:        from.Name,
		Description: from.Description,
	}
}
