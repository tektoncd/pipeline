package fake

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
)

const (
	// RoleAll lists both members and admins
	RoleAll = "all"
	// RoleAdmin specifies the user is an org admin, or lists only admins
	RoleAdmin = "admin"
	// RoleMaintainer specifies the user is a team maintainer, or lists only maintainers
	RoleMaintainer = "maintainer"
	// RoleMember specifies the user is a regular user, or only lists regular users
	RoleMember = "member"
	// StatePending specifies the user has an invitation to the org/team.
	StatePending = "pending"
	// StateActive specifies the user's membership is active.
	StateActive = "active"
)

type organizationService struct {
	client *wrapper
	data   *Data
}

func (s *organizationService) IsMember(ctx context.Context, org string, user string) (bool, *scm.Response, error) {
	panic("implement me")
}

func (s *organizationService) Find(context.Context, string) (*scm.Organization, *scm.Response, error) {
	panic("implement me")
}

func (s *organizationService) List(context.Context, scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	panic("implement me")
}

func (s *organizationService) ListTeams(ctx context.Context, org string, ops scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
	return []*scm.Team{
		{
			ID:   0,
			Name: "Admins",
		},
		{
			ID:   42,
			Name: "Leads",
		},
	}, nil, nil
}

func (s *organizationService) ListTeamMembers(ctx context.Context, teamID int, role string, ops scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	if role != RoleAll {
		return nil, nil, fmt.Errorf("unsupported role %v (only all supported)", role)
	}
	teams := map[int][]*scm.TeamMember{
		0:  {{Login: "default-sig-lead"}},
		42: {{Login: "sig-lead"}},
	}
	members, ok := teams[teamID]
	if !ok {
		return []*scm.TeamMember{}, nil, nil
	}
	return members, nil, nil
}
