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

func (s *organizationService) IsMember(ctx context.Context, org string, user string) (bool, *scm.Response, error) {
	users, res, err := s.ListMemberUsers(ctx, org)
	if err != nil {
		return false, res, err
	}
	member := false
	for _, u := range users {
		if u.Login == user {
			member = true
			break
		}
	}
	return member, res, err
}

func (s *organizationService) ListTeams(ctx context.Context, org string, ops scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	// TODO implement me
	return nil, nil, nil
}

func (s *organizationService) ListTeamMembers(ctx context.Context, id int, role string, ops scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	// TODO implement me
	return nil, nil, nil
}

func (s *organizationService) ListMemberUsers(ctx context.Context, org string) ([]scm.User, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/members/all", org)
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

func (s *organizationService) List(ctx context.Context, opts scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/groups?%s", encodeListOptions(opts))
	out := []*organization{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertOrganizationList(out), res, err
}

type organization struct {
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
		Name:   from.Path,
		Avatar: from.Avatar.String,
	}
}
