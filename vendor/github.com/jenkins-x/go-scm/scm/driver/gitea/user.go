// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

type userService struct {
	client *wrapper
}

func (s *userService) CreateToken(_ context.Context, user, name string) (*scm.UserToken, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.CreateAccessToken(gitea.CreateAccessTokenOption{
		Name: name,
	})
	if out == nil {
		return nil, toSCMResponse(resp), err
	}
	token := &scm.UserToken{
		ID:    out.ID,
		Token: out.Token,
	}
	return token, toSCMResponse(resp), err
}

func (s *userService) DeleteToken(_ context.Context, id int64) (*scm.Response, error) {
	resp, err := s.client.GiteaClient.DeleteAccessToken(id)
	return toSCMResponse(resp), err
}

func (s *userService) Find(ctx context.Context) (*scm.User, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.GetMyUserInfo()
	return convertUser(out), toSCMResponse(resp), err
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	out, resp, err := s.client.GiteaClient.GetUserInfo(login)
	return convertUser(out), toSCMResponse(resp), err
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	user, res, err := s.Find(ctx)
	if user != nil {
		return user.Email, res, err
	}
	return "", res, err
}

func (s *userService) ListInvitations(context.Context) ([]*scm.Invitation, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) AcceptInvitation(context.Context, int64) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

//
// native data structure conversion
//

func convertUsers(src []*gitea.User) []scm.User {
	answer := []scm.User{}
	for _, u := range src {
		user := convertUser(u)
		if user.Login != "" {
			answer = append(answer, *user)
		}
	}
	if len(answer) == 0 {
		return nil
	}
	return answer
}

func convertUser(src *gitea.User) *scm.User {
	if src == nil || src.UserName == "" {
		return nil
	}
	return &scm.User{
		ID:     int(src.ID),
		Login:  src.UserName,
		Name:   src.FullName,
		Email:  src.Email,
		Avatar: src.AvatarURL,
	}
}
