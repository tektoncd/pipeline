// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

// RepoActionVariable represents a action variable
type RepoActionVariable struct {
	OwnerID int64  `json:"owner_id"`
	RepoID  int64  `json:"repo_id"`
	Name    string `json:"name"`
	Value   string `json:"data"`
}
