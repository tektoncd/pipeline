// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type userService struct {
	client *wrapper
}

func (s *userService) CreateToken(context.Context, string, string) (*scm.UserToken, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) DeleteToken(context.Context, int64) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *userService) Find(ctx context.Context) (*scm.User, *scm.Response, error) {
	out := new(user)
	res, err := s.client.do(ctx, "GET", "user", nil, out)
	return convertUser(out), res, err
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	path := fmt.Sprintf("users/%s", login)
	out := new(user)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertUser(out), res, err
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	user, res, err := s.Find(ctx)
	return user.Email, res, err
}

func (s *userService) ListInvitations(ctx context.Context) ([]*scm.Invitation, *scm.Response, error) {
	path := "user/repository_invitations"
	out := []*repositoryInvitation{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, res, err
	}
	var invites []*scm.Invitation
	for _, orig := range out {
		invites = append(invites, convertRepositoryInvitation(orig))
	}
	return invites, res, nil
}

func (s *userService) AcceptInvitation(ctx context.Context, invitationID int64) (*scm.Response, error) {
	path := fmt.Sprintf("user/repository_invitations/%d", invitationID)
	return s.client.do(ctx, "PATCH", path, nil, nil)
}

type user struct {
	ID      int         `json:"id"`
	Login   string      `json:"login"`
	Name    string      `json:"name"`
	Email   null.String `json:"email"`
	Avatar  string      `json:"avatar_url"`
	HTMLURL string      `json:"html_url"`
	Created time.Time   `json:"created_at"`
	Updated time.Time   `json:"updated_at"`
}

type repositoryInvitation struct {
	ID      int64      `json:"id,omitempty"`
	Repo    repository `json:"repository,omitempty"`
	Invitee user       `json:"invitee,omitempty"`
	Inviter user       `json:"inviter,omitempty"`

	Permissions string    `json:"permissions,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	URL         string    `json:"url,omitempty"`
	HTMLURL     string    `json:"html_url,omitempty"`
}

func convertRepositoryInvitation(from *repositoryInvitation) *scm.Invitation {
	return &scm.Invitation{
		ID:          from.ID,
		Repo:        convertRepository(&from.Repo),
		Invitee:     convertUser(&from.Invitee),
		Inviter:     convertUser(&from.Inviter),
		Permissions: from.Permissions,
		Link:        from.URL,
		Created:     from.CreatedAt,
	}
}

func convertUser(from *user) *scm.User {
	if from == nil {
		return nil
	}
	return &scm.User{
		ID:      from.ID,
		Avatar:  from.Avatar,
		Email:   from.Email.String,
		Login:   from.Login,
		Name:    from.Name,
		Link:    from.HTMLURL,
		Created: from.Created,
		Updated: from.Updated,
	}
}
