/*
Copyright 2018 The Knative Authors

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

package prometheus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/test/logging"
	"knative.dev/pkg/test/monitoring"
)

const (
	prometheusPort = 9090
	appLabel       = "prometheus"
)

var (
	// sync.Once variable to ensure we execute zipkin setup only once.
	setupOnce sync.Once

	// sync.Once variable to ensure we execute zipkin cleanup only if zipkin is setup and it is executed only once.
	teardownOnce sync.Once
)

// PromProxy defines a proxy to the prometheus server
type PromProxy struct {
	Namespace string
	processID int
}

// Setup performs a port forwarding for app prometheus-test in given namespace
func (p *PromProxy) Setup(kubeClientset *kubernetes.Clientset, logf logging.FormatLogger) {
	setupOnce.Do(func() {
		if err := monitoring.CheckPortAvailability(prometheusPort); err != nil {
			logf("Prometheus port not available: %v", err)
			return
		}

		promPods, err := monitoring.GetPods(kubeClientset, appLabel, p.Namespace)
		if err != nil {
			logf("Error retrieving prometheus pod details: %v", err)
			return
		}

		p.processID, err = monitoring.PortForward(logf, promPods, prometheusPort, prometheusPort, p.Namespace)
		if err != nil {
			logf("Error starting kubectl port-forward command: %v", err)
			return
		}
	})
}

// Teardown will kill the port forwarding process if running.
func (p *PromProxy) Teardown(logf logging.FormatLogger) {
	teardownOnce.Do(func() {
		if err := monitoring.Cleanup(p.processID); err != nil {
			logf("Encountered error killing port-forward process: %v", err)
			return
		}
	})
}

// PromAPI gets a handle to the prometheus API
func PromAPI() (v1.API, error) {
	client, err := api.NewClient(api.Config{Address: fmt.Sprintf("http://localhost:%d", prometheusPort)})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

// AllowPrometheusSync sleeps for sometime to allow prometheus time to scrape the metrics.
func AllowPrometheusSync(logf logging.FormatLogger) {
	logf("Sleeping to allow prometheus to record metrics...")
	time.Sleep(30 * time.Second)
}

// RunQuery runs a prometheus query and returns the metric value
func RunQuery(ctx context.Context, logf logging.FormatLogger, promAPI v1.API, query string) (float64, error) {
	logf("Running prometheus query: %s", query)

	value, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}

	return VectorValue(value)
}

// RunQueryRange runs a prometheus query over the given range
func RunQueryRange(ctx context.Context, logf logging.FormatLogger, promAPI v1.API, query string, r v1.Range) (float64, error) {
	logf("Running prometheus query: %s", query)

	value, err := promAPI.QueryRange(ctx, query, r)
	if err != nil {
		return 0, err
	}

	return VectorValue(value)
}

// VectorValue gets the vector value from the value type
func VectorValue(val model.Value) (float64, error) {
	if val.Type() != model.ValVector {
		return 0, fmt.Errorf("Value type is %s. Expected: Valvector", val.String())
	}
	value := val.(model.Vector)
	if len(value) == 0 {
		return 0, errors.New("Query returned no results")
	}

	return float64(value[0].Value), nil
}
