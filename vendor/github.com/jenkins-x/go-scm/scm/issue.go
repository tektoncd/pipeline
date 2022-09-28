// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"strings"
	"time"
)

type (
	// Issue represents an issue.
	Issue struct {
		Number      int
		Title       string
		Body        string
		Link        string
		State       string
		Labels      []string
		Closed      bool
		Locked      bool
		Author      User
		Assignees   []User
		ClosedBy    *User
		PullRequest *PullRequest
		Created     time.Time
		Updated     time.Time
	}

	// SearchIssue for the results of a search which queries across repositories
	SearchIssue struct {
		Issue
		Repository Repository
	}

	// IssueInput provides the input fields required for
	// creating or updating an issue.
	IssueInput struct {
		Title string
		Body  string
	}

	// IssueListOptions provides options for querying a
	// list of repository issues.
	IssueListOptions struct {
		Page   int
		Size   int
		Open   bool
		Closed bool
	}

	// Comment represents a comment.
	Comment struct {
		ID      int
		Body    string
		Author  User
		Link    string
		Version int
		Created time.Time
		Updated time.Time
	}

	// CommentInput provides the input fields required for
	// creating an issue comment.
	CommentInput struct {
		Body string
	}

	// ListedIssueEvent for listing events on an issue
	ListedIssueEvent struct {
		Event   string
		Actor   User
		Label   Label
		Created time.Time
	}

	// SearchOptions thte query options for a search
	SearchOptions struct {
		Query     string
		Sort      string
		Ascending bool
	}

	// IssueService provides access to issue resources.
	IssueService interface {
		// Find returns the issue by number.
		Find(context.Context, string, int) (*Issue, *Response, error)

		// FindComment returns the issue comment.
		FindComment(context.Context, string, int, int) (*Comment, *Response, error)

		// List returns the repository issue list.
		List(context.Context, string, IssueListOptions) ([]*Issue, *Response, error)

		// Find returns the issue by number.
		Search(context.Context, SearchOptions) ([]*SearchIssue, *Response, error)

		// ListComments returns the issue comment list.
		ListComments(context.Context, string, int, *ListOptions) ([]*Comment, *Response, error)

		// ListLabels returns the labels on an issue
		ListLabels(context.Context, string, int, *ListOptions) ([]*Label, *Response, error)

		// ListEvents returns the events creating and removing the labels on an issue
		ListEvents(context.Context, string, int, *ListOptions) ([]*ListedIssueEvent, *Response, error)

		// Create creates a new issue.
		Create(context.Context, string, *IssueInput) (*Issue, *Response, error)

		// CreateComment creates a new issue comment.
		CreateComment(context.Context, string, int, *CommentInput) (*Comment, *Response, error)

		// DeleteComment deletes an issue comment.
		DeleteComment(context.Context, string, int, int) (*Response, error)

		// EditComment edits an existing issue comment.
		EditComment(context.Context, string, int, int, *CommentInput) (*Comment, *Response, error)

		// Close closes an issue.
		Close(context.Context, string, int) (*Response, error)

		// Reopen reopens a closed issue.
		Reopen(context.Context, string, int) (*Response, error)

		// Lock locks an issue discussion.
		Lock(context.Context, string, int) (*Response, error)

		// Unlock unlocks an issue discussion.
		Unlock(context.Context, string, int) (*Response, error)

		// AddLabel adds a label to an issue
		AddLabel(ctx context.Context, repo string, number int, label string) (*Response, error)

		// DeleteLabel deletes a label from an issue
		DeleteLabel(ctx context.Context, repo string, number int, label string) (*Response, error)

		// AssignIssue assigns one or more  users to an issue
		AssignIssue(ctx context.Context, repo string, number int, logins []string) (*Response, error)

		// UnassignIssue removes the assignment of ne or more users on an issue
		UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*Response, error)

		// SetMilestone adds a milestone to an issue
		SetMilestone(ctx context.Context, repo string, issueID int, number int) (*Response, error)

		// ClearMilestone removes the milestone from an issue
		ClearMilestone(ctx context.Context, repo string, id int) (*Response, error)
	}
)

// QueryArgument returns the query argument for the search using '+' to separate the search terms while escaping :
func (o *SearchOptions) QueryArgument() string {
	query := o.Query
	if query == "" {
		return ""
	}
	q := strings.Join(strings.Fields(query), "+")
	return strings.ReplaceAll(q, ":", "%3A")
}
