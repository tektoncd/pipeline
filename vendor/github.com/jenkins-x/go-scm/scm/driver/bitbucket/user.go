// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bitbucket

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
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
	res, err := s.client.do(ctx, "GET", "2.0/user", nil, out)
	return convertUser(out), res, err
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	path := fmt.Sprintf("2.0/users/%s", login)
	out := new(user)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertUser(out), res, err
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *userService) ListInvitations(context.Context) ([]*scm.Invitation, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) AcceptInvitation(context.Context, int64) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

type user struct {
	AccountID    string `json:"account_id"`
	Login        string `json:"username"`
	Name         string `json:"nickname"`
	EmailAddress string `json:"emailAddress"`
	ID           int    `json:"id"`
	DisplayName  string `json:"display_name"`
	Active       bool   `json:"active"`
	Slug         string `json:"slug"`
	Type         string `json:"type"`
	Links        struct {
		Self   link `json:"self"`
		HTML   link `json:"html"`
		Avatar link `json:"avatar"`
	} `json:"links"`
}

func (u *user) GetLogin() string {
	answer := u.AccountID
	return answer
}

type link struct {
	Href string `json:"href"`
}

func convertUser(from *user) *scm.User {
	name := from.Name
	if name == "" {
		name = from.DisplayName
	}
	return &scm.User{
		Avatar: fmt.Sprintf("https://bitbucket.org/account/%s/avatar/32/", from.Login),
		Login:  from.Login,
		Name:   name,
	}
}
