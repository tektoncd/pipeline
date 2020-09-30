/*
Copyright 2019 The Knative Authors

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

// spoof contains logic to make polling HTTP requests against an endpoint with optional host spoofing.

package spoof

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/ingress"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/zipkin"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
)

// Response is a stripped down subset of http.Response. The is primarily useful
// for ResponseCheckers to inspect the response body without consuming it.
// Notably, Body is a byte slice instead of an io.ReadCloser.
type Response struct {
	Status     string
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (r *Response) String() string {
	return fmt.Sprintf("status: %d, body: %s, headers: %v", r.StatusCode, string(r.Body), r.Header)
}

// https://medium.com/stupid-gopher-tricks/ensuring-go-interface-satisfaction-at-compile-time-1ed158e8fa17
var dialContext = (&net.Dialer{}).DialContext

// ResponseChecker is used to determine when SpoofinClient.Poll is done polling.
// This allows you to predicate wait.PollImmediate on the request's http.Response.
//
// See the apimachinery wait package:
// https://github.com/kubernetes/apimachinery/blob/cf7ae2f57dabc02a3d215f15ca61ae1446f3be8f/pkg/util/wait/wait.go#L172
type ResponseChecker func(resp *Response) (done bool, err error)

// ErrorRetryChecker is used to determine if an error should be retried or not.
// If an error should be retried, it should return true and the wrapped error to explain why to retry.
type ErrorRetryChecker func(e error) (retry bool, err error)

// SpoofingClient is a minimal HTTP client wrapper that spoofs the domain of requests
// for non-resolvable domains.
type SpoofingClient struct {
	Client          *http.Client
	RequestInterval time.Duration
	RequestTimeout  time.Duration
	Logf            logging.FormatLogger
}

// TransportOption allows callers to customize the http.Transport used by a SpoofingClient
type TransportOption func(transport *http.Transport) *http.Transport

// New returns a SpoofingClient that rewrites requests if the target domain is not `resolvable`.
// It does this by looking up the ingress at construction time, so reusing a client will not
// follow the ingress if it moves (or if there are multiple ingresses).
//
// If that's a problem, see test/request.go#WaitForEndpointState for oneshot spoofing.
func New(
	ctx context.Context,
	kubeClientset *kubernetes.Clientset,
	logf logging.FormatLogger,
	domain string,
	resolvable bool,
	endpointOverride string,
	requestInterval, requestTimeout time.Duration,
	opts ...TransportOption) (*SpoofingClient, error) {
	endpoint, mapper, err := ResolveEndpoint(ctx, kubeClientset, domain, resolvable, endpointOverride)
	if err != nil {
		return nil, fmt.Errorf("failed get the cluster endpoint: %w", err)
	}

	// Spoof the hostname at the resolver level
	logf("Spoofing %s -> %s", domain, endpoint)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, e error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			// The original hostname:port is spoofed by replacing the hostname by the value
			// returned by ResolveEndpoint.
			return dialContext(ctx, network, net.JoinHostPort(endpoint, mapper(port)))
		},
	}

	for _, opt := range opts {
		transport = opt(transport)
	}

	// Enable Zipkin tracing
	roundTripper := &ochttp.Transport{
		Base:        transport,
		Propagation: tracecontextb3.TraceContextB3Egress,
	}

	sc := &SpoofingClient{
		Client:          &http.Client{Transport: roundTripper},
		RequestInterval: requestInterval,
		RequestTimeout:  requestTimeout,
		Logf:            logf,
	}
	return sc, nil
}

// ResolveEndpoint resolves the endpoint address considering whether the domain is resolvable and taking into
// account whether the user overrode the endpoint address externally
func ResolveEndpoint(ctx context.Context, kubeClientset *kubernetes.Clientset, domain string, resolvable bool, endpointOverride string) (string, func(string) string, error) {
	id := func(in string) string { return in }
	// If the domain is resolvable, it can be used directly
	if resolvable {
		return domain, id, nil
	}
	// Otherwise, use the actual cluster endpoint
	return ingress.GetIngressEndpoint(ctx, kubeClientset, endpointOverride)
}

// Do dispatches to the underlying http.Client.Do, spoofing domains as needed
// and transforming the http.Response into a spoof.Response.
// Each response is augmented with "ZipkinTraceID" header that identifies the zipkin trace corresponding to the request.
func (sc *SpoofingClient) Do(req *http.Request, errorRetryCheckers ...ErrorRetryChecker) (*Response, error) {
	return sc.Poll(req, func(*Response) (bool, error) { return true, nil }, errorRetryCheckers...)
}

// Poll executes an http request until it satisfies the inState condition or, if there's an error,
// none of the error retry checkers permit a retry.
// If no retry checkers are specified `DefaultErrorRetryChecker` will be used.
func (sc *SpoofingClient) Poll(req *http.Request, inState ResponseChecker, errorRetryCheckers ...ErrorRetryChecker) (*Response, error) {
	if len(errorRetryCheckers) == 0 {
		errorRetryCheckers = []ErrorRetryChecker{DefaultErrorRetryChecker}
	}

	var resp *Response
	err := wait.PollImmediate(sc.RequestInterval, sc.RequestTimeout, func() (bool, error) {
		// Starting span to capture zipkin trace.
		traceContext, span := trace.StartSpan(req.Context(), "SpoofingClient-Trace")
		defer span.End()
		rawResp, err := sc.Client.Do(req.WithContext(traceContext))
		if err != nil {
			for _, checker := range errorRetryCheckers {
				retry, newErr := checker(err)
				if retry {
					sc.Logf("Retrying %s: %v", req.URL.String(), newErr)
					return false, nil
				}
			}
			sc.Logf("NOT Retrying %s: %v", req.URL.String(), err)
			return true, err
		}
		defer rawResp.Body.Close()

		body, err := ioutil.ReadAll(rawResp.Body)
		if err != nil {
			return true, err
		}
		rawResp.Header.Add(zipkin.ZipkinTraceIDHeader, span.SpanContext().TraceID.String())

		resp = &Response{
			Status:     rawResp.Status,
			StatusCode: rawResp.StatusCode,
			Header:     rawResp.Header,
			Body:       body,
		}
		return inState(resp)
	})

	if resp != nil {
		sc.logZipkinTrace(resp)
	}

	if err != nil {
		return resp, fmt.Errorf("response: %s did not pass checks: %w", resp, err)
	}
	return resp, nil
}

// DefaultErrorRetryChecker implements the defaults for retrying on error.
func DefaultErrorRetryChecker(err error) (bool, error) {
	if isTCPTimeout(err) {
		return true, fmt.Errorf("retrying for TCP timeout: %w", err)
	}
	// Retrying on DNS error, since we may be using xip.io or nip.io in tests.
	if isDNSError(err) {
		return true, fmt.Errorf("retrying for DNS error: %w", err)
	}
	// Repeat the poll on `connection refused` errors, which are usually transient Istio errors.
	if isConnectionRefused(err) {
		return true, fmt.Errorf("retrying for connection refused: %w", err)
	}
	if isConnectionReset(err) {
		return true, fmt.Errorf("retrying for connection reset: %w", err)
	}
	// Retry on connection/network errors.
	if errors.Is(err, io.EOF) {
		return true, fmt.Errorf("retrying for: %w", err)
	}
	return false, err
}

// logZipkinTrace provides support to log Zipkin Trace for param: spoofResponse
// We only log Zipkin trace for HTTP server errors i.e for HTTP status codes between 500 to 600
func (sc *SpoofingClient) logZipkinTrace(spoofResp *Response) {
	if !zipkin.ZipkinTracingEnabled || spoofResp.StatusCode < http.StatusInternalServerError || spoofResp.StatusCode >= 600 {
		return
	}

	traceID := spoofResp.Header.Get(zipkin.ZipkinTraceIDHeader)
	sc.Logf("Logging Zipkin Trace for: %s", traceID)

	json, err := zipkin.JSONTrace(traceID /* We don't know the expected number of spans */, -1, 5*time.Second)
	if err != nil {
		if _, ok := err.(*zipkin.TimeoutError); !ok {
			sc.Logf("Error getting zipkin trace: %v", err)
		}
	}

	sc.Logf("%s", json)
}
