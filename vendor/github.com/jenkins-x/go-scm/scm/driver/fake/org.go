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

func (s *organizationService) Create(context.Context, *scm.OrganizationInput) (*scm.Organization, *scm.Response, error) {
	panic("implement me")
}

func (s *organizationService) Delete(context.Context, string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *organizationService) IsMember(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	panic("implement me")
}

func (s *organizationService) IsAdmin(ctx context.Context, org, user string) (bool, *scm.Response, error) {
	return user == "adminUser", &scm.Response{}, nil
}

func (s *organizationService) Find(ctx context.Context, name string) (*scm.Organization, *scm.Response, error) {
	for _, org := range s.data.Organizations {
		if org.Name == name {
			return org, nil, nil
		}
	}
	return nil, nil, scm.ErrNotFound
}

func (s *organizationService) List(context.Context, *scm.ListOptions) ([]*scm.Organization, *scm.Response, error) {
	orgs := s.data.Organizations
	if orgs == nil {
		// Return hardcoded organizations if none specified explicitly
		for i := 0; i < 5; i++ {
			org := scm.Organization{
				ID:     i,
				Name:   fmt.Sprintf("organisation%d", i),
				Avatar: fmt.Sprintf("https://github.com/organisation%d.png", i),
				Permissions: scm.Permissions{
					MembersCreatePrivate:  true,
					MembersCreatePublic:   true,
					MembersCreateInternal: true,
				},
			}
			orgs = append(orgs, &org)
		}
	}
	return orgs, &scm.Response{}, nil
}

func (s *organizationService) ListTeams(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.Team, *scm.Response, error) {
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

func (s *organizationService) ListTeamMembers(ctx context.Context, teamID int, role string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
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

func (s *organizationService) ListOrgMembers(ctx context.Context, org string, opts *scm.ListOptions) ([]*scm.TeamMember, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *organizationService) ListPendingInvitations(_ context.Context, org string, opts *scm.ListOptions) ([]*scm.OrganizationPendingInvite, *scm.Response, error) {
	for _, o := range s.data.Organizations {
		if o.Name == org {
			return []*scm.OrganizationPendingInvite{{
				ID:           123,
				Login:        "fred",
				InviterLogin: "charles",
			}}, nil, nil
		}
	}
	return nil, nil, scm.ErrNotFound
}

func (s *organizationService) ListMemberships(ctx context.Context, opts *scm.ListOptions) ([]*scm.Membership, *scm.Response, error) {
	return []*scm.Membership{
		{
			OrganizationName: "test-org1",
			State:            "active",
			Role:             "admin",
		},
		{
			OrganizationName: "test-org2",
			State:            "pending",
			Role:             "member",
		},
	}, nil, nil
}

func (s *organizationService) AcceptOrganizationInvitation(_ context.Context, org string) (*scm.Response, error) {
	for _, o := range s.data.Organizations {
		if o.Name == org {
			return nil, nil
		}
	}
	return nil, scm.ErrNotFound
}
