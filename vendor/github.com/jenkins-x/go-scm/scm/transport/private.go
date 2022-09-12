// Copyright 2018 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import "net/http"

// PrivateToken is an http.RoundTripper that makes HTTP
// requests, wrapping a base RoundTripper and adding an
// Private-Token header with the GitLab personal token.
type PrivateToken struct {
	Base http.RoundTripper

	Token string // GitLab personal token
}

// RoundTrip adds the PrivateToken header to the request.
func (t *PrivateToken) RoundTrip(r *http.Request) (*http.Response, error) {
	// Do not overwrite the header if exists.
	if r.Header.Get("Private-Token") != "" {
		return t.base().RoundTrip(r)
	}
	r2 := cloneRequest(r)
	r2.Header.Set("Private-Token", t.Token)
	return t.base().RoundTrip(r2)
}

// base returns the base transport. If no base transport
// is configured, the default transport is returned.
func (t *PrivateToken) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
