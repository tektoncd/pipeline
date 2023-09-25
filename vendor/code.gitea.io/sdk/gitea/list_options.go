// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
)

// ListOptions options for using Gitea's API pagination
type ListOptions struct {
	// Setting Page to -1 disables pagination on endpoints that support it.
	// Page numbering starts at 1.
	Page int
	// The default value depends on the server config DEFAULT_PAGING_NUM
	// The highest valid value depends on the server config MAX_RESPONSE_ITEMS
	PageSize int
}

func (o ListOptions) getURLQuery() url.Values {
	query := make(url.Values)
	query.Add("page", fmt.Sprintf("%d", o.Page))
	query.Add("limit", fmt.Sprintf("%d", o.PageSize))

	return query
}

// setDefaults applies default pagination options.
// If .Page is set to -1, it will disable pagination.
// WARNING: This function is not idempotent, make sure to never call this method twice!
func (o *ListOptions) setDefaults() {
	if o.Page < 0 {
		o.Page, o.PageSize = 0, 0
		return
	} else if o.Page == 0 {
		o.Page = 1
	}
}
