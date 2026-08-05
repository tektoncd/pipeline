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
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/test/parse"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	knativetest "knative.dev/pkg/test"
	"knative.dev/pkg/test/helpers"
)

const (
	controllerMetricsPort = "9090"
	controllerLabelKey    = "app.kubernetes.io/name"
	controllerLabelValue  = "controller"
)

// --- Helper functions ---

func getControllerPodName(ctx context.Context, t *testing.T, c *clients) string {
	t.Helper()
	ns := getTektonNamespace()
	pods, err := c.KubeClient.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", controllerLabelKey, controllerLabelValue),
	})
	if err != nil {
		t.Fatalf("Failed to list controller pods: %v", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				break
			}
		}
		if allReady {
			return pod.Name
		}
	}
	t.Fatalf("No Running/Ready controller pod found in namespace %s", ns)
	return ""
}

func scrapeControllerMetrics(ctx context.Context, t *testing.T, c *clients) map[string]*dto.MetricFamily {
	t.Helper()
	podName := getControllerPodName(ctx, t, c)
	ns := getTektonNamespace()

	result := c.KubeClient.
		CoreV1().
		RESTClient().
		Get().
		Resource("pods").
		Name(podName + ":" + controllerMetricsPort).
		Namespace(ns).
		SubResource("proxy").
		Suffix("metrics").
		Do(ctx)

	body, err := result.Raw()
	if err != nil {
		t.Fatalf("Failed to scrape metrics from controller: %v", err)
	}

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("Failed to parse metrics: %v", err)
	}
	return families
}

func waitForMetric(ctx context.Context, t *testing.T, c *clients, metricName string, pollTimeout time.Duration) map[string]*dto.MetricFamily {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()
	for {
		families := scrapeControllerMetrics(ctx, t, c)
		if _, ok := families[metricName]; ok {
			return families
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for metric %q to appear (waited %v): %v", metricName, pollTimeout, ctx.Err())
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

func assertMetricExists(t *testing.T, families map[string]*dto.MetricFamily, name string) {
	t.Helper()
	if _, ok := families[name]; !ok {
		t.Errorf("Expected metric %q not found", name)
	}
}

func assertMetricAbsent(t *testing.T, families map[string]*dto.MetricFamily, name string) {
	t.Helper()
	if _, ok := families[name]; ok {
		t.Errorf("Metric %q should have been removed but is still present", name)
	}
}

func assertMetricHasLabel(t *testing.T, families map[string]*dto.MetricFamily, metricName, labelName string) {
	t.Helper()
	fam, ok := families[metricName]
	if !ok {
		t.Errorf("Metric %q not found, cannot check for label %q", metricName, labelName)
		return
	}
	for _, m := range fam.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelName {
				return
			}
		}
	}
	t.Errorf("Metric %q has no sample with label %q", metricName, labelName)
}

func assertMetricHasLabelValue(t *testing.T, families map[string]*dto.MetricFamily, metricName, labelName, labelValue string) {
	t.Helper()
	fam, ok := families[metricName]
	if !ok {
		t.Errorf("Metric %q not found, cannot check for %s=%q", metricName, labelName, labelValue)
		return
	}
	for _, m := range fam.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == labelName && lp.GetValue() == labelValue {
				return
			}
		}
	}
	t.Errorf("Metric %q has no sample with %s=%q", metricName, labelName, labelValue)
}

func counterValue(families map[string]*dto.MetricFamily, metricName, labelName, labelValue string) float64 {
	fam, ok := families[metricName]
	if !ok {
		return 0
	}
	var total float64
	for _, m := range fam.GetMetric() {
		if labelName != "" {
			matched := false
			for _, lp := range m.GetLabel() {
				if lp.GetName() == labelName && lp.GetValue() == labelValue {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		if c := m.GetCounter(); c != nil {
			total += c.GetValue()
		}
	}
	return total
}

func histogramSampleCount(families map[string]*dto.MetricFamily, metricName string) uint64 {
	fam, ok := families[metricName]
	if !ok {
		return 0
	}
	var total uint64
	for _, m := range fam.GetMetric() {
		if h := m.GetHistogram(); h != nil {
			total += h.GetSampleCount()
		}
	}
	return total
}

// counterDelta returns the increase in a counter between two scrapes.
// Returns 0 if the result would be negative (e.g. after a controller restart resets the counter).
func counterDelta(after, before map[string]*dto.MetricFamily, metricName, labelName, labelValue string) float64 {
	a := counterValue(after, metricName, labelName, labelValue)
	b := counterValue(before, metricName, labelName, labelValue)
	if a < b {
		return 0
	}
	return a - b
}

// histogramSampleCountDelta returns the increase in total observations between two scrapes.
func histogramSampleCountDelta(after, before map[string]*dto.MetricFamily, metricName string) uint64 {
	a := histogramSampleCount(after, metricName)
	b := histogramSampleCount(before, metricName)
	if a < b {
		return 0
	}
	return a - b
}

// histogramBucketCountForLabel returns the cumulative bucket count at the given le boundary
// for histogram series whose filterLabel matches filterValue. This lets the caller scope the
// check to a specific TaskRun or PipelineRun series rather than summing across all labels.
func histogramBucketCountForLabel(families map[string]*dto.MetricFamily, metricName, filterLabel, filterValue string, le float64) uint64 {
	fam, ok := families[metricName]
	if !ok {
		return 0
	}
	var total uint64
	for _, m := range fam.GetMetric() {
		hasLabel := false
		for _, lp := range m.GetLabel() {
			if lp.GetName() == filterLabel && lp.GetValue() == filterValue {
				hasLabel = true
				break
			}
		}
		if !hasLabel {
			continue
		}
		if h := m.GetHistogram(); h != nil {
			for _, b := range h.GetBucket() {
				if math.Abs(b.GetUpperBound()-le) < 0.001 {
					total += b.GetCumulativeCount()
				}
			}
		}
	}
	return total
}

func patchCancelPipelineRun(ctx context.Context, t *testing.T, c *clients, name string) {
	t.Helper()
	patches := []jsonpatch.JsonPatchOperation{{
		Operation: "add",
		Path:      "/spec/status",
		Value:     v1.PipelineRunSpecStatusCancelled,
	}}
	patchBytes, err := json.Marshal(patches)
	if err != nil {
		t.Fatalf("Failed to marshal cancel patch for PipelineRun %s: %v", name, err)
	}
	if _, err := c.V1PipelineRunClient.Patch(ctx, name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, ""); err != nil {
		t.Fatalf("Failed to cancel PipelineRun %s: %v", name, err)
	}
}

// --- Main test ---

// TestOTelMetrics is a consolidated e2e test for the OpenCensus-to-OpenTelemetry
// metrics migration (PR #9043). It creates a variety of TaskRuns and PipelineRuns,
// then scrapes the controller /metrics endpoint to verify metric existence,
// histogram observation counts, counter values, label presence, renames, and
// removed metrics.
//
// Runs created:
//   - 3 successful TaskRuns, 2 failed TaskRuns, 1 cancelled TaskRun, 1 timed (~5s) TaskRun
//   - 2 successful single-task PipelineRuns, 1 multi-task (3 tasks) PipelineRun, 1 cancelled PipelineRun
//
// @test:execution=serial
// @test:reason=verifies global controller metrics state after a known set of runs
func TestOTelMetrics(t *testing.T) {
	ctx := t.Context()
	c, namespace := setup(ctx, t)
	knativetest.CleanupOnInterrupt(func() { tearDown(ctx, t, c, namespace) }, t.Logf)
	defer tearDown(ctx, t, c, namespace)

	// Baseline scrape before any resources are created so assertions below can
	// compute deltas and avoid being satisfied by runs from earlier e2e tests.
	baseline := scrapeControllerMetrics(ctx, t, c)

	// ========== Create Tasks ==========

	successTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: pass
    image: mirror.gcr.io/alpine
    script: exit 0
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, successTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create success Task: %v", err)
	}

	failTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: fail
    image: mirror.gcr.io/alpine
    script: exit 1
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, failTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create fail Task: %v", err)
	}

	longTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: wait
    image: mirror.gcr.io/busybox
    script: sleep 600
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, longTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create long Task: %v", err)
	}

	timedTask := parse.MustParseV1Task(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  steps:
  - name: timed
    image: mirror.gcr.io/busybox
    script: sleep 5
`, helpers.ObjectNameForTest(t), namespace))
	if _, err := c.V1TaskClient.Create(ctx, timedTask, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create timed Task: %v", err)
	}

	// ========== Create TaskRuns ==========

	// 3 successful TaskRuns
	var successTRNames []string
	for range 3 {
		name := helpers.ObjectNameForTest(t)
		tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, name, namespace, successTask.Name))
		if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create success TaskRun: %v", err)
		}
		successTRNames = append(successTRNames, name)
	}

	// 2 failed TaskRuns
	var failTRNames []string
	for range 2 {
		name := helpers.ObjectNameForTest(t)
		tr := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, name, namespace, failTask.Name))
		if _, err := c.V1TaskRunClient.Create(ctx, tr, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create fail TaskRun: %v", err)
		}
		failTRNames = append(failTRNames, name)
	}

	// 1 TaskRun to cancel
	cancelTRName := helpers.ObjectNameForTest(t)
	cancelTR := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, cancelTRName, namespace, longTask.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, cancelTR, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create cancel TaskRun: %v", err)
	}

	// 1 timed TaskRun (~5s, lands in le=60 bucket to tolerate pod scheduling variability)
	timedTRName := helpers.ObjectNameForTest(t)
	timedTR := parse.MustParseV1TaskRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  taskRef:
    name: %s
`, timedTRName, namespace, timedTask.Name))
	if _, err := c.V1TaskRunClient.Create(ctx, timedTR, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create timed TaskRun: %v", err)
	}

	// ========== Create PipelineRuns ==========

	// 2 successful single-task PipelineRuns
	var successPRNames []string
	for range 2 {
		pName := helpers.ObjectNameForTest(t)
		p := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: echo
    taskSpec:
      steps:
      - name: hello
        image: mirror.gcr.io/alpine
        script: echo hello
`, pName, namespace))
		if _, err := c.V1PipelineClient.Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create Pipeline: %v", err)
		}

		prName := helpers.ObjectNameForTest(t)
		pr := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, prName, namespace, pName))
		if _, err := c.V1PipelineRunClient.Create(ctx, pr, metav1.CreateOptions{}); err != nil {
			t.Fatalf("Failed to create PipelineRun: %v", err)
		}
		successPRNames = append(successPRNames, prName)
	}

	// 1 multi-task PipelineRun (3 inline tasks)
	multiPName := helpers.ObjectNameForTest(t)
	multiP := parse.MustParseV1Pipeline(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  tasks:
  - name: task-a
    taskSpec:
      steps:
      - name: a
        image: mirror.gcr.io/alpine
        script: echo task-a
  - name: task-b
    taskSpec:
      steps:
      - name: b
        image: mirror.gcr.io/alpine
        script: echo task-b
  - name: task-c
    taskSpec:
      steps:
      - name: c
        image: mirror.gcr.io/alpine
        script: echo task-c
`, multiPName, namespace))
	if _, err := c.V1PipelineClient.Create(ctx, multiP, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create multi-task Pipeline: %v", err)
	}

	multiPRName := helpers.ObjectNameForTest(t)
	multiPR := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineRef:
    name: %s
`, multiPRName, namespace, multiPName))
	if _, err := c.V1PipelineRunClient.Create(ctx, multiPR, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create multi-task PipelineRun: %v", err)
	}

	// 1 PipelineRun to cancel
	cancelPRName := helpers.ObjectNameForTest(t)
	cancelPR := parse.MustParseV1PipelineRun(t, fmt.Sprintf(`
metadata:
  name: %s
  namespace: %s
spec:
  pipelineSpec:
    tasks:
    - name: wait
      taskSpec:
        steps:
        - name: wait
          image: mirror.gcr.io/busybox
          script: sleep 600
`, cancelPRName, namespace))
	if _, err := c.V1PipelineRunClient.Create(ctx, cancelPR, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create cancel PipelineRun: %v", err)
	}

	// ========== Wait for completions ==========

	t.Log("Waiting for successful TaskRuns")
	for _, name := range successTRNames {
		if err := WaitForTaskRunState(ctx, c, name, TaskRunSucceed(name), "TaskRunSucceeded", v1Version); err != nil {
			t.Fatalf("success TaskRun %s did not succeed: %v", name, err)
		}
	}

	t.Log("Waiting for failed TaskRuns")
	for _, name := range failTRNames {
		if err := WaitForTaskRunState(ctx, c, name, TaskRunFailed(name), "TaskRunFailed", v1Version); err != nil {
			t.Fatalf("fail TaskRun %s did not fail: %v", name, err)
		}
	}

	t.Log("Waiting for timed TaskRun")
	if err := WaitForTaskRunState(ctx, c, timedTRName, TaskRunSucceed(timedTRName), "TaskRunSucceeded", v1Version); err != nil {
		t.Fatalf("timed TaskRun did not succeed: %v", err)
	}

	t.Log("Waiting for successful PipelineRuns")
	for _, name := range successPRNames {
		if err := WaitForPipelineRunState(ctx, c, name, timeout, PipelineRunSucceed(name), "PipelineRunSucceeded", v1Version); err != nil {
			t.Fatalf("success PipelineRun %s did not succeed: %v", name, err)
		}
	}

	t.Log("Waiting for multi-task PipelineRun")
	if err := WaitForPipelineRunState(ctx, c, multiPRName, timeout, PipelineRunSucceed(multiPRName), "PipelineRunSucceeded", v1Version); err != nil {
		t.Fatalf("multi-task PipelineRun did not succeed: %v", err)
	}

	// Cancel the long-running TaskRun (reuses cancelTaskRun from taskrun_test.go
	// which waits for Running, then patches spec.status = TaskRunCancelled).
	t.Log("Cancelling TaskRun")
	if err := cancelTaskRun(t, ctx, cancelTRName, c); err != nil {
		t.Fatalf("cancel TaskRun failed: %v", err)
	}
	t.Log("Waiting for cancelled TaskRun")
	if err := WaitForTaskRunState(ctx, c, cancelTRName, FailedWithReason(v1.TaskRunReasonCancelled.String(), cancelTRName), v1.TaskRunReasonCancelled.String(), v1Version); err != nil {
		t.Fatalf("cancel TaskRun did not cancel: %v", err)
	}

	// Cancel the long-running PipelineRun
	t.Log("Waiting for cancel PipelineRun to start running")
	if err := WaitForPipelineRunState(ctx, c, cancelPRName, timeout, Running(cancelPRName), "PipelineRunRunning", v1Version); err != nil {
		t.Fatalf("cancel PipelineRun did not start: %v", err)
	}
	patchCancelPipelineRun(ctx, t, c, cancelPRName)
	t.Log("Waiting for cancelled PipelineRun")
	if err := WaitForPipelineRunState(ctx, c, cancelPRName, timeout, FailedWithReason(v1.PipelineRunReasonCancelled.String(), cancelPRName), v1.PipelineRunReasonCancelled.String(), v1Version); err != nil {
		t.Fatalf("cancel PipelineRun did not cancel: %v", err)
	}

	// ========== Scrape metrics ==========

	t.Log("Scraping controller metrics")
	families := waitForMetric(ctx, t, c, "tekton_pipelines_controller_taskrun_total", 2*time.Minute)
	t.Logf("Scraped %d metric families from controller", len(families))

	// ========== TaskRun counter delta subtests ==========

	t.Run("TaskRun/total_counter", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_taskrun_total")
		assertMetricHasLabelValue(t, families, "tekton_pipelines_controller_taskrun_total", "status", "success")
		assertMetricHasLabelValue(t, families, "tekton_pipelines_controller_taskrun_total", "status", "failed")
		assertMetricHasLabelValue(t, families, "tekton_pipelines_controller_taskrun_total", "status", "cancelled")

		// 3 success + 1 timed + TaskRuns from PipelineRuns (2 single + 3 multi) = at least 9 successes, 2 failures, 1 cancel
		tests := []struct {
			name       string
			metricName string
			labelName  string
			labelValue string
			wantMin    float64
		}{
			{"success", "tekton_pipelines_controller_taskrun_total", "status", "success", 9},
			{"failed", "tekton_pipelines_controller_taskrun_total", "status", "failed", 2},
			{"cancelled", "tekton_pipelines_controller_taskrun_total", "status", "cancelled", 1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				delta := counterDelta(families, baseline, tt.metricName, tt.labelName, tt.labelValue)
				if delta < tt.wantMin {
					t.Errorf("taskrun_total{status=%s} delta = %v, want >= %v", tt.labelValue, delta, tt.wantMin)
				}
				t.Logf("taskrun_total{status=%s} delta: %v", tt.labelValue, delta)
			})
		}
	})

	// ========== Histogram sample count subtests ==========

	t.Run("TaskRun/duration_histogram", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_taskrun_duration_seconds")
		assertMetricHasLabel(t, families, "tekton_pipelines_controller_taskrun_duration_seconds", "namespace")
		assertMetricHasLabel(t, families, "tekton_pipelines_controller_taskrun_duration_seconds", "status")

		// Standalone TaskRuns: 3 success + 2 fail + 1 cancelled + 1 timed = 7
		tests := []struct {
			name       string
			metricName string
			wantMin    uint64
		}{
			{"observations", "tekton_pipelines_controller_taskrun_duration_seconds", 7},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				count := histogramSampleCountDelta(families, baseline, tt.metricName)
				if count < tt.wantMin {
					t.Errorf("taskrun_duration_seconds histogram sample count delta = %d, want >= %d", count, tt.wantMin)
				}
				t.Logf("taskrun_duration_seconds histogram observations delta: %d", count)
			})
		}
	})

	t.Run("TaskRun/duration_histogram_bucket_check", func(t *testing.T) {
		// The timed TaskRun sleeps ~5s but total wall-clock including pod scheduling
		// and image pull can vary. At the default "task" metric level the series carries
		// a "task" label (not "taskrun"), so scope the check to task=timedTask.Name to
		// isolate this run's bucket placement. Use le=60 to tolerate variable image-pull
		// times while still confirming the observation lands in a sub-minute bucket.
		bucketCount := histogramBucketCountForLabel(families, "tekton_pipelines_controller_taskrun_duration_seconds", "task", timedTask.Name, 60)
		if bucketCount < 1 {
			t.Errorf("taskrun_duration_seconds{task=%q} le=60 bucket count = %d, want >= 1", timedTask.Name, bucketCount)
		}
		t.Logf("taskrun_duration_seconds{task=%q} le=60 bucket cumulative count: %d", timedTask.Name, bucketCount)
	})

	t.Run("TaskRun/pod_latency_histogram", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_taskruns_pod_latency_milliseconds")

		// All TaskRuns that got pods scheduled: 7 standalone + 5 from PipelineRuns = 12
		tests := []struct {
			name       string
			metricName string
			wantMin    uint64
		}{
			{"observations", "tekton_pipelines_controller_taskruns_pod_latency_milliseconds", 12},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				count := histogramSampleCountDelta(families, baseline, tt.metricName)
				if count < tt.wantMin {
					t.Errorf("taskruns_pod_latency_milliseconds histogram sample count delta = %d, want >= %d", count, tt.wantMin)
				}
				t.Logf("taskruns_pod_latency_milliseconds histogram observations delta: %d", count)
			})
		}
	})

	t.Run("TaskRun/running_gauge", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_running_taskruns")
	})

	// ========== PipelineRun counter delta subtests ==========

	t.Run("PipelineRun/total_counter", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_pipelinerun_total")
		assertMetricHasLabelValue(t, families, "tekton_pipelines_controller_pipelinerun_total", "status", "success")
		assertMetricHasLabelValue(t, families, "tekton_pipelines_controller_pipelinerun_total", "status", "cancelled")

		// 2 single-task + 1 multi-task = 3 successes, 1 cancelled
		tests := []struct {
			name       string
			metricName string
			labelName  string
			labelValue string
			wantMin    float64
		}{
			{"success", "tekton_pipelines_controller_pipelinerun_total", "status", "success", 3},
			{"cancelled", "tekton_pipelines_controller_pipelinerun_total", "status", "cancelled", 1},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				delta := counterDelta(families, baseline, tt.metricName, tt.labelName, tt.labelValue)
				if delta < tt.wantMin {
					t.Errorf("pipelinerun_total{status=%s} delta = %v, want >= %v", tt.labelValue, delta, tt.wantMin)
				}
				t.Logf("pipelinerun_total{status=%s} delta: %v", tt.labelValue, delta)
			})
		}
	})

	t.Run("PipelineRun/duration_histogram", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_pipelinerun_duration_seconds")
		assertMetricHasLabel(t, families, "tekton_pipelines_controller_pipelinerun_duration_seconds", "namespace")
		assertMetricHasLabel(t, families, "tekton_pipelines_controller_pipelinerun_duration_seconds", "status")

		// 2 single + 1 multi + 1 cancelled = 4
		tests := []struct {
			name       string
			metricName string
			wantMin    uint64
		}{
			{"observations", "tekton_pipelines_controller_pipelinerun_duration_seconds", 4},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				count := histogramSampleCountDelta(families, baseline, tt.metricName)
				if count < tt.wantMin {
					t.Errorf("pipelinerun_duration_seconds histogram sample count delta = %d, want >= %d", count, tt.wantMin)
				}
				t.Logf("pipelinerun_duration_seconds histogram observations delta: %d", count)
			})
		}
	})

	t.Run("PipelineRun/taskrun_duration_histogram", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds")

		// 2 from single-task PRs + 3 from multi-task PR = 5
		tests := []struct {
			name       string
			metricName string
			wantMin    uint64
		}{
			{"observations", "tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds", 5},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				count := histogramSampleCountDelta(families, baseline, tt.metricName)
				if count < tt.wantMin {
					t.Errorf("pipelinerun_taskrun_duration_seconds histogram sample count delta = %d, want >= %d", count, tt.wantMin)
				}
				t.Logf("pipelinerun_taskrun_duration_seconds histogram observations delta: %d", count)
			})
		}
	})

	t.Run("PipelineRun/running_gauge", func(t *testing.T) {
		assertMetricExists(t, families, "tekton_pipelines_controller_running_pipelineruns")
	})

	// ========== Metric rename subtests ==========

	tests := []struct {
		name   string
		prefix string
		errMsg string
	}{
		{
			name:   "Renames/workqueue_uses_kn_prefix",
			prefix: "kn_workqueue_",
			errMsg: "tekton_pipelines_controller_workqueue_",
		},
		{
			name:   "Renames/go_runtime_uses_standard_prefix",
			prefix: "go_",
			errMsg: "tekton_pipelines_controller_go_",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for name := range families {
				if strings.HasPrefix(name, tt.prefix) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected at least one %s* metric, found none", tt.prefix)
			}
			for name := range families {
				if strings.HasPrefix(name, tt.errMsg) {
					t.Errorf("Old metric %q still present; expected %s* prefix", name, tt.prefix)
				}
			}
		})
	}

	// ========== Removed metric checks ==========
	// These metrics were removed as part of the OTel migration.
	// TODO: Remove these assertions in the future release when no OpenCensus based release is supported.

	for _, metricName := range []string{
		"tekton_pipelines_controller_reconcile_count",
		"tekton_pipelines_controller_reconcile_latency",
	} {
		assertMetricAbsent(t, families, metricName)
	}
}
