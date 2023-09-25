// Copyright 2023 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import "time"

type Secret struct {
	// the secret's name
	Name string `json:"name"`
	// Date and Time of secret creation
	Created time.Time `json:"created_at"`
}
