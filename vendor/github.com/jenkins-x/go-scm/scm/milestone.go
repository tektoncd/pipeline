package scm

import (
	"context"
	"time"
)

type (
	// MilestoneInput contains the information needed to create a milestone
	MilestoneInput struct {
		Title       string
		Description string
		State       string
		DueDate     *time.Time
	}

	// MilestoneListOptions provides options for querying a list of repository milestones.
	MilestoneListOptions struct {
		Page   int
		Size   int
		Open   bool
		Closed bool
	}

	// MilestoneService provides access to creating, listing, updating, and deleting milestones
	MilestoneService interface {
		// Find returns the milestone for the given number in the given repository
		Find(context.Context, string, int) (*Milestone, *Response, error)

		// List returns a list of milestones in the given repository
		List(context.Context, string, MilestoneListOptions) ([]*Milestone, *Response, error)

		// Create creates a milestone in the given repository
		Create(context.Context, string, *MilestoneInput) (*Milestone, *Response, error)

		// Update updates a milestone in the given repository
		Update(context.Context, string, int, *MilestoneInput) (*Milestone, *Response, error)

		// Delete deletes a milestone in the given repository
		Delete(context.Context, string, int) (*Response, error)
	}
)
