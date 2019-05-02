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

//util has constants and helper methods useful for zipkin tracing support.

package zipkin

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/knative/pkg/test/logging"
	"github.com/knative/pkg/test/monitoring"
	"go.opencensus.io/trace"
	"k8s.io/client-go/kubernetes"
)

const (
	//ZipkinTraceIDHeader HTTP response header key to be used to store Zipkin Trace ID.
	ZipkinTraceIDHeader = "ZIPKIN_TRACE_ID"

	// ZipkinPort is port exposed by the Zipkin Pod
	// https://github.com/knative/serving/blob/master/config/monitoring/200-common/100-zipkin.yaml#L25 configures the Zipkin Port on the cluster.
	ZipkinPort = 9411

	// ZipkinTraceEndpoint port-forwarded zipkin endpoint
	ZipkinTraceEndpoint = "http://localhost:9411/api/v2/trace/"

	// App is the name of this component.
	// This will be used as a label selector
	app = "zipkin"

	// Namespace we are using for istio components
	istioNs = "istio-system"
)

var (
	zipkinPortForwardPID int

	// ZipkinTracingEnabled variable indicating if zipkin tracing is enabled.
	ZipkinTracingEnabled = false

	// sync.Once variable to ensure we execute zipkin setup only once.
	setupOnce sync.Once

	// sync.Once variable to ensure we execute zipkin cleanup only if zipkin is setup and it is executed only once.
	teardownOnce sync.Once
)

// SetupZipkinTracing sets up zipkin tracing which involves:
// 1. Setting up port-forwarding from localhost to zipkin pod on the cluster
//    (pid of the process doing Port-Forward is stored in a global variable).
// 2. Enable AlwaysSample config for tracing.
func SetupZipkinTracing(kubeClientset *kubernetes.Clientset, logf logging.FormatLogger) {
	setupOnce.Do(func() {
		if err := monitoring.CheckPortAvailability(ZipkinPort); err != nil {
			logf("Zipkin port not available on the machine: %v", err)
			return
		}

		zipkinPods, err := monitoring.GetPods(kubeClientset, app, istioNs)
		if err != nil {
			logf("Error retrieving Zipkin pod details: %v", err)
			return
		}

		zipkinPortForwardPID, err = monitoring.PortForward(logf, zipkinPods, ZipkinPort, ZipkinPort, istioNs)
		if err != nil {
			logf("Error starting kubectl port-forward command: %v", err)
			return
		}

		logf("Zipkin port-forward process started with PID: %d", zipkinPortForwardPID)

		// Applying AlwaysSample config to ensure we propagate zipkin header for every request made by this client.
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
		logf("Successfully setup SpoofingClient for Zipkin Tracing")
		ZipkinTracingEnabled = true
	})
}

// CleanupZipkinTracingSetup cleans up the Zipkin tracing setup on the machine. This involves killing the process performing port-forward.
func CleanupZipkinTracingSetup(logf logging.FormatLogger) {
	teardownOnce.Do(func() {
		if !ZipkinTracingEnabled {
			return
		}

		if err := monitoring.Cleanup(zipkinPortForwardPID); err != nil {
			logf("Encountered error killing port-forward process in CleanupZipkingTracingSetup() : %v", err)
			return
		}

		ZipkinTracingEnabled = false
	})
}

// CheckZipkinPortAvailability checks to see if Zipkin Port is available on the machine.
// returns error if the port is not available.
func CheckZipkinPortAvailability() error {
	return monitoring.CheckPortAvailability(ZipkinPort)
}

// JSONTrace returns a trace for the given traceId in JSON format
func JSONTrace(traceID string) (string, error) {
	// Check if zipkin port forwarding is setup correctly
	if err := CheckZipkinPortAvailability(); err == nil {
		return "", err
	}

	resp, err := http.Get(ZipkinTraceEndpoint + traceID)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	trace, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, trace, "", "\t")
	if err != nil {
		return "", err
	}

	return prettyJSON.String(), nil
}
