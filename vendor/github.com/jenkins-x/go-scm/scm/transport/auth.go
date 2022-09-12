// Copyright 2018 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import "net/http"

// Authorization is an http.RoundTripper that makes HTTP
// requests, wrapping a base RoundTripper and adding an
// Authorization header with credentials
type Authorization struct {
	Base http.RoundTripper

	Scheme      string
	Credentials string
}

// RoundTrip adds the Authorization header to the request.
func (t *Authorization) RoundTrip(r *http.Request) (*http.Response, error) {
	// Do not overwrite the authorization header if exists.
	if r.Header.Get("Authorization") != "" {
		return t.base().RoundTrip(r)
	}
	r2 := cloneRequest(r)
	r2.Header.Set("Authorization", t.Scheme+" "+t.Credentials)
	return t.base().RoundTrip(r2)
}

// base returns the base transport. If no base transport
// is configured, the default transport is returned.
func (t *Authorization) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
