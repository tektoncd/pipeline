// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import "context"

type (
	// Content stores the contents of a repository file.
	Content struct {
		Path   string
		Data   []byte
		Sha    string
		BlobID string
	}

	// ContentParams provide parameters for creating and
	// updating repository content.
	ContentParams struct {
		Ref       string
		Branch    string
		Message   string
		Data      []byte
		Sha       string
		BlobID    string
		Signature Signature
	}

	// FileEntry returns the details of a file
	FileEntry struct {
		Name string
		Path string
		Type string
		Size int
		Sha  string
		Link string
	}

	// ContentService provides access to repository content.
	ContentService interface {
		// Find returns the repository file content by path.
		Find(ctx context.Context, repo, path, ref string) (*Content, *Response, error)

		// List the files or directories at the given path
		List(ctx context.Context, repo, path, ref string) ([]*FileEntry, *Response, error)

		// Create creates a new repository file.
		Create(ctx context.Context, repo, path string, params *ContentParams) (*Response, error)

		// Update updates a repository file.
		Update(ctx context.Context, repo, path string, params *ContentParams) (*Response, error)

		// Delete deletes a repository file.
		Delete(ctx context.Context, repo, path string, params *ContentParams) (*Response, error)
	}
)
