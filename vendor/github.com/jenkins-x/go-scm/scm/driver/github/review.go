// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type reviewService struct {
	client *wrapper
}

func (s *reviewService) Find(ctx context.Context, repo string, number, id int) (*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/comments/%d", repo, id)
	out := new(review)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertReview(out), res, err
}

func (s *reviewService) List(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/comments?%s", repo, number, encodeListOptions(opts))
	out := []*review{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertReviewList(out), res, err
}

func (s *reviewService) Create(ctx context.Context, repo string, number int, input *scm.ReviewInput) (*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/comments", repo, number)
	in := &reviewInput{
		Body:     input.Body,
		Path:     input.Path,
		Position: input.Line,
		CommitID: input.Sha,
	}
	out := new(review)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertReview(out), res, err
}

func (s *reviewService) Delete(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/comments/%d", repo, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

type review struct {
	ID       int    `json:"id"`
	CommitID string `json:"commit_id"`
	Position int    `json:"position"`
	Path     string `json:"path"`
	User     struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type reviewInput struct {
	Body     string `json:"body"`
	Path     string `json:"path"`
	CommitID string `json:"commit_id"`
	Position int    `json:"position"`
}

func convertReviewList(from []*review) []*scm.Review {
	to := []*scm.Review{}
	for _, v := range from {
		to = append(to, convertReview(v))
	}
	return to
}

func convertReview(from *review) *scm.Review {
	return &scm.Review{
		ID:    from.ID,
		Body:  from.Body,
		Path:  from.Path,
		Line:  from.Position,
		Link:  from.HTMLURL,
		State: from.State,
		Sha:   from.CommitID,
		Author: scm.User{
			Login:  from.User.Login,
			Avatar: from.User.AvatarURL,
		},
		Created: from.CreatedAt,
		Updated: from.UpdatedAt,
	}
}
