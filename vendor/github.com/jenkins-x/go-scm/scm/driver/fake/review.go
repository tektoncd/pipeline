package fake

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type reviewService struct {
	client *wrapper
	data   *Data
}

func (s *reviewService) Find(ctx context.Context, repo string, number int, reviewID int) (*scm.Review, *scm.Response, error) {
	reviews, r, err := s.List(ctx, repo, number, scm.ListOptions{})
	if err != nil {
		return nil, r, err
	}
	for _, review := range reviews {
		if review.ID == reviewID {
			return review, nil, nil
		}
	}
	return nil, r, err
}

func (s *reviewService) List(ctx context.Context, repo string, number int, opt scm.ListOptions) ([]*scm.Review, *scm.Response, error) {
	f := s.data
	return append([]*scm.Review{}, f.Reviews[number]...), nil, nil
}

func (s *reviewService) Create(ctx context.Context, repo string, number int, input *scm.ReviewInput) (*scm.Review, *scm.Response, error) {
	f := s.data
	review := &scm.Review{
		ID:     f.ReviewID,
		Author: scm.User{Login: botName},
		Body:   input.Body,
	}
	f.Reviews[number] = append(f.Reviews[number], review)
	f.ReviewID++
	return review, nil, nil
}

func (s *reviewService) Delete(context.Context, string, int, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *reviewService) ListComments(ctx context.Context, repo string, prID int, reviewID int, options scm.ListOptions) ([]*scm.ReviewComment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Update(ctx context.Context, repo string, prID int, reviewID int, body string) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Submit(ctx context.Context, repo string, prID int, reviewID int, input *scm.ReviewSubmitInput) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *reviewService) Dismiss(ctx context.Context, repo string, prID int, reviewID int, msg string) (*scm.Review, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}
