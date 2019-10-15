// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"time"
)

type (
	// Repository represents a git repository.
	Repository struct {
		ID        string
		Namespace string
		Name      string
		FullName  string
		Perm      *Perm
		Branch    string
		Private   bool
		Clone     string
		CloneSSH  string
		Link      string
		Created   time.Time
		Updated   time.Time
	}

	// Perm represents a user's repository permissions.
	Perm struct {
		Pull  bool
		Push  bool
		Admin bool
	}

	// Hook represents a repository hook.
	Hook struct {
		ID         string
		Name       string
		Target     string
		Events     []string
		Active     bool
		SkipVerify bool
	}

	// HookInput provides the input fields required for
	// creating or updating repository webhooks.
	HookInput struct {
		Name       string
		Target     string
		Secret     string
		Events     HookEvents
		SkipVerify bool

		// NativeEvents are used to create hooks with
		// provider-specific event types that cannot be
		// abstracted or represented in HookEvents.
		NativeEvents []string
	}

	// HookEvents represents supported hook events.
	HookEvents struct {
		Branch             bool
		Issue              bool
		IssueComment       bool
		PullRequest        bool
		PullRequestComment bool
		Push               bool
		ReviewComment      bool
		Tag                bool
	}

	// CombinedStatus is the latest statuses for a ref.
	CombinedStatus struct {
		State    State
		Sha      string
		Statuses []*Status
	}

	// Status represents a commit status.
	Status struct {
		State  State
		Label  string
		Desc   string
		Target string
	}

	// StatusInput provides the input fields required for
	// creating or updating commit statuses.
	StatusInput struct {
		State  State
		Label  string
		Desc   string
		Target string
	}

	// RepositoryService provides access to repository resources.
	RepositoryService interface {
		// Find returns a repository by name.
		Find(context.Context, string) (*Repository, *Response, error)

		// FindHook returns a repository hook.
		FindHook(context.Context, string, string) (*Hook, *Response, error)

		// FindPerms returns repository permissions.
		FindPerms(context.Context, string) (*Perm, *Response, error)

		// List returns a list of repositories.
		List(context.Context, ListOptions) ([]*Repository, *Response, error)

		// ListLabels returns the labels on a repo
		ListLabels(context.Context, string, ListOptions) ([]*Label, *Response, error)

		// ListHooks returns a list or repository hooks.
		ListHooks(context.Context, string, ListOptions) ([]*Hook, *Response, error)

		// ListStatus returns a list of commit statuses.
		ListStatus(context.Context, string, string, ListOptions) ([]*Status, *Response, error)

		// FindCombinedStatus returns the combined status for a ref
		FindCombinedStatus(ctx context.Context, repo, ref string) (*CombinedStatus, *Response, error)

		// CreateHook creates a new repository webhook.
		CreateHook(context.Context, string, *HookInput) (*Hook, *Response, error)

		// CreateStatus creates a new commit status.
		CreateStatus(context.Context, string, string, *StatusInput) (*Status, *Response, error)

		// DeleteHook deletes a repository webhook.
		DeleteHook(context.Context, string, string) (*Response, error)

		// IsCollaborator returns true if the user is a collaborator on the repository
		IsCollaborator(ctx context.Context, repo, user string) (bool, *Response, error)

		// ListCollaborators lists the collaborators on a repository
		ListCollaborators(ctx context.Context, repo string) ([]User, *Response, error)

		// FindUserPermission returns the user's permission level for a repo
		FindUserPermission(ctx context.Context, repo string, user string) (string, *Response, error)
	}
)

// TODO(bradrydzewski): Add endpoint to get a repository deploy key
// TODO(bradrydzewski): Add endpoint to list repository deploy keys
// TODO(bradrydzewski): Add endpoint to create a repository deploy key
// TODO(bradrydzewski): Add endpoint to delete a repository deploy key
