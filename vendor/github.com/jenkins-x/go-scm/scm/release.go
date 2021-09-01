package scm

import (
	"context"
	"time"
)

type (
	// Release the release
	Release struct {
		ID          int
		Title       string
		Description string
		Link        string
		Tag         string
		Commitish   string
		Draft       bool
		Prerelease  bool
		Created     time.Time
		Published   time.Time
	}

	// ReleaseInput contains the information needed to create a release
	ReleaseInput struct {
		Title       string
		Description string
		Tag         string
		Commitish   string
		Draft       bool
		Prerelease  bool
	}

	// ReleaseListOptions provides options for querying a list of repository releases.
	ReleaseListOptions struct {
		Page   int
		Size   int
		Open   bool
		Closed bool
	}

	// ReleaseService provides access to creating, listing, updating, and deleting releases
	ReleaseService interface {
		// Find returns the release for the given number in the given repository
		Find(context.Context, string, int) (*Release, *Response, error)

		// FindByTag returns the release for the given tag in the given repository
		FindByTag(context.Context, string, string) (*Release, *Response, error)

		// List returns a list of releases in the given repository
		List(context.Context, string, ReleaseListOptions) ([]*Release, *Response, error)

		// Create creates a release in the given repository
		Create(context.Context, string, *ReleaseInput) (*Release, *Response, error)

		// Update updates a release in the given repository
		Update(context.Context, string, int, *ReleaseInput) (*Release, *Response, error)

		// Update updates a release in the given repository by tag
		UpdateByTag(context.Context, string, string, *ReleaseInput) (*Release, *Response, error)

		// Delete deletes a release in the given repository
		Delete(context.Context, string, int) (*Response, error)

		// Delete deletes a release in the given repository by tag
		DeleteByTag(context.Context, string, string) (*Response, error)
	}
)
