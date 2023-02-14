// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package azure

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type reviewService struct {
	client *wrapper
}

func (s *reviewService) ListComments(ctx context.Context, s2 string, i, i2 int, options *scm.ListOptions) ([]*scm.ReviewComment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Update(ctx context.Context, s3 string, i, i2 int, s2 string) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Submit(ctx context.Context, s2 string, i, i2 int, input *scm.ReviewSubmitInput) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Dismiss(ctx context.Context, s3 string, i, i2 int, s2 string) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Find(ctx context.Context, repo string, number, id int) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) List(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Create(ctx context.Context, repo string, number int, input *scm.ReviewInput) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Delete(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}
