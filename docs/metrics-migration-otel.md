<!--
---
linkTitle: "OpenTelemetry Metrics Migration"
weight: 305
---
-->

# Metrics Migration: OpenCensus to OpenTelemetry

**Breaking Changes - Action Required**

Related PR: [#9043](https://github.com/tektoncd/pipeline/pull/9043)

---

## Executive Summary

Tekton Pipelines has migrated from **OpenCensus** (deprecated) to **OpenTelemetry** (CNCF standard). This is a **BREAKING CHANGE** requiring updates to dashboards, alerts, and configuration.

### What Changed

**Infrastructure Metrics (HIGH IMPACT)**
* Workqueue: `tekton_pipelines_controller_workqueue_*` → `kn_workqueue_*`
* K8s Client: `tekton_pipelines_controller_client_*` → Standard HTTP/K8s metrics
* Go Runtime: `tekton_pipelines_controller_go_*` → `go_*`

**Core Tekton Metrics (LOW IMPACT)**
* Names preserved: `tekton_pipelines_controller_pipelinerun_*` and `tekton_pipelines_controller_taskrun_*`
* Labels unchanged by default
* Optional `reason` label available (disabled by default, see Section 2.2)

**Configuration (MEDIUM IMPACT)**
* `metrics.backend-destination` → `metrics-protocol`
* New OTLP export options (gRPC, HTTP/protobuf)

**Removed Metrics (LOW IMPACT)**
* `reconcile_count` and `reconcile_latency` → Use `kn_workqueue_*` instead

### Action Required

1. **Update ConfigMap**: `metrics.backend-destination: prometheus` → `metrics-protocol: prometheus`
2. **Update Dashboards**: Replace infrastructure metric names (see tables below)
3. **Update Alerts**: Use new metric names
4. **Test First**: Validate in staging before production

---

## 1. Infrastructure Metrics - Breaking Changes

### 1.1 Workqueue Metrics

| Old Name (OpenCensus) | New Name (OpenTelemetry) | Type | Change |
| :---- | :---- | :---- | :---- |
| `tekton_pipelines_controller_workqueue_adds_total` | `kn_workqueue_adds_total` | Counter | Prefix change |
| `tekton_pipelines_controller_workqueue_depth` | `kn_workqueue_depth` | Gauge | Prefix change |
| `tekton_pipelines_controller_workqueue_queue_latency_seconds` | `kn_workqueue_queue_duration_seconds` | Histogram | Renamed latency → duration |
| `tekton_pipelines_controller_workqueue_work_duration_seconds` | `kn_workqueue_process_duration_seconds` | Histogram | Renamed work → process |
| `tekton_pipelines_controller_workqueue_retries_total` | `kn_workqueue_retries_total` | Counter | Prefix change |
| `tekton_pipelines_controller_workqueue_unfinished_work_seconds` | `kn_workqueue_unfinished_work_seconds` | Gauge | Prefix change |

### 1.2 Kubernetes Client Metrics

| Old Name (OpenCensus) | New Name (OpenTelemetry) | Type | Change |
| :---- | :---- | :---- | :---- |
| `tekton_pipelines_controller_client_latency` | `http_client_request_duration_seconds` | Histogram | Standard HTTP metric |
| `tekton_pipelines_controller_client_results` | `kn_k8s_client_http_response_status_code_total` | Counter | Detailed status tracking |

### 1.3 Go Runtime Metrics

All Go runtime metrics renamed: `tekton_pipelines_controller_go_*` → `go_*`

**Examples:**
* `tekton_pipelines_controller_go_goroutines` → `go_goroutines`
* `tekton_pipelines_controller_go_memstats_alloc_bytes` → `go_memstats_alloc_bytes`
* `tekton_pipelines_controller_go_threads` → `go_threads`

---

## 2. Core Tekton Metrics - No Breaking Changes

Core metrics retain their names **and labels**. By default, there are **no changes** to PipelineRun and TaskRun metrics.

| Metric Name | Labels | Change |
| :---- | :---- | :---- |
| `tekton_pipelines_controller_pipelinerun_duration_seconds` | pipeline, status | ✅ No change |
| `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds` | pipeline, pipelinerun, status, task, taskrun | ✅ No change |
| `tekton_pipelines_controller_taskrun_duration_seconds` | status, task, taskrun | ✅ No change |
| `tekton_pipelines_controller_pipelinerun_total` | status | ✅ No change |
| `tekton_pipelines_controller_taskrun_total` | status | ✅ No change |

### 2.1 Backward Compatibility

These metrics maintain **full backward compatibility** with the OpenCensus implementation when using default configuration.

### 2.2 Optional Feature: `reason` Label (Disabled by Default)

An **opt-in** feature allows adding a `reason` label to duration metrics for more granular failure analysis.

**To enable** (not recommended for high-volume clusters):
```yaml
data:
  metrics.count.enable-reason: "true"
```

**When enabled**, duration metrics gain the `reason` label:

| Metric Name | Default Labels | With `enable-reason: true` |
| :---- | :---- | :---- |
| `tekton_pipelines_controller_pipelinerun_duration_seconds` | pipeline, status | pipeline, status, **reason** |
| `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds` | pipeline, pipelinerun, status, task, taskrun | All previous + **reason** |
| `tekton_pipelines_controller_taskrun_duration_seconds` | status, task, taskrun | All previous + **reason** |

**Reason values:** `Succeeded`, `Failed`, `Completed`, `Cancelled`, `PipelineRunCancelled`, `TimedOut`, `StoppedRunFinally`, `CancelledRunFinally`

**⚠️ Cardinality Impact:** Enabling this increases time series by 3-5x. Use `sum by(le)` aggregation in queries to mitigate.

**Note:** Total counters (`*_total`) never include the `reason` label, even when enabled.

### 2.3 Preserved Behavior

| Feature | Implementation |
| :---- | :---- |
| Pod latency metric | Remains a **Gauge** (metric type preserved) |
| Duration histogram buckets | Custom explicit buckets: [10s, 30s, 1m, 5m, 15m, 30m, 1h, 1.5h, 3h, 6h, +Inf] |
| Metric cardinality | **Unchanged** with default configuration |

---

## 3. Removed Metrics

| Removed Metric | Replacement | Migration |
| :---- | :---- | :---- |
| `tekton_pipelines_controller_reconcile_count` | `kn_workqueue_adds_total` | Monitor reconciliation activity |
| `tekton_pipelines_controller_reconcile_latency` | `kn_workqueue_process_duration_seconds` | Monitor reconciliation duration |

---

## 4. Configuration Changes

### 4.1 Before (OpenCensus) - REMOVED

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-observability
  namespace: tekton-pipelines
data:
  metrics.backend-destination: prometheus  # DEPRECATED - no longer supported
```

### 4.2 After (OpenTelemetry) - Current

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-observability
  namespace: tekton-pipelines
data:
  # ===== Metrics Configuration =====
  metrics-protocol: prometheus                    # Required: prometheus, grpc, http/protobuf, none
  metrics-endpoint: ""                            # Optional: OTLP endpoint (for grpc/http)
  metrics-export-interval: "30s"                  # Optional: Export frequency

  # ===== Tracing Configuration (NEW) =====
  tracing-protocol: none                          # Optional: grpc, http/protobuf, none, stdout
  tracing-endpoint: ""                            # Optional: OTLP tracing endpoint
  tracing-sampling-rate: "1.0"                    # Optional: 0.0-1.0 (1.0 = 100%)

  # ===== Runtime Configuration =====
  runtime-profiling: disabled                     # Optional: enabled, disabled
  runtime-export-interval: "15s"                  # Optional: Runtime metrics export interval

  # ===== Tekton-Specific Settings =====
  metrics.taskrun.level: "task"                   # task, namespace, cluster
  metrics.taskrun.duration-type: "histogram"      # histogram, gauge
  metrics.pipelinerun.level: "pipeline"           # pipeline, namespace, cluster
  metrics.pipelinerun.duration-type: "histogram"  # histogram, gauge
  metrics.count.enable-reason: "false"            # Add reason label to duration metrics (see Section 2.2)
```

**Key Change:** `metrics.backend-destination` → `metrics-protocol`

### 4.3 Supported Protocols

| Protocol | Use Case | Endpoint Required | Format |
| :---- | :---- | :---- | :---- |
| **prometheus** | Prometheus scraping (default) | No | Prometheus exposition |
| **grpc** | OTLP over gRPC | Yes | OpenTelemetry Protocol |
| **http** / **http/protobuf** | OTLP over HTTP | Yes | OpenTelemetry Protocol |
| **none** | Disable metrics | No | N/A |

### 4.4 Configuration Examples

**Example 1: Prometheus (Default)**
```yaml
data:
  metrics-protocol: prometheus
```

**Example 2: OTLP gRPC to OpenTelemetry Collector**
```yaml
data:
  metrics-protocol: grpc
  metrics-endpoint: "otel-collector.observability.svc.cluster.local:4317"
  metrics-export-interval: "30s"
```

**Example 3: OTLP HTTP with Tracing**
```yaml
data:
  metrics-protocol: http/protobuf
  metrics-endpoint: "http://otel-collector.observability.svc.cluster.local:4318/v1/metrics"

  tracing-protocol: grpc
  tracing-endpoint: "otel-collector.observability.svc.cluster.local:4317"
  tracing-sampling-rate: "0.1"  # 10% sampling
```

---

## 5. Migration Steps

### 5.1 Pre-Migration Checklist

- [ ] Inventory all dashboards using Tekton metrics
- [ ] Inventory all alerts using Tekton metrics
- [ ] Back up `config-observability` ConfigMap
- [ ] Plan maintenance window

### 5.2 Step-by-Step Process

**Step 1: Update Configuration**

```bash
kubectl edit configmap config-observability -n tekton-pipelines
# Change: metrics.backend-destination: prometheus
# To:     metrics-protocol: prometheus
```

**Step 2: Upgrade Tekton Pipelines**

```bash
# Apply the new version containing the OTel migration
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# Wait for rollout
kubectl rollout status deployment/tekton-pipelines-controller -n tekton-pipelines
```

**Step 3: Verify Metrics Endpoint**

```bash
# Port-forward to controller
kubectl port-forward -n tekton-pipelines deployment/tekton-pipelines-controller 9090:9090

# Check metrics (in another terminal)
curl http://localhost:9090/metrics | grep -E "(kn_workqueue|tekton_pipelines_controller)"
```

**Step 4: Update Dashboards**

| Old Query | New Query |
| :---- | :---- |
| `rate(tekton_pipelines_controller_workqueue_adds_total[5m])` | `rate(kn_workqueue_adds_total[5m])` |
| `tekton_pipelines_controller_pipelinerun_duration_seconds` | ✅ **No change needed** |
| `tekton_pipelines_controller_go_goroutines` | `go_goroutines` |

**Note:** Core Tekton metrics (PipelineRun/TaskRun) queries remain unchanged with default configuration.

**Step 5: Update Alerts**

Before:
```yaml
- alert: HighWorkqueueDepth
  expr: tekton_pipelines_controller_workqueue_depth > 100
```

After:
```yaml
- alert: HighWorkqueueDepth
  expr: kn_workqueue_depth > 100
```

**Step 6: Verify and Monitor**

- [ ] Confirm metrics appear in Prometheus
- [ ] Check dashboards display correctly
- [ ] Verify alerts work
- [ ] Monitor for errors in controller logs

---

## 6. Query Examples

### Example 1: PipelineRun Success Rate

**Before:**
```promql
sum(rate(tekton_pipelines_controller_pipelinerun_total{status="success"}[5m]))
/
sum(rate(tekton_pipelines_controller_pipelinerun_total[5m]))
```

**After:** ✅ No change needed (reason label not added to `*_total`)
```promql
sum(rate(tekton_pipelines_controller_pipelinerun_total{status="success"}[5m]))
/
sum(rate(tekton_pipelines_controller_pipelinerun_total[5m]))
```

### Example 2: P95 PipelineRun Duration

**Before:**
```promql
histogram_quantile(0.95,
  rate(tekton_pipelines_controller_pipelinerun_duration_seconds_bucket[5m])
)
```

**After:** ✅ No change needed (with default configuration)
```promql
histogram_quantile(0.95,
  rate(tekton_pipelines_controller_pipelinerun_duration_seconds_bucket[5m])
)
```

**Note:** If you enable `metrics.count.enable-reason: "true"`, you'll need to aggregate by `le`:
```promql
histogram_quantile(0.95,
  sum by(le) (
    rate(tekton_pipelines_controller_pipelinerun_duration_seconds_bucket[5m])
  )
)

# OR keep reason for granular failure analysis:
histogram_quantile(0.95,
  sum by(le, reason) (
    rate(tekton_pipelines_controller_pipelinerun_duration_seconds_bucket[5m])
  )
)
```

### Example 3: Controller Memory

**Before:**
```promql
tekton_pipelines_controller_go_memstats_alloc_bytes
```

**After:** ✅ Simple prefix change
```promql
go_memstats_alloc_bytes
```

### Example 4: Workqueue Processing Rate

**Before:**
```promql
rate(tekton_pipelines_controller_workqueue_adds_total{name="pipelinerun"}[5m])
```

**After:** ✅ Prefix change only
```promql
rate(kn_workqueue_adds_total{name="pipelinerun"}[5m])
```

---

## 7. Troubleshooting

### 7.1 Verifying Metrics Export

```bash
# Check metrics endpoint
kubectl port-forward -n tekton-pipelines deployment/tekton-pipelines-controller 9090:9090

# Verify new metrics exist
curl http://localhost:9090/metrics | grep kn_workqueue
curl http://localhost:9090/metrics | grep tekton_pipelines_controller
```

### 7.2 Prometheus Not Scraping

**Check Prometheus targets:**
1. Navigate to Prometheus UI → Status → Targets
2. Look for `tekton-pipelines-controller`
3. Verify State is `UP`

**Verify ServiceMonitor:**
```bash
kubectl get servicemonitor -n tekton-pipelines -o yaml
```

### 7.3 High Cardinality Warnings

**Problem:** Prometheus warns about high series cardinality after enabling `metrics.count.enable-reason`

**Root Cause:** The `reason` label increases time series by 3-5x

**Solution 1:** Disable the feature (recommended):
```yaml
data:
  metrics.count.enable-reason: "false"
```

**Solution 2:** Aggregate away the `reason` label in queries:
```promql
# Instead of:
histogram_quantile(0.95, rate(tekton_pipelines_controller_pipelinerun_duration_seconds_bucket[5m]))

# Use:
histogram_quantile(0.95, sum by(le) (rate(tekton_pipelines_controller_pipelinerun_duration_seconds_bucket[5m])))
```

**Note:** This issue only occurs if you explicitly enabled `metrics.count.enable-reason: "true"`.

### 7.4 Configuration Not Taking Effect

```bash
# Restart controller to pick up ConfigMap changes
kubectl rollout restart deployment/tekton-pipelines-controller -n tekton-pipelines
kubectl rollout status deployment/tekton-pipelines-controller -n tekton-pipelines

# Check logs
kubectl logs -n tekton-pipelines deployment/tekton-pipelines-controller | grep -i "observability"
```

### 7.5 OTLP Export Failures

**Check controller logs:**
```bash
kubectl logs -n tekton-pipelines deployment/tekton-pipelines-controller | grep -i "otel\|export\|metric"
```

**Test endpoint connectivity:**
```bash
kubectl exec -n tekton-pipelines deployment/tekton-pipelines-controller -- \
  nc -zv otel-collector.observability.svc.cluster.local 4317
```

### 7.6 Enable Debug Logging

```bash
kubectl edit configmap config-logging -n tekton-pipelines
# Add: loglevel.controller: "debug"

kubectl rollout restart deployment/tekton-pipelines-controller -n tekton-pipelines
```

---

## 8. Frequently Asked Questions

**Q: Do I need to upgrade immediately?**

A: This migration is included starting from a future Tekton Pipelines release. Plan to migrate when upgrading to that version. Test in staging first.

**Q: Will old metrics continue to work during transition?**

A: No. This is a hard cutover. Only OpenTelemetry metrics will be available after upgrade.

**Q: What if I don't update my configuration?**

A: If using Prometheus, the default behavior is preserved, but dashboards/alerts will break due to metric name changes.

**Q: Can I use both OpenCensus and OpenTelemetry?**

A: No. The controller only emits OpenTelemetry metrics after upgrade.

**Q: How do I test without affecting production?**

A: Deploy in a test/staging environment first. Verify all metrics, dashboards, and alerts work before upgrading production.

**Q: Where can I get help?**

A: File an issue at https://github.com/tektoncd/pipeline/issues or ask in Slack (#tekton channel).

---

## 9. Quick Reference Checklist

### Infrastructure Metrics Update

| Category | Old Prefix | New Prefix | Action |
|----------|-----------|-----------|---------|
| Workqueue | `tekton_pipelines_controller_workqueue_*` | `kn_workqueue_*` | Update all queries |
| K8s Client | `tekton_pipelines_controller_client_*` | `http_client_*` or `kn_k8s_client_*` | Update all queries |
| Go Runtime | `tekton_pipelines_controller_go_*` | `go_*` | Update all queries |

### Core Metrics Update

- [ ] ✅ **No changes needed** - Core metrics are backward compatible
- [ ] If you enable `metrics.count.enable-reason`, add `sum by(le)` aggregation to duration queries

### Configuration Update

- [ ] Change `metrics.backend-destination` → `metrics-protocol`
- [ ] Add OTLP endpoint if using gRPC/HTTP protocols
- [ ] Configure tracing if desired (new capability)

### Dashboard Update

- [ ] Replace workqueue metric names
- [ ] Replace Go runtime metric names
- [ ] Replace K8s client metric names
- [ ] **No changes needed** for core Tekton metrics (PipelineRun/TaskRun)
- [ ] Test all panels

### Alert Update

- [ ] Update workqueue metrics in alert rules
- [ ] Update Go runtime metrics in alert rules
- [ ] Update K8s client metrics if used
- [ ] Verify thresholds still appropriate
- [ ] Test alerts fire correctly

---

## Additional Resources

* **Tekton PR**: https://github.com/tektoncd/pipeline/pull/9043
* **OpenTelemetry Documentation**: https://opentelemetry.io/docs/
* **Tekton Metrics Documentation**: https://tekton.dev/docs/pipelines/metrics/
* **Tekton Slack**: https://kubernetes.slack.com - #tekton channel
* **GitHub Issues**: https://github.com/tektoncd/pipeline/issues

---

*For questions or clarifications, please refer to PR #9043 or contact the Tekton team.*
