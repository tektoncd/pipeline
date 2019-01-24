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

// spoof contains logic to make polling HTTP requests against an endpoint with optional host spoofing.

package spoof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/knative/pkg/test/logging"
	zipkin "github.com/knative/pkg/test/zipkin"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/trace"
)

const (
	requestInterval = 1 * time.Second
	requestTimeout  = 5 * time.Minute
	// TODO(tcnghia): These probably shouldn't be hard-coded here?
	ingressNamespace = "istio-system"
)

// Temporary work around the upgrade test issue for knative/serving#2434.
// TODO(lichuqiang): remove the backward compatibility for knative-ingressgateway
// once knative/serving#2434 is merged
var ingressNames = []string{"knative-ingressgateway", "istio-ingressgateway"}

// Response is a stripped down subset of http.Response. The is primarily useful
// for ResponseCheckers to inspect the response body without consuming it.
// Notably, Body is a byte slice instead of an io.ReadCloser.
type Response struct {
	Status     string
	StatusCode int
	Header     http.Header
	Body       []byte
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

	logger *logging.BaseLogger
}

// New returns a SpoofingClient that rewrites requests if the target domain is not `resolveable`.
// It does this by looking up the ingress at construction time, so reusing a client will not
// follow the ingress if it moves (or if there are multiple ingresses).
//
// If that's a problem, see test/request.go#WaitForEndpointState for oneshot spoofing.
func New(kubeClientset *kubernetes.Clientset, logger *logging.BaseLogger, domain string, resolvable bool) (*SpoofingClient, error) {
	sc := SpoofingClient{
		Client:          &http.Client{Transport: &ochttp.Transport{Propagation: &b3.HTTPFormat{}}}, // Using ochttp Transport required for zipkin-tracing
		RequestInterval: requestInterval,
		RequestTimeout:  requestTimeout,
		logger:          logger,
	}

	if !resolvable {
		// If the domain that the Route controller is configured to assign to Route.Status.Domain
		// (the domainSuffix) is not resolvable, we need to retrieve the IP of the endpoint and
		// spoof the Host in our requests.
		e, err := GetServiceEndpoint(kubeClientset)
		if err != nil {
			return nil, err
		}

		sc.endpoint = *e
		sc.domain = domain
	} else {
		// If the domain is resolvable, we can use it directly when we make requests.
		sc.endpoint = domain
	}

	return &sc, nil
}

// GetServiceEndpoint gets the endpoint IP or hostname to use for the service.
func GetServiceEndpoint(kubeClientset *kubernetes.Clientset) (*string, error) {
	var err error

	for _, ingressName := range ingressNames {
		var ingress *v1.Service
		ingress, err = kubeClientset.CoreV1().Services(ingressNamespace).Get(ingressName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		var endpoint string
		endpoint, err = endpointFromService(ingress)
		if err != nil {
			continue
		}

		return &endpoint, nil
	}
	return nil, err
}

// endpointFromService extracts the endpoint from the service's ingress.
func endpointFromService(svc *v1.Service) (string, error) {
	ingresses := svc.Status.LoadBalancer.Ingress
	if len(ingresses) != 1 {
		return "", fmt.Errorf("Expected exactly one ingress load balancer, instead had %d: %v", len(ingresses), ingresses)
	}
	itu := ingresses[0]

	switch {
	case itu.IP != "":
		return itu.IP, nil
	case itu.Hostname != "":
		return itu.Hostname, nil
	default:
		return "", fmt.Errorf("Expected ingress loadbalancer IP or hostname for %s to be set, instead was empty", svc.Name)
	}
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

	return &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       body,
	}, nil
}

// Poll executes an http request until it satisfies the inState condition or encounters an error.
func (sc *SpoofingClient) Poll(req *http.Request, inState ResponseChecker) (*Response, error) {
	var (
		resp *Response
		err  error
	)

	err = wait.PollImmediate(sc.RequestInterval, sc.RequestTimeout, func() (bool, error) {
		resp, err = sc.Do(req)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				sc.logger.Infof("Retrying %s for TCP timeout %v", req.URL.String(), err)
				return false, nil
			}
			return true, err
		}

		return inState(resp)
	})

	return resp, err
}

// LogZipkinTrace provides support to log Zipkin Trace for param: traceID
func (sc *SpoofingClient) LogZipkinTrace(traceID string) error {
	if err := zipkin.CheckZipkinPortAvailability(); err == nil {
		return errors.New("port-forwarding for Zipkin is not-setup. Failing Zipkin Trace retrieval")
	}

	sc.logger.Infof("Logging Zipkin Trace: %s", traceID)

	zipkinTraceEndpoint := zipkin.ZipkinTraceEndpoint + traceID
	// Sleep to ensure all traces are correctly pushed on the backend.
	time.Sleep(5 * time.Second)
	resp, err := http.Get(zipkinTraceEndpoint)
	if err != nil {
		return fmt.Errorf("Error retrieving Zipkin trace: %v", err)
	}

	trace, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading Zipkin trace response: %v", err)
	}

	var prettyJSON bytes.Buffer
	error := json.Indent(&prettyJSON, trace, "", "\t")
	if error != nil {
		return fmt.Errorf("JSON Parser Error while trying for Pretty-Format: %v, Original Response: %s", error, string(trace))
	}
	sc.logger.Infof(prettyJSON.String())

	return nil
}
