// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"time"
)

type (
	// Review represents a review comment.
	Review struct {
		ID      int
		Body    string
		Path    string
		Sha     string
		Line    int
		Link    string
		State   string
		Author  User
		Created time.Time
		Updated time.Time
	}

	// ReviewHook represents a review web hook
	ReviewHook struct {
		Action      Action
		PullRequest PullRequest
		Repo        Repository
		Review      Review

		// GUID is included in the header of the request received by Github.
		GUID string
	}

	// ReviewInput provides the input fields required for
	// creating a review comment.
	ReviewInput struct {
		Body string
		Sha  string
		Path string
		Line int
	}

	// ReviewService provides access to review resources.
	ReviewService interface {
		// Find returns the review comment by id.
		Find(context.Context, string, int, int) (*Review, *Response, error)

		// List returns the review comment list.
		List(context.Context, string, int, ListOptions) ([]*Review, *Response, error)

		// Create creates a review comment.
		Create(context.Context, string, int, *ReviewInput) (*Review, *Response, error)

		// Delete deletes a review comment.
		Delete(context.Context, string, int, int) (*Response, error)
	}
)

const (
	ReviewStateApproved         string = "APPROVED"
	ReviewStateChangesRequested string = "CHANGES_REQUESTED"
	ReviewStateCommented        string = "COMMENTED"
	ReviewStateDismissed        string = "DISMISSED"
	ReviewStatePending          string = "PENDING"
)
