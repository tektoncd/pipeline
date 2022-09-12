// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

type reviewService struct {
	client *wrapper
}

func (s *reviewService) Find(ctx context.Context, repo string, number, id int) (*scm.Review, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	review, resp, err := s.client.GiteaClient.GetPullReview(namespace, name, int64(number), int64(id))
	return convertReview(review), toSCMResponse(resp), err
}

func (s *reviewService) List(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Review, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	reviews, resp, err := s.client.GiteaClient.ListPullReviews(namespace, name, int64(number), gitea.ListPullReviewsOptions{ListOptions: toGiteaListOptions(opts)})

	return convertReviewList(reviews), toSCMResponse(resp), err
}

func (s *reviewService) Create(ctx context.Context, repo string, number int, input *scm.ReviewInput) (*scm.Review, *scm.Response, error) {
	namespace, name := scm.Split(repo)

	in := gitea.CreatePullReviewOptions{
		State:    toGiteaState(input.Event),
		Body:     input.Body,
		CommitID: input.Sha,
		Comments: toCreatePullRequestComments(input.Comments),
	}
	review, resp, err := s.client.GiteaClient.CreatePullReview(namespace, name, int64(number), in)
	return convertReview(review), toSCMResponse(resp), err
}

func (s *reviewService) Delete(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	resp, err := s.client.GiteaClient.DeletePullReview(namespace, name, int64(number), int64(id))
	return toSCMResponse(resp), err
}

func (s *reviewService) ListComments(ctx context.Context, repo string, prID, reviewID int, options *scm.ListOptions) ([]*scm.ReviewComment, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	comments, resp, err := s.client.GiteaClient.ListPullReviewComments(namespace, name, int64(prID), int64(reviewID))
	return convertReviewCommentList(comments), toSCMResponse(resp), err
}

func (s *reviewService) Update(ctx context.Context, repo string, prID, reviewID int, body string) (*scm.Review, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.SubmitPullReviewOptions{
		Body: body,
	}
	review, resp, err := s.client.GiteaClient.SubmitPullReview(namespace, name, int64(prID), int64(reviewID), in)
	return convertReview(review), toSCMResponse(resp), err
}

func (s *reviewService) Submit(ctx context.Context, repo string, prID, reviewID int, input *scm.ReviewSubmitInput) (*scm.Review, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.SubmitPullReviewOptions{
		State: toGiteaState(input.Event),
		Body:  input.Body,
	}
	review, resp, err := s.client.GiteaClient.SubmitPullReview(namespace, name, int64(prID), int64(reviewID), in)
	return convertReview(review), toSCMResponse(resp), err
}

// TODO: Figure out whether this actually is a _thing_ exactly in Gitea. I don't think it is.
func (s *reviewService) Dismiss(ctx context.Context, repo string, prID, reviewID int, msg string) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func convertReviewList(from []*gitea.PullReview) []*scm.Review {
	to := []*scm.Review{}
	for _, v := range from {
		to = append(to, convertReview(v))
	}
	return to
}

func convertReview(src *gitea.PullReview) *scm.Review {
	if src == nil || src.Reviewer == nil {
		return nil
	}
	return &scm.Review{
		ID:      int(src.ID),
		Body:    src.Body,
		Sha:     src.CommitID,
		Link:    src.HTMLURL,
		State:   string(src.State),
		Author:  *convertUser(src.Reviewer),
		Created: src.Submitted,
	}
}

func convertReviewCommentList(src []*gitea.PullReviewComment) []*scm.ReviewComment {
	var out []*scm.ReviewComment
	for _, v := range src {
		out = append(out, convertReviewComment(v))
	}
	return out
}

func convertReviewComment(src *gitea.PullReviewComment) *scm.ReviewComment {
	return &scm.ReviewComment{
		ID:      int(src.ID),
		Body:    src.Body,
		Path:    src.Path,
		Sha:     src.CommitID,
		Line:    int(src.LineNum),
		Link:    src.HTMLURL,
		Author:  *convertUser(src.Reviewer),
		Created: src.Created,
		Updated: src.Updated,
	}
}

func toCreatePullRequestComments(src []*scm.ReviewCommentInput) []gitea.CreatePullReviewComment {
	var out []gitea.CreatePullReviewComment
	for _, c := range src {
		out = append(out, gitea.CreatePullReviewComment{
			Path:       c.Path,
			Body:       c.Body,
			NewLineNum: int64(c.Line),
		})
	}
	return out
}

func toGiteaState(src string) gitea.ReviewStateType {
	switch src {
	case "APPROVED":
		return gitea.ReviewStateApproved
	case "PENDING":
		return gitea.ReviewStatePending
	case "COMMENT":
		return gitea.ReviewStateComment
	case "REQUEST_CHANGES":
		return gitea.ReviewStateRequestChanges
	case "REQUEST_REVIEW":
		return gitea.ReviewStateRequestReview
	default:
		return gitea.ReviewStateComment
	}
}
