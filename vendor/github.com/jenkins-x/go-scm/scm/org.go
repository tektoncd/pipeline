// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
)

type (
	// Organization represents an organization account.
	Organization struct {
		ID          int
		Name        string
		Avatar      string
		Permissions Permissions
	}

	// OrganizationPendingInvite represents a pending invite to an organisation
	OrganizationPendingInvite struct {
		ID           int
		Login        string
		InviterLogin string
	}

	// OrganizationInput provides the input fields required for
	// creating a new organization.
	OrganizationInput struct {
		Name        string
		Description string
		Homepage    string
		Private     bool
	}

	// Permissions represents the possible permissions a user can have on an org
	Permissions struct {
		MembersCreatePrivate  bool
		MembersCreatePublic   bool
		MembersCreateInternal bool
	}
	// Team is a organizational team
	Team struct {
		ID          int
		Name        string
		Slug        string
		Description string
		Privacy     string

		// Parent is populated in queries
		Parent *Team

		// ParentTeamID is only valid when creating / editing teams
		ParentTeamID int
	}

	// TeamMember is a member of an organizational team
	TeamMember struct {
		Login   string `json:"login"`
		IsAdmin bool   `json:"isAdmin,omitempty"`
	}

	// Membership describes the membership a user has to an organisation
	Membership struct {
		State            string
		Role             string
		OrganizationName string
	}

	// OrganizationService provides access to organization resources.
	OrganizationService interface {
		// Find returns the organization by name.
		Find(context.Context, string) (*Organization, *Response, error)

		// Create creates an organization.
		Create(context.Context, *OrganizationInput) (*Organization, *Response, error)

		// Delete deletes an organization.
		Delete(context.Context, string) (*Response, error)

		// List returns the user organization list.
		List(context.Context, *ListOptions) ([]*Organization, *Response, error)

		// ListTeams returns the user organization list.
		ListTeams(ctx context.Context, org string, ops *ListOptions) ([]*Team, *Response, error)

		// IsMember returns true if the user is a member of the organization
		IsMember(ctx context.Context, org string, user string) (bool, *Response, error)

		// IsAdmin returns true if the user is an admin of the organization
		IsAdmin(ctx context.Context, org string, user string) (bool, *Response, error)

		// ListTeamMembers lists the members of a team with a given role
		ListTeamMembers(ctx context.Context, id int, role string, ops *ListOptions) ([]*TeamMember, *Response, error)

		// ListOrgMembers lists the members of the organization
		ListOrgMembers(ctx context.Context, org string, ops *ListOptions) ([]*TeamMember, *Response, error)

		// ListPendingInvitations lists the pending invitations for an organisation
		ListPendingInvitations(ctx context.Context, org string, ops *ListOptions) ([]*OrganizationPendingInvite, *Response, error)

		// AcceptPendingInvitation accepts a pending invitation for an organisation
		AcceptOrganizationInvitation(ctx context.Context, org string) (*Response, error)

		// ListMemberships lists organisation memberships for the authenticated user
		ListMemberships(ctx context.Context, opts *ListOptions) ([]*Membership, *Response, error)
	}
)
