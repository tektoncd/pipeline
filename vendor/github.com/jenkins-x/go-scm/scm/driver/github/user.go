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

func convertUser(from *user) *scm.User {
	return &scm.User{
		Avatar:  from.Avatar,
		Email:   from.Email.String,
		Login:   from.Login,
		Name:    from.Name,
		Link:    from.HTMLURL,
		Created: from.Created,
		Updated: from.Updated,
	}
}
