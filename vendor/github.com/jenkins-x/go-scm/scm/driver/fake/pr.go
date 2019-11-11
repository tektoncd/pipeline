package fake

import (
	"context"
	"fmt"

	"github.com/jenkins-x/go-scm/scm"
)

type pullService struct {
	client *wrapper
	data   *Data
}

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	f := s.data
	val, exists := f.PullRequests[number]
	if !exists {
		return nil, nil, fmt.Errorf("Pull request number %d does not exit", number)
	}
	if labels, _, err := s.client.Issues.ListLabels(ctx, repo, number, scm.ListOptions{}); err == nil {
		val.Labels = labels
	}
	return val, nil, nil
}

func (s *pullService) FindComment(context.Context, string, int, int) (*scm.Comment, *scm.Response, error) {
	panic("implement me")
}

func (s *pullService) List(context.Context, string, scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	panic("implement me")
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	f := s.data
	return f.PullRequestChanges[number], nil, nil
}

func (s *pullService) ListComments(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	f := s.data
	return append([]*scm.Comment{}, f.PullRequestComments[number]...), nil, nil
}

func (s *pullService) ListLabels(context.Context, string, int, scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	r := []*scm.Label{}
	for _, l := range s.data.PullRequestLabelsExisting {
		r = append(r, &scm.Label{
			Name: l,
		})
	}
	return r, nil, nil
}

func (s *pullService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	s.data.PullRequestLabelsAdded = append(s.data.PullRequestLabelsAdded, label)
	s.data.PullRequestLabelsExisting = append(s.data.PullRequestLabelsExisting, label)
	return nil, nil
}

func (s *pullService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	s.data.PullRequestLabelsRemoved = append(s.data.PullRequestLabelsRemoved, label)

	left := []string{}
	for _, l := range s.data.PullRequestLabelsExisting {
		if l != label {
			left = append(left, l)
		}
	}
	s.data.PullRequestLabelsExisting = left
	return nil, nil
}

func (s *pullService) Merge(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) Close(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) CreateComment(ctx context.Context, repo string, number int, comment *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	f := s.data
	answer := &scm.Comment{
		ID:     f.IssueCommentID,
		Body:   comment.Body,
		Author: scm.User{Login: botName},
	}
	f.PullRequestComments[number] = append(f.PullRequestComments[number], answer)
	f.IssueCommentID++
	return answer, nil, nil
}

func (s *pullService) DeleteComment(ctx context.Context, repo string, number int, id int) (*scm.Response, error) {
	f := s.data
	newComments := []*scm.Comment{}
	for _, c := range f.PullRequestComments[number] {
		if c.ID != id {
			newComments = append(newComments, c)
		}
	}
	f.PullRequestComments[number] = newComments
	return nil, nil
}
