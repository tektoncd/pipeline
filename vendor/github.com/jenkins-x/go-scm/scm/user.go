// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"context"
	"time"
)

type (
	// User represents a user account.
	User struct {
		ID      int
		Login   string
		Name    string
		Email   string
		Avatar  string
		Link    string
		IsAdmin bool `json:"isAdmin,omitempty"`
		Created time.Time
		Updated time.Time
	}

	// UserToken represents a user token.
	UserToken struct {
		ID    int64
		Token string
	}

	// Invitation represents a repo invitation
	Invitation struct {
		ID          int64
		Repo        *Repository
		Invitee     *User
		Inviter     *User
		Permissions string
		Link        string
		Created     time.Time
	}

	// UserService provides access to user account resources.
	UserService interface {
		// Find returns the authenticated user.
		Find(context.Context) (*User, *Response, error)

		// CreateToken creates a user token.
		CreateToken(context.Context, string, string) (*UserToken, *Response, error)

		// DeleteToken deletes a user token.
		DeleteToken(context.Context, int64) (*Response, error)

		// FindEmail returns the authenticated user email.
		FindEmail(context.Context) (string, *Response, error)

		// FindLogin returns the user account by username.
		FindLogin(context.Context, string) (*User, *Response, error)

		// ListInvitations lists repository or organization invitations for the current user
		ListInvitations(context.Context) ([]*Invitation, *Response, error)

		// AcceptInvitation accepts an invitation for the current user
		AcceptInvitation(context.Context, int64) (*Response, error)
	}
)
