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
	"net/url"
	"strings"
	"time"

	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/spoof"
)

// RequestOption enables configuration of requests
// when polling for endpoint states.
type RequestOption = spoof.RequestOption

// WithHeader will add the provided headers to the request.
//
// Deprecated: Use the spoof package version
var WithHeader = spoof.WithHeader

// Retrying modifies a ResponseChecker to retry certain response codes.
//
// Deprecated: Use the spoof package version
var Retrying = spoof.Retrying

// IsOneOfStatusCodes checks that the response code is equal to the given one.
//
// Deprecated: Use the spoof package version
var IsOneOfStatusCodes = spoof.IsOneOfStatusCodes

// IsStatusOK checks that the response code is a 200.
//
// Deprecated: Use the spoof package version
var IsStatusOK = spoof.IsStatusOK

// MatchesAllBodies checks that the *first* response body matches the "expected" body, otherwise failing.
//
// Deprecated: Use the spoof package version
var MatchesAllBodies = spoof.MatchesAllBodies

// MatchesBody checks that the *first* response body matches the "expected" body, otherwise failing.
//
// Deprecated: Use the spoof package version
var MatchesBody = spoof.MatchesBody

// MatchesAllOf combines multiple ResponseCheckers to one ResponseChecker with a logical AND. The
// checkers are executed in order. The first function to trigger an error or a retry will short-circuit
// the other functions (they will not be executed).
//
// This is useful for combining a body with a status check like:
// MatchesAllOf(IsStatusOK, MatchesBody("test"))
//
// The MatchesBody check will only be executed after the IsStatusOK has passed.
//
// Deprecated: Use the spoof package version
var MatchesAllOf = spoof.MatchesAllOf

// EventuallyMatchesBody checks that the response body *eventually* matches the expected body.
// TODO(#1178): Delete me. We don't want to need this; we should be waiting for an appropriate Status instead.
func EventuallyMatchesBody(expected string) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		// Returning (false, nil) causes SpoofingClient.Poll to retry.
		return strings.Contains(string(resp.Body), expected), nil
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
	kubeClient kubernetes.Interface,
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
	kubeClient kubernetes.Interface,
	logf logging.FormatLogger,
	url *url.URL,
	inState spoof.ResponseChecker,
	desc string,
	resolvable bool,
	timeout time.Duration,
	opts ...interface{}) (*spoof.Response, error) {

	client, rOpts, err := makeSpoofClient(ctx, kubeClient, logf, url, resolvable, timeout /* true, */, opts...)
	if err != nil {
		return nil, err
	}
	return client.WaitForEndpointState(ctx, url, inState, desc, rOpts...)
}

func makeSpoofClient(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	logf logging.FormatLogger,
	url *url.URL,
	resolvable bool,
	timeout time.Duration,
	opts ...interface{}) (*spoof.SpoofingClient, []spoof.RequestOption, error) {

	var tOpts []spoof.TransportOption
	var rOpts []spoof.RequestOption

	for _, opt := range opts {
		switch o := opt.(type) {
		case spoof.RequestOption:
			rOpts = append(rOpts, o)
		case spoof.TransportOption:
			tOpts = append(tOpts, o)
		}
	}

	client, err := NewSpoofingClient(ctx, kubeClient, logf, url.Hostname(), resolvable, tOpts...)
	if err != nil {
		return nil, nil, err
	}
	client.RequestTimeout = timeout

	return client, rOpts, nil
}

func CheckEndpointState(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	logf logging.FormatLogger,
	url *url.URL,
	inState spoof.ResponseChecker,
	desc string,
	resolvable bool,
	opts ...interface{},
) (*spoof.Response, error) {
	client, rOpts, err := makeSpoofClient(ctx, kubeClient, logf, url, resolvable, Flags.SpoofRequestTimeout /* false, */, opts...)
	if err != nil {
		return nil, err
	}
	return client.CheckEndpointState(ctx, url, inState, desc, rOpts...)
}
