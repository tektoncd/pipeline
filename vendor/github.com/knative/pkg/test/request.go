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
	"strings"

	"github.com/knative/pkg/test/logging"
	"github.com/knative/pkg/test/spoof"
	"go.opencensus.io/trace"
)

// MatchesAny is a NOP matcher. This is useful for polling until a 200 is returned.
func MatchesAny(_ *spoof.Response) (bool, error) {
	return true, nil
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

// MatchesBody checks that the *first* response body matches the "expected" body, otherwise failing.
func MatchesBody(expected string) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		if !strings.Contains(string(resp.Body), expected) {
			// Returning (true, err) causes SpoofingClient.Poll to fail.
			return true, fmt.Errorf("body mismatch: got %q, want %q", string(resp.Body), expected)
		}

		return true, nil
	}
}

// EventuallyMatchesBody checks that the response body *eventually* matches the expected body.
// TODO(#1178): Delete me. We don't want to need this; we should be waiting for an appropriate Status instead.
func EventuallyMatchesBody(expected string) spoof.ResponseChecker {
	return func(resp *spoof.Response) (bool, error) {
		if !strings.Contains(string(resp.Body), expected) {
			// Returning (false, nil) causes SpoofingClient.Poll to retry.
			return false, nil
		}

		return true, nil
	}
}

// WaitForEndpointState will poll an endpoint until inState indicates the state is achieved.
// If resolvableDomain is false, it will use kubeClientset to look up the ingress and spoof
// the domain in the request headers, otherwise it will make the request directly to domain.
// desc will be used to name the metric that is emitted to track how long it took for the
// domain to get into the state checked by inState.  Commas in `desc` must be escaped.
func WaitForEndpointState(kubeClient *KubeClient, logger *logging.BaseLogger, domain string, inState spoof.ResponseChecker, desc string, resolvable bool) (*spoof.Response, error) {
	metricName := fmt.Sprintf("WaitForEndpointState/%s", desc)
	_, span := trace.StartSpan(context.Background(), metricName)
	defer span.End()

	client, err := NewSpoofingClient(kubeClient, logger, domain, resolvable)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s", domain), nil)
	if err != nil {
		return nil, err
	}

	return client.Poll(req, inState)
}
