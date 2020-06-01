/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

import (
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultConnTimeout specifies a short default connection timeout
	// to avoid hitting the issue fixed in
	// https://github.com/kubernetes/kubernetes/pull/72534 but only
	// avalailable after Kubernetes 1.14.
	//
	// Our connections are usually between pods in the same cluster
	// like activator <-> queue-proxy, or even between containers
	// within the same pod queue-proxy <-> user-container, so a
	// smaller connect timeout would be justifiable.
	//
	// We should consider exposing this as a configuration.
	DefaultConnTimeout = 200 * time.Millisecond

	// DefaultDrainTimeout is the time that Knative components on the data
	// path will wait before shutting down server, but after starting to fail
	// readiness probes to ensure network layer propagation and so that no requests
	// are routed to this pod.
	DefaultDrainTimeout = 30 * time.Second

	// UserAgentKey is the constant for header "User-Agent".
	UserAgentKey = "User-Agent"

	// ProbeHeaderName is the name of a header that can be added to
	// requests to probe the knative networking layer.  Requests
	// with this header will not be passed to the user container or
	// included in request metrics.
	ProbeHeaderName = "K-Network-Probe"

	// Since K8s 1.8, prober requests have
	//   User-Agent = "kube-probe/{major-version}.{minor-version}".
	KubeProbeUAPrefix = "kube-probe/"

	// Istio with mTLS rewrites probes, but their probes pass a different
	// user-agent.  So we augment the probes with this header.
	KubeletProbeHeaderName = "K-Kubelet-Probe"
)

// IsKubeletProbe returns true if the request is a Kubernetes probe.
func IsKubeletProbe(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("User-Agent"), KubeProbeUAPrefix) ||
		r.Header.Get(KubeletProbeHeaderName) != ""
}
