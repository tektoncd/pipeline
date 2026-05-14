//go:build e2e

/*
Copyright 2026 The Tekton Authors

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

package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/resolvermetrics"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	resolverMetricsPort     = "9090"
	resolverNamespace       = "tekton-pipelines-resolvers"
	resolverDeployment      = "tekton-pipelines-remote-resolvers"
	durationMetricName      = "tekton_pipelines_resolver_resolution_duration_seconds"
	totalMetricName         = "tekton_pipelines_resolver_resolution_total"
	cacheHitMetricName      = "tekton_pipelines_resolver_cache_hit_total"
	cacheMissMetricName     = "tekton_pipelines_resolver_cache_miss_total"
	singleflightDedupMetric = "tekton_pipelines_resolver_singleflight_dedup_total"
)

// TestResolverMetrics_SuccessfulResolution verifies that successful resolution
// increments the duration histogram and total counter with status=success.
// @test:execution=parallel
func TestResolverMetrics_SuccessfulResolution(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	taskName := helpers.ObjectNameForTest(t)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: echo hello
`, taskName, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	trName := helpers.ObjectNameForTest(t)
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: %s
    - name: namespace
      value: %s
`, trName, namespace, taskName, namespace))

	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	t.Logf("Waiting for TaskRun %s to succeed", trName)
	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to succeed: %s", err)
	}

	metrics := scrapeResolverMetrics(ctx, t, c)

	assertMetricExists(t, metrics, totalMetricName)
	assertMetricHasLabel(t, metrics, totalMetricName, "resolver_type", "cluster")
	assertMetricHasLabel(t, metrics, totalMetricName, "status", resolvermetrics.StatusSuccess)
	assertMetricExists(t, metrics, durationMetricName)
}

// TestResolverMetrics_FailedResolution verifies that a failed resolution
// (nonexistent resource) records metrics with status=error.
// @test:execution=parallel
func TestResolverMetrics_FailedResolution(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, clusterFeatureFlags)

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	prName := helpers.ObjectNameForTest(t)
	pipelineRun := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    resolver: cluster
    params:
    - name: kind
      value: pipeline
    - name: name
      value: does-not-exist-for-metrics-test
    - name: namespace
      value: %s
`, prName, namespace, namespace))

	if _, err := c.V1PipelineRunClient.Create(ctx, pipelineRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create PipelineRun: %s", err)
	}

	t.Logf("Waiting for PipelineRun %s to fail", prName)
	if err := WaitForPipelineRunState(ctx, c, prName, timeout,
		FailedWithReason(pipelinerun.ReasonCouldntGetPipeline, prName),
		"PipelineRunFailed", v1Version); err != nil {
		t.Fatalf("Error waiting for PipelineRun to fail: %s", err)
	}

	metrics := scrapeResolverMetrics(ctx, t, c)

	assertMetricExists(t, metrics, totalMetricName)
	assertMetricCounterValue(t, metrics, totalMetricName,
		map[string]string{"resolver_type": "cluster", "status": resolvermetrics.StatusError},
	)
}

// TestResolverMetrics_MultipleResolverTypes verifies that metrics are correctly
// labeled per resolver type when multiple resolver types are used.
// @test:execution=parallel
func TestResolverMetrics_MultipleResolverTypes(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t, requireAllGates(map[string]string{
		"enable-cluster-resolver": "true",
		"enable-http-resolver":    "true",
		"enable-api-fields":       "beta",
	}))

	t.Parallel()

	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Test 1: Cluster resolver
	taskName := helpers.ObjectNameForTest(t)
	task := parse.MustParseV1Task(t, fmt.Sprintf(`
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: echo
    image: mirror.gcr.io/ubuntu
    script: echo hello
`, taskName, namespace))
	if _, err := c.V1TaskClient.Create(ctx, task, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Task: %s", err)
	}

	trName := helpers.ObjectNameForTest(t)
	taskRun := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    resolver: cluster
    params:
    - name: kind
      value: task
    - name: name
      value: %s
    - name: namespace
      value: %s
`, trName, namespace, taskName, namespace))

	if _, err := c.V1TaskRunClient.Create(ctx, taskRun, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create TaskRun: %s", err)
	}

	if err := WaitForTaskRunState(ctx, c, trName, TaskRunSucceed(trName), "TaskRunSuccess", v1Version); err != nil {
		t.Fatalf("Error waiting for TaskRun to succeed: %s", err)
	}

	metrics := scrapeResolverMetrics(ctx, t, c)

	// Verify cluster resolver metrics exist
	assertMetricHasLabel(t, metrics, totalMetricName, "resolver_type", "cluster")
}

// scrapeResolverMetrics fetches Prometheus metrics from the resolver pod.
func scrapeResolverMetrics(ctx context.Context, t *testing.T, c *clients) map[string]*dto.MetricFamily {
	t.Helper()

	pods, err := c.KubeClient.CoreV1().Pods(resolverNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=resolvers",
	})
	if err != nil || len(pods.Items) == 0 {
		t.Fatalf("Failed to find resolver pod: %v", err)
	}

	podName := pods.Items[0].Name

	result := c.KubeClient.
		CoreV1().
		RESTClient().
		Get().
		Resource("pods").
		Name(podName + ":" + resolverMetricsPort).
		Namespace(resolverNamespace).
		SubResource("proxy").
		Suffix("metrics").
		Do(ctx)

	body, err := result.Raw()
	if err != nil {
		t.Fatalf("Failed to scrape resolver metrics from pod %s: %v", podName, err)
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("Failed to parse resolver metrics: %v", err)
	}

	return families
}

// assertMetricExists checks that a metric family exists in the scraped output.
func assertMetricExists(t *testing.T, families map[string]*dto.MetricFamily, name string) {
	t.Helper()
	if _, ok := families[name]; !ok {
		t.Errorf("Expected metric %q to exist, but it was not found. Available metrics: %s",
			name, availableMetricNames(families))
	}
}

// assertMetricHasLabel checks that at least one data point has the given label value.
func assertMetricHasLabel(t *testing.T, families map[string]*dto.MetricFamily, name, labelName, labelValue string) {
	t.Helper()
	family, ok := families[name]
	if !ok {
		t.Errorf("Metric %q not found", name)
		return
	}

	for _, m := range family.GetMetric() {
		for _, label := range m.GetLabel() {
			if label.GetName() == labelName && label.GetValue() == labelValue {
				return
			}
		}
	}

	t.Errorf("Metric %q has no data point with label %s=%s", name, labelName, labelValue)
}

// assertMetricCounterValue checks that a counter metric has at least one data point
// matching the given labels and with a value > 0.
func assertMetricCounterValue(t *testing.T, families map[string]*dto.MetricFamily, name string, labels map[string]string) {
	t.Helper()
	family, ok := families[name]
	if !ok {
		t.Errorf("Metric %q not found", name)
		return
	}

	for _, m := range family.GetMetric() {
		if matchLabels(m, labels) {
			if m.GetCounter() != nil && m.GetCounter().GetValue() > 0 {
				return
			}
		}
	}

	t.Errorf("Metric %q has no data point > 0 with labels %v", name, labels)
}

func matchLabels(m *dto.Metric, expected map[string]string) bool {
	labelMap := make(map[string]string)
	for _, label := range m.GetLabel() {
		labelMap[label.GetName()] = label.GetValue()
	}
	for k, v := range expected {
		if labelMap[k] != v {
			return false
		}
	}
	return true
}

func availableMetricNames(families map[string]*dto.MetricFamily) string {
	names := make([]string, 0, len(families))
	for name := range families {
		if strings.HasPrefix(name, "tekton_") {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}
