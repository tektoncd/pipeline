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
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/knative/pkg/test/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	requestInterval = 1 * time.Second
	requestTimeout  = 5 * time.Minute
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

// SpoofingClient is a minimal http client wrapper that spoofs the domain of requests
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
		Client:          http.DefaultClient,
		RequestInterval: requestInterval,
		RequestTimeout:  requestTimeout,
		logger:          logger,
	}

	if !resolvable {
		// If the domain that the Route controller is configured to assign to Route.Status.Domain
		// (the domainSuffix) is not resolvable, we need to retrieve the IP of the endpoint and
		// spoof the Host in our requests.

		// TODO(tcnghia): These probably shouldn't be hard-coded here?
		ingressName := "knative-ingressgateway"
		ingressNamespace := "istio-system"

		ingress, err := kubeClientset.CoreV1().Services(ingressNamespace).Get(ingressName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if len(ingress.Status.LoadBalancer.Ingress) != 1 {
			return nil, fmt.Errorf("Expected exactly one ingress load balancer, instead had %d: %s", len(ingress.Status.LoadBalancer.Ingress), ingress.Status.LoadBalancer.Ingress)
		}

		if ingress.Status.LoadBalancer.Ingress[0].IP == "" {
			return nil, fmt.Errorf("Expected ingress loadbalancer IP for %s to be set, instead was empty", ingressName)
		}

		sc.endpoint = ingress.Status.LoadBalancer.Ingress[0].IP
		sc.domain = domain
	} else {
		// If the domain is resolvable, we can use it directly when we make requests.
		sc.endpoint = domain
	}

	return &sc, nil
}

// Do dispatches to the underlying http.Client.Do, spoofing domains as needed
// and transforming the http.Response into a spoof.Response.
func (sc *SpoofingClient) Do(req *http.Request) (*Response, error) {
	// Controls the Host header, for spoofing.
	if sc.domain != "" {
		req.Host = sc.domain
	}

	// Controls the actual resolution.
	if sc.endpoint != "" {
		req.URL.Host = sc.endpoint
	}

	resp, err := sc.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
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
				sc.logger.Infof("Retrying for TCP timeout %v", err)
				return false, nil
			}
			return true, err
		}

		return inState(resp)
	})

	return resp, err
}
