// Copyright 2026 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import "time"

// Activity represents a user or organization activity
type Activity struct {
	ID        int64       `json:"id"`
	ActUserID int64       `json:"act_user_id"`
	ActUser   *User       `json:"act_user"`
	OpType    string      `json:"op_type"`
	Content   string      `json:"content"`
	RepoID    int64       `json:"repo_id"`
	Repo      *Repository `json:"repo"`
	CommentID int64       `json:"comment_id"`
	Comment   *Comment    `json:"comment"`
	RefName   string      `json:"ref_name"`
	IsPrivate bool        `json:"is_private"`
	UserID    int64       `json:"user_id"`
	Created   time.Time   `json:"created"`
}
