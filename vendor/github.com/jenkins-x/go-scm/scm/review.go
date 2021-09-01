// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"time"
)

type (
	// Review represents a review.
	Review struct {
		ID      int
		Body    string
		Sha     string
		Link    string
		State   string
		Author  User
		Created time.Time
		Updated time.Time
	}

	// ReviewComment represents a review comment.
	ReviewComment struct {
		ID      int
		Body    string
		Path    string
		Sha     string
		Line    int
		Link    string
		Author  User
		Created time.Time
		Updated time.Time
	}

	// ReviewHook represents a review web hook
	ReviewHook struct {
		Action       Action
		PullRequest  PullRequest
		Repo         Repository
		Review       Review
		Installation *InstallationRef
		// GUID is included in the header of the request received by Github.
		GUID string
	}

	// ReviewCommentInput provides the input fields required for
	// creating a review comment.
	ReviewCommentInput struct {
		Body string
		Path string
		Line int
	}

	// ReviewSubmitInput provides the input fields required for submitting a pending review.
	ReviewSubmitInput struct {
		Body  string
		Event string
	}

	// ReviewInput provides the input fields required for creating or updating a review.
	ReviewInput struct {
		Body     string
		Sha      string
		Event    string
		Comments []*ReviewCommentInput
	}

	// ReviewService provides access to review resources.
	ReviewService interface {
		// Find returns the review by id.
		Find(context.Context, string, int, int) (*Review, *Response, error)

		// List returns the review list.
		List(context.Context, string, int, ListOptions) ([]*Review, *Response, error)

		// Create creates a review.
		Create(context.Context, string, int, *ReviewInput) (*Review, *Response, error)

		// Delete deletes a review.
		Delete(context.Context, string, int, int) (*Response, error)

		// ListComments returns comments from a review
		ListComments(context.Context, string, int, int, ListOptions) ([]*ReviewComment, *Response, error)

		// Update updates the body of a review
		Update(context.Context, string, int, int, string) (*Review, *Response, error)

		// Submit submits a pending review
		Submit(context.Context, string, int, int, *ReviewSubmitInput) (*Review, *Response, error)

		// Dismiss dismisses a review
		Dismiss(context.Context, string, int, int, string) (*Review, *Response, error)
	}
)

const (
	// ReviewStateApproved is used for approved reviews
	ReviewStateApproved string = "APPROVED"
	// ReviewStateChangesRequested is used for reviews with changes requested
	ReviewStateChangesRequested string = "CHANGES_REQUESTED"
	// ReviewStateCommented is used for reviews with comments
	ReviewStateCommented string = "COMMENTED"
	// ReviewStateDismissed is used for reviews that have been dismissed
	ReviewStateDismissed string = "DISMISSED"
	// ReviewStatePending is used for reviews that are awaiting response
	ReviewStatePending string = "PENDING"
)
