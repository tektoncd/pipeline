// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitlab

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/internal/null"
)

type userService struct {
	client *wrapper
}

func (s *userService) Find(ctx context.Context) (*scm.User, *scm.Response, error) {
	out := new(user)
	res, err := s.client.do(ctx, "GET", "api/v4/user", nil, out)
	return convertUser(out), res, err
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/users?search=%s", login)
	out := []*user{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	if err != nil {
		return nil, nil, err
	}
	if len(out) != 1 || !strings.EqualFold(out[0].Username, login) {
		return nil, nil, scm.ErrNotFound
	}
	return convertUser(out[0]), res, err
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	user, res, err := s.Find(ctx)
	return user.Email, res, err
}

type user struct {
	Username string      `json:"username"`
	Name     string      `json:"name"`
	Email    null.String `json:"email"`
	Avatar   string      `json:"avatar_url"`
}

func convertUser(from *user) *scm.User {
	return &scm.User{
		Avatar: from.Avatar,
		Email:  from.Email.String,
		Login:  from.Username,
		Name:   from.Name,
	}
}

func convertUserList(users []*user) []scm.User {
	dst := []scm.User{}
	for _, src := range users {
		dst = append(dst, *convertUser(src))
	}
	return dst
}
