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
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	ingress "github.com/knative/pkg/test/ingress"
	"github.com/knative/pkg/test/logging"
	"github.com/knative/pkg/test/zipkin"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/trace"
)

const (
	requestInterval = 1 * time.Second
	// RequestTimeout is the default timeout for the polling requests.
	RequestTimeout = 5 * time.Minute
	// Name of the temporary HTTP header that is added to http.Request to indicate that
	// it is a SpoofClient.Poll request. This header is removed before making call to backend.
	pollReqHeader = "X-Kn-Poll-Request-Do-Not-Trace"
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

// Interface defines the actions that can be performed by the spoofing client.
type Interface interface {
	Do(*http.Request) (*Response, error)
	Poll(*http.Request, ResponseChecker) (*Response, error)
}

// https://medium.com/stupid-gopher-tricks/ensuring-go-interface-satisfaction-at-compile-time-1ed158e8fa17
var _ Interface = (*SpoofingClient)(nil)

// ResponseChecker is used to determine when SpoofinClient.Poll is done polling.
// This allows you to predicate wait.PollImmediate on the request's http.Response.
//
// See the apimachinery wait package:
// https://github.com/kubernetes/apimachinery/blob/cf7ae2f57dabc02a3d215f15ca61ae1446f3be8f/pkg/util/wait/wait.go#L172
type ResponseChecker func(resp *Response) (done bool, err error)

// SpoofingClient is a minimal HTTP client wrapper that spoofs the domain of requests
// for non-resolvable domains.
type SpoofingClient struct {
	Client          *http.Client
	RequestInterval time.Duration
	RequestTimeout  time.Duration

	endpoint string
	domain   string

	logf logging.FormatLogger
}

// New returns a SpoofingClient that rewrites requests if the target domain is not `resolveable`.
// It does this by looking up the ingress at construction time, so reusing a client will not
// follow the ingress if it moves (or if there are multiple ingresses).
//
// If that's a problem, see test/request.go#WaitForEndpointState for oneshot spoofing.
func New(kubeClientset *kubernetes.Clientset, logf logging.FormatLogger, domain string, resolvable bool, endpointOverride string) (*SpoofingClient, error) {
	sc := SpoofingClient{
		Client:          &http.Client{Transport: &ochttp.Transport{Propagation: &b3.HTTPFormat{}}}, // Using ochttp Transport required for zipkin-tracing
		RequestInterval: requestInterval,
		RequestTimeout:  RequestTimeout,
		logf:            logf,
	}

	if !resolvable {
		e := &endpointOverride
		if endpointOverride == "" {
			var err error
			// If the domain that the Route controller is configured to assign to Route.Status.Domain
			// (the domainSuffix) is not resolvable, we need to retrieve the endpoint and spoof
			// the Host in our requests.
			e, err = ingress.GetIngressEndpoint(kubeClientset)
			if err != nil {
				return nil, err
			}
		}

		sc.endpoint = *e
		sc.domain = domain
	} else {
		// If the domain is resolvable, we can use it directly when we make requests.
		sc.endpoint = domain
	}

	return &sc, nil
}

// Do dispatches to the underlying http.Client.Do, spoofing domains as needed
// and transforming the http.Response into a spoof.Response.
// Each response is augmented with "ZipkinTraceID" header that identifies the zipkin trace corresponding to the request.
func (sc *SpoofingClient) Do(req *http.Request) (*Response, error) {
	// Controls the Host header, for spoofing.
	if sc.domain != "" {
		req.Host = sc.domain
	}

	// Controls the actual resolution.
	if sc.endpoint != "" {
		req.URL.Host = sc.endpoint
	}

	// Starting span to capture zipkin trace.
	traceContext, span := trace.StartSpan(req.Context(), "SpoofingClient-Trace")
	defer span.End()

	// Check to see if the call to this method is coming from a Poll call.
	logZipkinTrace := true
	if req.Header.Get(pollReqHeader) != "" {
		req.Header.Del(pollReqHeader)
		logZipkinTrace = false
	}
	resp, err := sc.Client.Do(req.WithContext(traceContext))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	resp.Header.Add(zipkin.ZipkinTraceIDHeader, span.SpanContext().TraceID.String())
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	spoofResp := &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
	}

	if logZipkinTrace {
		sc.logZipkinTrace(spoofResp)
	}

	return spoofResp, nil
}

// Poll executes an http request until it satisfies the inState condition or encounters an error.
func (sc *SpoofingClient) Poll(req *http.Request, inState ResponseChecker) (*Response, error) {
	var (
		resp *Response
		err  error
	)

	err = wait.PollImmediate(sc.RequestInterval, sc.RequestTimeout, func() (bool, error) {
		// As we may do multiple Do calls as part of a single Poll we add this temporary header
		// to the request to indicate to Do method not to log Zipkin trace, instead it is
		// handled by this method itself.
		req.Header.Add(pollReqHeader, "True")
		resp, err = sc.Do(req)
		if err != nil {
			if isTCPTimeout(err) {
				sc.logf("Retrying %s for TCP timeout %v", req.URL.String(), err)
				return false, nil
			}
			// Retrying on DNS error, since we may be using xip.io or nip.io in tests.
			if isDNSError(err) {
				sc.logf("Retrying %s for DNS error %v", req.URL.String(), err)
				return false, nil
			}
			// Repeat the poll on `connection refused` errors, which are usually transient Istio errors.
			if isTCPConnectRefuse(err) {
				sc.logf("Retrying %s for connection refused %v", req.URL.String(), err)
				return false, nil
			}
			return true, err
		}

		return inState(resp)
	})

	if resp != nil {
		sc.logZipkinTrace(resp)
	}

	if err != nil {
		return resp, errors.Wrapf(err, "response: %s did not pass checks", resp)
	}
	return resp, nil
}

// logZipkinTrace provides support to log Zipkin Trace for param: spoofResponse
// We only log Zipkin trace for HTTP server errors i.e for HTTP status codes between 500 to 600
func (sc *SpoofingClient) logZipkinTrace(spoofResp *Response) {
	if !zipkin.ZipkinTracingEnabled || spoofResp.StatusCode < http.StatusInternalServerError || spoofResp.StatusCode >= 600 {
		return
	}

	traceID := spoofResp.Header.Get(zipkin.ZipkinTraceIDHeader)
	sc.logf("Logging Zipkin Trace for: %s", traceID)

	// Sleep to ensure all traces are correctly pushed on the backend.
	time.Sleep(5 * time.Second)

	json, err := zipkin.JSONTrace(traceID)
	if err != nil {
		sc.logf("Error getting zipkin trace: %v", err)
	}

	sc.logf("%s", json)
}
