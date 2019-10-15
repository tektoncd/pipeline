// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import "context"

type (
	// Content stores the contents of a repository file.
	Content struct {
		Path string
		Data []byte
	}

	// ContentParams provide parameters for creating and
	// updating repository content.
	ContentParams struct {
		Ref     string
		Branch  string
		Message string
		Data    []byte
	}

	// ContentService provides access to repositroy content.
	ContentService interface {
		// Find returns the repository file content by path.
		Find(ctx context.Context, repo, path, ref string) (*Content, *Response, error)

		// Create creates a new repositroy file.
		Create(ctx context.Context, repo, path string, params *ContentParams) (*Response, error)

		// Update updates a repository file.
		Update(ctx context.Context, repo, path string, params *ContentParams) (*Response, error)

		// Delete deletes a reository file.
		Delete(ctx context.Context, repo, path, ref string) (*Response, error)
	}
)
