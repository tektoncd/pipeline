package github

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type milestoneService struct {
	client *wrapper
}

type milestone struct {
	ID           int       `json:"id"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	State        string    `json:"state"`
	DueOn        time.Time `json:"due_on"`
	URL          string    `json:"url"`
	HTMLURL      string    `json:"html_url"`
	LabelsURL    string    `json:"labels_url"`
	Creator      user      `json:"creator"`
	OpenIssues   int       `json:"open_issues"`
	ClosedIssues int       `json:"closed_issues"`
	NodeID       string    `json:"node_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ClosedAt     time.Time `json:"closed_at"`
}

type milestoneInput struct {
	Title       string    `json:"title"`
	State       string    `json:"state"`
	Description string    `json:"description"`
	DueOn       time.Time `json:"due_on"`
}

func (s *milestoneService) Find(ctx context.Context, repo string, id int) (*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/milestones/%d", repo, id)
	out := new(milestone)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertMilestone(out), res, err
}

func (s *milestoneService) List(ctx context.Context, repo string, opts scm.MilestoneListOptions) ([]*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/milestones?%s", repo, encodeMilestoneListOptions(opts))
	out := []*milestone{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertMilestoneList(out), res, err
}

func (s *milestoneService) Create(ctx context.Context, repo string, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/milestones", repo)
	in := &milestoneInput{
		Title:       input.Title,
		State:       input.State,
		Description: input.Description,
		DueOn:       *input.DueDate,
	}
	out := new(milestone)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertMilestone(out), res, err
}

func (s *milestoneService) Delete(ctx context.Context, repo string, id int) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/milestones/%d", repo, id)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *milestoneService) Update(ctx context.Context, repo string, id int, input *scm.MilestoneInput) (*scm.Milestone, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/milestones/%d", repo, id)
	in := &milestoneInput{}
	if input.Title != "" {
		in.Title = input.Title
	}
	if input.State != "" {
		in.State = input.State
	}
	if input.Description != "" {
		in.Description = input.Description
	}
	if input.DueDate != nil {
		in.DueOn = *input.DueDate
	}
	out := new(milestone)
	res, err := s.client.do(ctx, "PATCH", path, in, out)
	return convertMilestone(out), res, err
}

func convertMilestoneList(from []*milestone) []*scm.Milestone {
	var to []*scm.Milestone
	for _, m := range from {
		to = append(to, convertMilestone(m))
	}
	return to
}

func convertMilestone(from *milestone) *scm.Milestone {
	return &scm.Milestone{
		Number:      from.Number,
		ID:          from.ID,
		Title:       from.Title,
		Description: from.Description,
		Link:        from.HTMLURL,
		State:       from.State,
		DueDate:     &from.DueOn,
	}
}
