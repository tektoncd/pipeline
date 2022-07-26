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
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews/%d", repo, number, id)
	out := new(review)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertReview(out), res, err
}

func (s *reviewService) List(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews?%s", repo, number, encodeListOptions(opts))
	out := []*review{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertReviewList(out), res, err
}

func (s *reviewService) Create(ctx context.Context, repo string, number int, input *scm.ReviewInput) (*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, number)
	in := &reviewInput{
		Body:     input.Body,
		CommitID: input.Sha,
		Event:    input.Event,
	}
	for _, c := range input.Comments {
		in.Comments = append(in.Comments, &reviewCommentInput{
			Body:     c.Body,
			Path:     c.Path,
			Position: c.Line,
		})
	}
	out := new(review)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertReview(out), res, err
}

func (s *reviewService) Delete(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews/%d", repo, number, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *reviewService) ListComments(ctx context.Context, repo string, prID, reviewID int, options *scm.ListOptions) ([]*scm.ReviewComment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews/%d/comments?%s", repo, prID, reviewID, encodeListOptions(options))
	out := []*reviewComment{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertReviewCommentList(out), res, err
}

func (s *reviewService) Update(ctx context.Context, repo string, prID, reviewID int, body string) (*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews/%d", repo, prID, reviewID)
	in := &reviewUpdateInput{Body: body}

	out := new(review)
	res, err := s.client.do(ctx, "PUT", path, in, out)
	return convertReview(out), res, err
}

func (s *reviewService) Submit(ctx context.Context, repo string, prID, reviewID int, input *scm.ReviewSubmitInput) (*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews/%d/events", repo, prID, reviewID)
	in := &reviewSubmitInput{
		Body:  input.Body,
		Event: input.Event,
	}

	out := new(review)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertReview(out), res, err
}

func (s *reviewService) Dismiss(ctx context.Context, repo string, prID, reviewID int, msg string) (*scm.Review, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/pulls/%d/reviews/%d/dismissals", repo, prID, reviewID)
	in := &reviewDismissInput{
		Message: msg,
	}
	if in.Message == "" {
		in.Message = "Dismissing"
	}

	out := new(review)
	res, err := s.client.do(ctx, "PUT", path, in, out)
	return convertReview(out), res, err
}

type reviewComment struct {
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
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type reviewInput struct {
	Body     string                `json:"body,omitempty"`
	CommitID string                `json:"commit_id,omitempty"`
	Event    string                `json:"event,omitempty"`
	Comments []*reviewCommentInput `json:"comments,omitempty"`
}

type reviewCommentInput struct {
	Body     string `json:"body"`
	Path     string `json:"path"`
	Position int    `json:"position"`
}

type reviewUpdateInput struct {
	Body string `json:"body"`
}

type reviewSubmitInput struct {
	Body  string `json:"body"`
	Event string `json:"event"`
}

type reviewDismissInput struct {
	Message string `json:"message"`
}

type review struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
	User struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		AvatarURL string `json:"avatar_url"`
	} `json:"user"`
	SubmittedAt time.Time `json:"submitted_at"`
	CommitID    string    `json:"commit_id"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
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
		Link:  from.HTMLURL,
		State: from.State,
		Sha:   from.CommitID,
		Author: scm.User{
			Login:  from.User.Login,
			Avatar: from.User.AvatarURL,
		},
		Created: from.SubmittedAt,
	}
}

func convertReviewCommentList(from []*reviewComment) []*scm.ReviewComment {
	to := []*scm.ReviewComment{}
	for _, v := range from {
		to = append(to, convertReviewComment(v))
	}
	return to
}

func convertReviewComment(from *reviewComment) *scm.ReviewComment {
	return &scm.ReviewComment{
		ID:   0,
		Body: from.Body,
		Path: from.Path,
		Sha:  from.CommitID,
		Line: from.Position,
		Link: from.HTMLURL,
		Author: scm.User{
			Login:  from.User.Login,
			Avatar: from.User.AvatarURL,
		},
		Created: from.CreatedAt,
		Updated: from.UpdatedAt,
	}
}
