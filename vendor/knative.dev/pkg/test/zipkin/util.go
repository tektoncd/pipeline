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

// util has constants and helper methods useful for zipkin tracing support.

package zipkin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tracingconfig "knative.dev/pkg/tracing/config"

	"github.com/openzipkin/zipkin-go/model"
	"go.opencensus.io/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/monitoring"
)

const (
	// ZipkinTraceIDHeader HTTP response header key to be used to store Zipkin Trace ID.
	ZipkinTraceIDHeader = "ZIPKIN_TRACE_ID"

	// ZipkinPort is port exposed by the Zipkin Pod
	// https://github.com/knative/serving/blob/main/config/monitoring/200-common/100-zipkin.yaml#L25 configures the Zipkin Port on the cluster.
	ZipkinPort = 9411

	// ZipkinTraceEndpoint port-forwarded zipkin endpoint
	ZipkinTraceEndpoint = "http://localhost:9411/api/v2/trace/"

	// App is the name of this component.
	// This will be used as a label selector.
	appLabel = "zipkin"
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

// SetupZipkinTracingFromConfigTracing setups zipkin tracing like SetupZipkinTracing but retrieving the zipkin configuration
// from config-tracing config map
func SetupZipkinTracingFromConfigTracing(ctx context.Context, kubeClientset kubernetes.Interface, logf logging.FormatLogger, configMapNamespace string) error {
	cm, err := kubeClientset.CoreV1().ConfigMaps(configMapNamespace).Get(ctx, "config-tracing", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error while retrieving config-tracing config map: %w", err)
	}
	c, err := tracingconfig.NewTracingConfigFromConfigMap(cm)
	if err != nil {
		return fmt.Errorf("error while parsing config-tracing config map: %w", err)
	}
	zipkinEndpointURL, err := url.Parse(c.ZipkinEndpoint)
	if err != nil {
		return fmt.Errorf("error while parsing the zipkin endpoint in config-tracing config map: %w", err)
	}
	unparsedPort := zipkinEndpointURL.Port()
	port := uint64(80)
	if unparsedPort != "" {
		port, err = strconv.ParseUint(unparsedPort, 10, 16)
		if err != nil {
			return fmt.Errorf("error while parsing the zipkin endpoint port in config-tracing config map: %w", err)
		}
	}

	namespace, err := parseNamespaceFromHostname(zipkinEndpointURL.Host)
	if err != nil {
		return fmt.Errorf("error while parsing the Zipkin endpoint in config-tracing config map: %w", err)
	}

	return SetupZipkinTracing(ctx, kubeClientset, logf, int(port), namespace)
}

// SetupZipkinTracingFromConfigTracingOrFail is same as SetupZipkinTracingFromConfigTracing, but fails the test if an error happens
func SetupZipkinTracingFromConfigTracingOrFail(ctx context.Context, t testing.TB, kubeClientset kubernetes.Interface, configMapNamespace string) {
	if err := SetupZipkinTracingFromConfigTracing(ctx, kubeClientset, t.Logf, configMapNamespace); err != nil {
		t.Fatal("Error while setup Zipkin tracing:", err)
	}
}

// SetupZipkinTracing sets up zipkin tracing which involves:
//  1. Setting up port-forwarding from localhost to zipkin pod on the cluster
//     (pid of the process doing Port-Forward is stored in a global variable).
//  2. Enable AlwaysSample config for tracing for the SpoofingClient.
//
// The zipkin deployment must have the label app=zipkin
func SetupZipkinTracing(ctx context.Context, kubeClientset kubernetes.Interface, logf logging.FormatLogger, zipkinRemotePort int, zipkinNamespace string) (err error) {
	setupOnce.Do(func() {
		if e := monitoring.CheckPortAvailability(zipkinRemotePort); e != nil {
			err = fmt.Errorf("zipkin port not available on the machine: %w", err)
			return
		}

		zipkinPods, e := monitoring.GetPods(ctx, kubeClientset, appLabel, zipkinNamespace)
		if e != nil {
			err = fmt.Errorf("error retrieving Zipkin pod details: %w", err)
			return
		}

		zipkinPortForwardPID, e = monitoring.PortForward(logf, zipkinPods, ZipkinPort, zipkinRemotePort, zipkinNamespace)
		if e != nil {
			err = fmt.Errorf("error starting kubectl port-forward command: %w", err)
			return
		}

		logf("Zipkin port-forward process started with PID: %d", zipkinPortForwardPID)

		// Applying AlwaysSample config to ensure we propagate zipkin header for every request made by this client.
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
		logf("Successfully setup SpoofingClient for Zipkin Tracing")
	})
	return
}

// SetupZipkinTracingOrFail is same as SetupZipkinTracing, but fails the test if an error happens
func SetupZipkinTracingOrFail(ctx context.Context, t testing.TB, kubeClientset kubernetes.Interface, zipkinRemotePort int, zipkinNamespace string) {
	if err := SetupZipkinTracing(ctx, kubeClientset, t.Logf, zipkinRemotePort, zipkinNamespace); err != nil {
		t.Fatal("Error while setup zipkin tracing:", err)
	}
}

// CleanupZipkinTracingSetup cleans up the Zipkin tracing setup on the machine. This involves killing the process performing port-forward.
// This should be called exactly once in TestMain. Likely in the form:
//
//	func TestMain(m *testing.M) {
//	    os.Exit(func() int {
//	      // Any setup required for the tests.
//	      defer zipkin.CleanupZipkinTracingSetup(logger)
//	      return m.Run()
//	    }())
//	}
func CleanupZipkinTracingSetup(logf logging.FormatLogger) {
	teardownOnce.Do(func() {
		// Because CleanupZipkinTracingSetup only runs once, make sure that now that it has been
		// run, SetupZipkinTracing will no longer setup any port forwarding.
		setupOnce.Do(func() {})

		if !ZipkinTracingEnabled {
			return
		}

		if err := monitoring.Cleanup(zipkinPortForwardPID); err != nil {
			logf("Encountered error killing port-forward process in CleanupZipkinTracingSetup() : %v", err)
			return
		}

		ZipkinTracingEnabled = false
	})
}

// JSONTrace returns a trace for the given traceID. It will continually try to get the trace. If the
// trace it gets has the expected number of spans, then it will be returned. If not, it will try
// again. If it reaches timeout, then it returns everything it has so far with an error.
func JSONTrace(traceID string, expected int, timeout time.Duration) ([]model.SpanModel, error) {
	return JSONTracePred(traceID, timeout, func(trace []model.SpanModel) bool { return len(trace) == expected })
}

// JSONTracePred returns a trace for the given traceID. It will
// continually try to get the trace until the trace spans satisfy the
// predicate. If the timeout is reached then the last fetched trace
// tree if available is returned along with an error.
func JSONTracePred(traceID string, timeout time.Duration, pred func([]model.SpanModel) bool) (trace []model.SpanModel, err error) {
	t := time.After(timeout)
	for !pred(trace) {
		select {
		case <-t:
			return trace, &TimeoutError{
				lastErr: err,
			}
		default:
			trace, err = jsonTrace(traceID)
		}
	}
	return trace, err
}

// TimeoutError is an error returned by JSONTrace if it times out before getting the expected number
// of traces.
type TimeoutError struct {
	lastErr error
}

func (t *TimeoutError) Error() string {
	return fmt.Sprint("timeout getting JSONTrace, most recent error:", t.lastErr)
}

// jsonTrace gets a trace from Zipkin and returns it. Errors returned from this function should be
// retried, as they are likely caused by random problems communicating with Zipkin, or Zipkin
// communicating with its data store.
func jsonTrace(traceID string) ([]model.SpanModel, error) {
	var empty []model.SpanModel

	resp, err := http.Get(ZipkinTraceEndpoint + traceID)
	if err != nil {
		return empty, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return empty, err
	}

	var models []model.SpanModel
	err = json.Unmarshal(body, &models)
	if err != nil {
		return empty, fmt.Errorf("got an error in unmarshalling JSON %q: %w", body, err)
	}
	return models, nil
}

func parseNamespaceFromHostname(hostname string) (string, error) {
	parts := strings.Split(hostname, ".")
	if len(parts) < 3 || !(parts[2] == "svc" || strings.HasPrefix(parts[2], "svc:")) {
		return "", fmt.Errorf("could not extract namespace/name from %s", hostname)
	}
	return parts[1], nil
}
