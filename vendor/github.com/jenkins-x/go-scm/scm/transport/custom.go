// Copyright 2018 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import "net/http"

// Custom is an http.RoundTripper that can be used to
// implement custom HTTP request authorization.
type Custom struct {
	Base http.RoundTripper

	// Before defines an func to mutate the http.Request
	// before the transaction is executed.
	Before func(*http.Request)
}

// RoundTrip authorizes and authenticates the request with
// a user-defined function.
func (t *Custom) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := cloneRequest(r)
	t.Before(r2)
	return t.base().RoundTrip(r2)
}

// base returns the base transport. If no base transport
// is configured, the default transport is returned.
func (t *Custom) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}
