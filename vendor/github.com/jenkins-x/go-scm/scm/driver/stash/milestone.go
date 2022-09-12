package stash

import (
	"context"

	"github.com/jenkins-x/go-scm/scm"
)

type milestoneService struct {
	client *wrapper
}

func (s *milestoneService) Find(ctx context.Context, repo string, id int) (*scm.Milestone, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *milestoneService) List(ctx context.Context, repo string, opts scm.MilestoneListOptions) ([]*scm.Milestone, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *milestoneService) Create(ctx context.Context, repo string, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *milestoneService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *milestoneService) Update(ctx context.Context, repo string, id int, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}
