// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stash

import (
	"bytes"
	"context"
	"crypto/md5" // #nosec
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

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
	path := "plugins/servlet/applinks/whoami"
	out := new(bytes.Buffer)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	if err != nil {
		return nil, res, err
	}
	login := out.String()
	login = strings.TrimSpace(login)
	return s.FindLogin(ctx, login)
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	path := fmt.Sprintf("rest/api/1.0/users/%s", url.PathEscape(login))
	out := new(user)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertUser(out), res, err
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	user, res, err := s.Find(ctx)
	var email string
	if err == nil {
		email = user.Email
	}
	return email, res, err
}

func (s *userService) ListInvitations(context.Context) ([]*scm.Invitation, *scm.Response, error) {
	// bitbucket server does not have an invite concept, so always return successfully
	return []*scm.Invitation{}, &scm.Response{Status: 200}, nil
}

func (s *userService) AcceptInvitation(context.Context, int64) (*scm.Response, error) {
	// bitbucket server does not have an invite concept, so always return successfully
	return &scm.Response{Status: 200}, nil
}

type user struct {
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	ID           int    `json:"id"`
	DisplayName  string `json:"displayName"`
	Active       bool   `json:"active"`
	Slug         string `json:"slug"`
	Type         string `json:"type"`
	Links        struct {
		Self []struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"links"`
}

func convertUser(from *user) *scm.User {
	if from == nil {
		return nil
	}
	return &scm.User{
		Avatar: avatarLink(from.EmailAddress),
		Login:  from.Slug,
		Name:   from.DisplayName,
		Email:  from.EmailAddress,
	}
}

func avatarLink(email string) string {
	hasher := md5.New()                          // #nosec
	hasher.Write([]byte(strings.ToLower(email))) // #nosec
	emailHash := hex.EncodeToString(hasher.Sum(nil))
	avatarURL := fmt.Sprintf("https://www.gravatar.com/avatar/%s.jpg", emailHash)
	return avatarURL
}
