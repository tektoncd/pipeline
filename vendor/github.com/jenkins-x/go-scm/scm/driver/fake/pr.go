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

func (s *pullService) Merge(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) Close(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) CreateComment(ctx context.Context, repo string, number int, comment *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	f := s.data
	f.IssueCommentsAdded = append(f.IssueCommentsAdded, fmt.Sprintf("%s#%d:%s", repo, number, comment.Body))
	answer := &scm.Comment{
		ID:     f.IssueCommentID,
		Body:   comment.Body,
		Author: scm.User{Login: botName},
	}
	f.IssueComments[number] = append(f.IssueComments[number], answer)
	f.IssueCommentID++
	return answer, nil, nil
}

func (s *pullService) DeleteComment(context.Context, string, int, int) (*scm.Response, error) {
	panic("implement me")
}
