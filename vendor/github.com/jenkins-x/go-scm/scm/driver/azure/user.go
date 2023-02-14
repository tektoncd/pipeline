// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type userService struct {
	client *wrapper
}

func (s *userService) CreateToken(ctx context.Context, s2, s3 string) (*scm.UserToken, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) DeleteToken(ctx context.Context, i int64) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *userService) ListInvitations(ctx context.Context) ([]*scm.Invitation, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) AcceptInvitation(ctx context.Context, i int64) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *userService) Find(ctx context.Context) (*scm.User, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) FindLogin(ctx context.Context, login string) (*scm.User, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *userService) FindEmail(ctx context.Context) (string, *scm.Response, error) {
	return "", nil, scm.ErrNotSupported
}

func (s *userService) ListEmail(context.Context, scm.ListOptions) ([]*scm.Email, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}
