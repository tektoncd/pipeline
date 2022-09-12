package gitea

import (
	"context"

	"code.gitea.io/sdk/gitea"
	"github.com/jenkins-x/go-scm/scm"
)

type milestoneService struct {
	client *wrapper
}

func (s *milestoneService) Find(ctx context.Context, repo string, id int) (*scm.Milestone, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	out, resp, err := s.client.GiteaClient.GetMilestone(namespace, name, int64(id))
	return convertMilestone(out), toSCMResponse(resp), err
}

func (s *milestoneService) List(ctx context.Context, repo string, opts scm.MilestoneListOptions) ([]*scm.Milestone, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.ListMilestoneOption{
		ListOptions: gitea.ListOptions{
			Page:     opts.Page,
			PageSize: opts.Size,
		},
	}
	if opts.Closed && opts.Open {
		in.State = gitea.StateAll
	} else if opts.Closed {
		in.State = gitea.StateClosed
	} else if opts.Open {
		in.State = gitea.StateOpen
	}
	out, resp, err := s.client.GiteaClient.ListRepoMilestones(namespace, name, in)
	return convertMilestoneList(out), toSCMResponse(resp), err
}

func (s *milestoneService) Create(ctx context.Context, repo string, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.CreateMilestoneOption{
		Title:       input.Title,
		Description: input.Description,
		State:       gitea.StateOpen,
		Deadline:    input.DueDate,
	}
	if input.State == "closed" {
		in.State = gitea.StateClosed
	}
	out, resp, err := s.client.GiteaClient.CreateMilestone(namespace, name, in)
	return convertMilestone(out), toSCMResponse(resp), err
}

func (s *milestoneService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	namespace, name := scm.Split(repo)
	resp, err := s.client.GiteaClient.DeleteMilestone(namespace, name, int64(id))
	return toSCMResponse(resp), err
}

func (s *milestoneService) Update(ctx context.Context, repo string, id int, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	namespace, name := scm.Split(repo)
	in := gitea.EditMilestoneOption{}
	stateOpen := gitea.StateOpen
	stateClosed := gitea.StateClosed
	if input.Title != "" {
		in.Title = input.Title
	}
	switch input.State {
	case "open":
		in.State = &stateOpen
	case "close", "closed":
		in.State = &stateClosed
	}
	if input.Description != "" {
		in.Description = &input.Description
	}
	if input.DueDate != nil {
		in.Deadline = input.DueDate
	}
	out, resp, err := s.client.GiteaClient.EditMilestone(namespace, name, int64(id), in)
	return convertMilestone(out), toSCMResponse(resp), err
}

func convertMilestoneList(from []*gitea.Milestone) []*scm.Milestone {
	var to []*scm.Milestone
	for _, m := range from {
		to = append(to, convertMilestone(m))
	}
	return to
}

func convertMilestone(from *gitea.Milestone) *scm.Milestone {
	if from == nil || from.Deadline == nil {
		return nil
	}
	return &scm.Milestone{
		Number:      int(from.ID),
		ID:          int(from.ID),
		Title:       from.Title,
		Description: from.Description,
		State:       string(from.State),
		DueDate:     from.Deadline,
	}
}
