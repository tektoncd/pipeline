/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// request contains logic to make polling HTTP requests against an endpoint with optional host spoofing.

package test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/spoof"
)

// RequestOption enables configuration of requests
// when polling for endpoint states.
type RequestOption func(*http.Request)

// WithHeader will add the provided headers to the request.
func WithHeader(header http.Header) RequestOption {
	return func(r *http.Request) {
		if r.Header == nil {
			r.Header = header
			return
		}
		for key, values := range header {
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}
	}
}

// Retrying modifies a ResponseChecker to retry certain response codes.
func Retrying(rc spoof.ResponseChecker, codes ...int) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		for _, code := range codes {
			if resp.StatusCode == code {
				// Returning (false, nil) causes SpoofingClient.Poll to retry.
				// sc.logger.Infof("Retrying for code %v", resp.StatusCode)
				return false, nil
			}
		}

		// If we didn't match any retryable codes, invoke the ResponseChecker that we wrapped.
		return rc(resp)
	}
}

// IsOneOfStatusCodes checks that the response code is equal to the given one.
func IsOneOfStatusCodes(codes ...int) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		for _, code := range codes {
			if resp.StatusCode == code {
				return true, nil
			}
		}

		return true, fmt.Errorf("status = %d %s, want one of: %v", resp.StatusCode, resp.Status, codes)
	}
}

// IsStatusOK checks that the response code is a 200.
func IsStatusOK(resp *spoof.Response) (bool, error) {
	return IsOneOfStatusCodes(http.StatusOK)(resp)
}

// MatchesAllBodies checks that the *first* response body matches the "expected" body, otherwise failing.
func MatchesAllBodies(all ...string) spoof.ResponseChecker {
	var m sync.Mutex
	// This helps with two things:
	// 1. we can use Equal on sets
	// 2. it will collapse the duplicates
	want := sets.NewString(all...)
	seen := make(sets.String, len(all))

	return func(resp *spoof.Response) (bool, error) {
		bs := string(resp.Body)
		for expected := range want {
			if !strings.Contains(bs, expected) {
				// See if the next one matches.
				continue
			}

			m.Lock()
			defer m.Unlock()
			seen.Insert(expected)

			// Stop once we've seen them all.
			return want.Equal(seen), nil
		}

		// Returning (true, err) causes SpoofingClient.Poll to fail.
		return true, fmt.Errorf("body = %s, want one of: %s", bs, all)
	}
}

// MatchesBody checks that the *first* response body matches the "expected" body, otherwise failing.
func MatchesBody(expected string) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		if !strings.Contains(string(resp.Body), expected) {
			// Returning (true, err) causes SpoofingClient.Poll to fail.
			return true, fmt.Errorf("body = %s, want: %s", string(resp.Body), expected)
		}

		return true, nil
	}
}

// EventuallyMatchesBody checks that the response body *eventually* matches the expected body.
// TODO(#1178): Delete me. We don't want to need this; we should be waiting for an appropriate Status instead.
func EventuallyMatchesBody(expected string) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		// Returning (false, nil) causes SpoofingClient.Poll to retry.
		return strings.Contains(string(resp.Body), expected), nil
	}
}

// MatchesAllOf combines multiple ResponseCheckers to one ResponseChecker with a logical AND. The
// checkers are executed in order. The first function to trigger an error or a retry will short-circuit
// the other functions (they will not be executed).
//
// This is useful for combining a body with a status check like:
// MatchesAllOf(IsStatusOK, MatchesBody("test"))
//
// The MatchesBody check will only be executed after the IsStatusOK has passed.
func MatchesAllOf(checkers ...spoof.ResponseChecker) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		for _, checker := range checkers {
			if done, err := checker(resp); err != nil || !done {
				return done, err
			}
		}
		return true, nil
	}
}

// WaitForEndpointState will poll an endpoint until inState indicates the state is achieved,
// or default timeout is reached.
// If resolvableDomain is false, it will use kubeClientset to look up the ingress and spoof
// the domain in the request headers, otherwise it will make the request directly to domain.
// desc will be used to name the metric that is emitted to track how long it took for the
// domain to get into the state checked by inState.  Commas in `desc` must be escaped.
func WaitForEndpointState(
	ctx context.Context,
	kubeClient *KubeClient,
	logf logging.FormatLogger,
	url *url.URL,
	inState spoof.ResponseChecker,
	desc string,
	resolvable bool,
	opts ...interface{}) (*spoof.Response, error) {
	return WaitForEndpointStateWithTimeout(ctx, kubeClient, logf, url, inState,
		desc, resolvable, Flags.SpoofRequestTimeout, opts...)
}

// WaitForEndpointStateWithTimeout will poll an endpoint until inState indicates the state is achieved
// or the provided timeout is achieved.
// If resolvableDomain is false, it will use kubeClientset to look up the ingress and spoof
// the domain in the request headers, otherwise it will make the request directly to domain.
// desc will be used to name the metric that is emitted to track how long it took for the
// domain to get into the state checked by inState.  Commas in `desc` must be escaped.
func WaitForEndpointStateWithTimeout(
	ctx context.Context,
	kubeClient *KubeClient,
	logf logging.FormatLogger,
	url *url.URL,
	inState spoof.ResponseChecker,
	desc string,
	resolvable bool,
	timeout time.Duration,
	opts ...interface{}) (*spoof.Response, error) {
	defer logging.GetEmitableSpan(ctx, "WaitForEndpointState/"+desc).End()

	if url.Scheme == "" || url.Host == "" {
		return nil, fmt.Errorf("invalid URL: %q", url.String())
	}

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, err
	}

	var tOpts []spoof.TransportOption
	for _, opt := range opts {
		rOpt, ok := opt.(RequestOption)
		if ok {
			rOpt(req)
		} else if tOpt, ok := opt.(spoof.TransportOption); ok {
			tOpts = append(tOpts, tOpt)
		}
	}

	client, err := NewSpoofingClient(ctx, kubeClient, logf, url.Hostname(), resolvable, tOpts...)
	if err != nil {
		return nil, err
	}
	client.RequestTimeout = timeout

	return client.Poll(req, inState)
}
