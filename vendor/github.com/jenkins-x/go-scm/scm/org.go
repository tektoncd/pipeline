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
		Name   string
		Avatar string
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
		Login string `json:"login"`
	}

	// OrganizationService provides access to organization resources.
	OrganizationService interface {
		// Find returns the organization by name.
		Find(context.Context, string) (*Organization, *Response, error)

		// List returns the user organization list.
		List(context.Context, ListOptions) ([]*Organization, *Response, error)

		// ListTeams returns the user organization list.
		ListTeams(ctx context.Context, org string, ops ListOptions) ([]*Team, *Response, error)

		// IsMember returns true if the user is a member of the organiation
		IsMember(ctx context.Context, org string, user string) (bool, *Response, error)

		// ListTeamMembers lists the members of a team with a given role
		ListTeamMembers(ctx context.Context, id int, role string, ops ListOptions) ([]*TeamMember, *Response, error)
	}
)
