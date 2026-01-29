// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
)

// Team represents a team in an organization
type Team struct {
	ID                      int64          `json:"id"`
	Name                    string         `json:"name"`
	Description             string         `json:"description"`
	Organization            *Organization  `json:"organization"`
	Permission              AccessMode     `json:"permission"`
	CanCreateOrgRepo        bool           `json:"can_create_org_repo"`
	IncludesAllRepositories bool           `json:"includes_all_repositories"`
	Units                   []RepoUnitType `json:"units"`
}

// RepoUnitType represent all unit types of a repo gitea currently offer
type RepoUnitType string

const (
	// RepoUnitCode represent file view of a repository
	RepoUnitCode RepoUnitType = "repo.code"
	// RepoUnitIssues represent issues of a repository
	RepoUnitIssues RepoUnitType = "repo.issues"
	// RepoUnitPulls represent pulls of a repository
	RepoUnitPulls RepoUnitType = "repo.pulls"
	// RepoUnitExtIssues represent external issues of a repository
	RepoUnitExtIssues RepoUnitType = "repo.ext_issues"
	// RepoUnitWiki represent wiki of a repository
	RepoUnitWiki RepoUnitType = "repo.wiki"
	// RepoUnitExtWiki represent external wiki of a repository
	RepoUnitExtWiki RepoUnitType = "repo.ext_wiki"
	// RepoUnitReleases represent releases of a repository
	RepoUnitReleases RepoUnitType = "repo.releases"
	// RepoUnitProjects represent projects of a repository
	RepoUnitProjects RepoUnitType = "repo.projects"
	// RepoUnitPackages represents packages of a repository
	RepoUnitPackages RepoUnitType = "repo.packages"
	// RepoUnitActions represents actions of a repository
	RepoUnitActions RepoUnitType = "repo.actions"
)

// ListTeamsOptions options for listing teams
type ListTeamsOptions struct {
	ListOptions
}

// ListOrgTeams lists all teams of an organization
func (c *Client) ListOrgTeams(org string, opt ListTeamsOptions) ([]*Team, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	teams := make([]*Team, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/teams?%s", org, opt.getURLQuery().Encode()), nil, nil, &teams)
	return teams, resp, err
}

// ListMyTeams lists all the teams of the current user
func (c *Client) ListMyTeams(opt *ListTeamsOptions) ([]*Team, *Response, error) {
	opt.setDefaults()
	teams := make([]*Team, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/user/teams?%s", opt.getURLQuery().Encode()), nil, nil, &teams)
	return teams, resp, err
}

// GetTeam gets a team by ID
func (c *Client) GetTeam(id int64) (*Team, *Response, error) {
	t := new(Team)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/teams/%d", id), nil, nil, t)
	return t, resp, err
}

// SearchTeamsOptions options for searching teams.
type SearchTeamsOptions struct {
	ListOptions
	Query              string
	IncludeDescription bool
}

func (o SearchTeamsOptions) getURLQuery() url.Values {
	query := make(url.Values)
	query.Add("page", fmt.Sprintf("%d", o.Page))
	query.Add("limit", fmt.Sprintf("%d", o.PageSize))
	query.Add("q", o.Query)
	query.Add("include_desc", fmt.Sprintf("%t", o.IncludeDescription))

	return query
}

// TeamSearchResults is the JSON struct that is returned from Team search API.
type TeamSearchResults struct {
	OK    bool    `json:"ok"`
	Error string  `json:"error"`
	Data  []*Team `json:"data"`
}

// SearchOrgTeams search for teams in a org.
func (c *Client) SearchOrgTeams(org string, opt *SearchTeamsOptions) ([]*Team, *Response, error) {
	responseBody := TeamSearchResults{}
	opt.setDefaults()
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/orgs/%s/teams/search?%s", org, opt.getURLQuery().Encode()), nil, nil, &responseBody)
	if err != nil {
		return nil, resp, err
	}
	if !responseBody.OK {
		return nil, resp, fmt.Errorf("gitea error: %v", responseBody.Error)
	}
	return responseBody.Data, resp, err
}

// CreateTeamOption options for creating a team
type CreateTeamOption struct {
	Name                    string         `json:"name"`
	Description             string         `json:"description"`
	Permission              AccessMode     `json:"permission"`
	CanCreateOrgRepo        bool           `json:"can_create_org_repo"`
	IncludesAllRepositories bool           `json:"includes_all_repositories"`
	Units                   []RepoUnitType `json:"units"`
}

// Validate the CreateTeamOption struct
func (opt *CreateTeamOption) Validate() error {
	if opt.Permission == AccessModeOwner {
		opt.Permission = AccessModeAdmin
	} else if opt.Permission != AccessModeRead && opt.Permission != AccessModeWrite && opt.Permission != AccessModeAdmin {
		return fmt.Errorf("permission mode invalid")
	}
	if len(opt.Name) == 0 {
		return fmt.Errorf("name required")
	}
	if len(opt.Name) > 255 {
		return fmt.Errorf("name too long")
	}
	if len(opt.Description) > 255 {
		return fmt.Errorf("description too long")
	}
	return nil
}

// CreateTeam creates a team for an organization
func (c *Client) CreateTeam(org string, opt CreateTeamOption) (*Team, *Response, error) {
	if err := escapeValidatePathSegments(&org); err != nil {
		return nil, nil, err
	}
	if err := (&opt).Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, nil, err
	}
	t := new(Team)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/orgs/%s/teams", org), jsonHeader, bytes.NewReader(body), t)
	return t, resp, err
}

// EditTeamOption options for editing a team
type EditTeamOption struct {
	Name                    string         `json:"name"`
	Description             *string        `json:"description"`
	Permission              AccessMode     `json:"permission"`
	CanCreateOrgRepo        *bool          `json:"can_create_org_repo"`
	IncludesAllRepositories *bool          `json:"includes_all_repositories"`
	Units                   []RepoUnitType `json:"units"`
}

// Validate the EditTeamOption struct
func (opt *EditTeamOption) Validate() error {
	if opt.Permission == AccessModeOwner {
		opt.Permission = AccessModeAdmin
	} else if opt.Permission != AccessModeRead && opt.Permission != AccessModeWrite && opt.Permission != AccessModeAdmin {
		return fmt.Errorf("permission mode invalid")
	}
	if len(opt.Name) == 0 {
		return fmt.Errorf("name required")
	}
	if len(opt.Name) > 30 {
		return fmt.Errorf("name to long")
	}
	if opt.Description != nil && len(*opt.Description) > 255 {
		return fmt.Errorf("description to long")
	}
	return nil
}

// EditTeam edits a team of an organization
func (c *Client) EditTeam(id int64, opt EditTeamOption) (*Response, error) {
	if err := (&opt).Validate(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(&opt)
	if err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PATCH", fmt.Sprintf("/teams/%d", id), jsonHeader, bytes.NewReader(body))
}

// DeleteTeam deletes a team of an organization
func (c *Client) DeleteTeam(id int64) (*Response, error) {
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/teams/%d", id), nil, nil)
}

// ListTeamMembersOptions options for listing team's members
type ListTeamMembersOptions struct {
	ListOptions
}

// ListTeamMembers lists all members of a team
func (c *Client) ListTeamMembers(id int64, opt ListTeamMembersOptions) ([]*User, *Response, error) {
	opt.setDefaults()
	members := make([]*User, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/teams/%d/members?%s", id, opt.getURLQuery().Encode()), nil, nil, &members)
	return members, resp, err
}

// GetTeamMember gets a member of a team
func (c *Client) GetTeamMember(id int64, user string) (*User, *Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, nil, err
	}
	m := new(User)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/teams/%d/members/%s", id, user), nil, nil, m)
	return m, resp, err
}

// AddTeamMember adds a member to a team
func (c *Client) AddTeamMember(id int64, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/teams/%d/members/%s", id, user), nil, nil)
}

// RemoveTeamMember removes a member from a team
func (c *Client) RemoveTeamMember(id int64, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&user); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/teams/%d/members/%s", id, user), nil, nil)
}

// ListTeamRepositoriesOptions options for listing team's repositories
type ListTeamRepositoriesOptions struct {
	ListOptions
}

// ListTeamRepositories lists all repositories of a team
func (c *Client) ListTeamRepositories(id int64, opt ListTeamRepositoriesOptions) ([]*Repository, *Response, error) {
	opt.setDefaults()
	repos := make([]*Repository, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/teams/%d/repos?%s", id, opt.getURLQuery().Encode()), nil, nil, &repos)
	return repos, resp, err
}

// AddTeamRepository adds a repository to a team
func (c *Client) AddTeamRepository(id int64, org, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("PUT", fmt.Sprintf("/teams/%d/repos/%s/%s", id, org, repo), nil, nil)
}

// RemoveTeamRepository removes a repository from a team
func (c *Client) RemoveTeamRepository(id int64, org, repo string) (*Response, error) {
	if err := escapeValidatePathSegments(&org, &repo); err != nil {
		return nil, err
	}
	return c.doRequestWithStatusHandle("DELETE", fmt.Sprintf("/teams/%d/repos/%s/%s", id, org, repo), nil, nil)
}
