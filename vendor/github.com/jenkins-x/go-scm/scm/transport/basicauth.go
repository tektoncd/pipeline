// Copyright 2018 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import "net/http"

// BasicAuth is an http.RoundTripper that makes HTTP
// requests, wrapping a base RoundTripper and adding a
// Basic Authorization header.
type BasicAuth struct {
	Base http.RoundTripper

	Username string
	Password string
}

// RoundTrip adds the Authorization header to the request.
func (t *BasicAuth) RoundTrip(r *http.Request) (*http.Response, error) {
	// Do not overwrite the authorization header if exists.
	if r.Header.Get("Authorization") != "" {
		return t.base().RoundTrip(r)
	}
	r2 := cloneRequest(r)
	r2.SetBasicAuth(t.Username, t.Password)
	return t.base().RoundTrip(r2)
}

// base returns the base transport. If no base transport
// is configured, the default transport is returned.
func (t *BasicAuth) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
