<!--
---
linkTitle: "Pipeline Metrics"
weight: 304
---
-->

# Pipeline Controller Metrics

The following pipeline metrics are available at `controller-service` on port `9090`.

Metrics are exported using [OpenTelemetry](https://opentelemetry.io/) and
can be configured via the
[observability ConfigMap](../config/config-observability.yaml). By default,
Prometheus export is enabled. OTLP (gRPC and HTTP) export is also available
for sending metrics to an OpenTelemetry Collector or compatible backend.

## Core Tekton Metrics

| Name | Type | Labels/Tags | Status |
|---|---|---|---|
| `tekton_pipelines_controller_pipelinerun_duration_seconds_[bucket, sum, count]` | Histogram/LastValue(Gauge) | `*pipeline`=&lt;pipeline_name&gt; <br> `*pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `namespace`=&lt;pipelinerun-namespace&gt; <br> `*reason`=&lt;reason&gt; | experimental |
| `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds_[bucket, sum, count]` | Histogram/LastValue(Gauge) | `*pipeline`=&lt;pipeline_name&gt; <br> `*pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `*task`=&lt;task_name&gt; <br> `*taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt; <br> `*reason`=&lt;reason&gt; | experimental |
| `tekton_pipelines_controller_pipelinerun_total` | Counter | `status`=&lt;status&gt; | experimental |
| `tekton_pipelines_controller_running_pipelineruns` | Gauge | | experimental |
| `tekton_pipelines_controller_taskrun_duration_seconds_[bucket, sum, count]` | Histogram/LastValue(Gauge) | `status`=&lt;status&gt; <br> `*task`=&lt;task_name&gt; <br> `*taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt; <br> `*reason`=&lt;reason&gt; | experimental |
| `tekton_pipelines_controller_taskrun_total` | Counter | `status`=&lt;status&gt; | experimental |
| `tekton_pipelines_controller_running_taskruns` | Gauge | | experimental |
| `tekton_pipelines_controller_running_taskruns_throttled_by_quota` | Gauge | `namespace`=&lt;pipelinerun-namespace&gt; | experimental |
| `tekton_pipelines_controller_running_taskruns_throttled_by_node` | Gauge | `namespace`=&lt;pipelinerun-namespace&gt; | experimental |
| `tekton_pipelines_controller_running_pipelineruns_waiting_on_pipeline_resolution` | Gauge | | experimental |
| `tekton_pipelines_controller_running_pipelineruns_waiting_on_task_resolution` | Gauge | | experimental |
| `tekton_pipelines_controller_running_taskruns_waiting_on_task_resolution_count` | Gauge | | experimental |
| `tekton_pipelines_controller_taskruns_pod_latency_milliseconds` | Histogram | `namespace`=&lt;namespace&gt; `*task`=&lt;task_name&gt; `*taskrun`=&lt;taskrun_name&gt; (unbounded cardinality, see [#9393](https://github.com/tektoncd/pipeline/issues/9393)) | experimental |

The Labels/Tags marked as "\*" are optional. There is a choice between Histogram and LastValue(Gauge) for pipelinerun and taskrun duration metrics.

> **Note:** All metrics now carry an `otel_scope_name` label identifying the
> instrumentation package. This label is informational and transparent to
> most PromQL queries.

## Infrastructure Metrics

These metrics are provided by the Knative and Go runtime infrastructure.
Their names changed as part of the OpenCensus to OpenTelemetry migration
(see [migration guide](metrics-migration-otel.md) for full details).

| Name | Type | Description |
|---|---|---|
| `kn_workqueue_adds_total` | Counter | Workqueue additions |
| `kn_workqueue_depth` | Gauge | Current workqueue depth |
| `kn_workqueue_queue_duration_seconds` | Histogram | Time items spend in queue |
| `kn_workqueue_process_duration_seconds` | Histogram | Time to process items |
| `kn_workqueue_retries_total` | Counter | Workqueue retries |
| `kn_workqueue_unfinished_work_seconds` | Gauge | Seconds of work in progress |
| `http_client_request_duration_seconds` | Histogram | K8s API client request duration |
| `kn_k8s_client_http_response_status_code_total` | Counter | K8s API response status codes |
| `go_goroutines` | Gauge | Number of goroutines |
| `go_memstats_alloc_bytes` | Gauge | Bytes allocated and still in use |

## Configuring Metrics using `config-observability` ConfigMap

A sample ConfigMap has been provided as [config-observability](./../config/config-observability.yaml).

### Metrics and tracing protocol

The `metrics-protocol` key controls how metrics are exported:

| Value | Description |
|---|---|
| `prometheus` | Starts an HTTP server on port 9090 serving `/metrics` (default) |
| `grpc` | Exports via OTLP gRPC to the configured `metrics-endpoint` |
| `http/protobuf` | Exports via OTLP HTTP to the configured `metrics-endpoint` |
| `none` | Disables metrics export |

The `tracing-protocol` key controls distributed tracing:

| Value | Description |
|---|---|
| `none` | Disables tracing (default) |
| `grpc` | Exports traces via OTLP gRPC to `tracing-endpoint` |
| `http/protobuf` | Exports traces via OTLP HTTP to `tracing-endpoint` |
| `stdout` | Prints traces to stdout (for debugging) |

> **Note:** The previous OpenCensus configuration keys
> (`metrics.backend-destination`, `metrics.stackdriver-project-id`, etc.)
> are no longer supported. See the
> [migration guide](metrics-migration-otel.md) for details on upgrading.

### Tekton-specific metrics settings

By default, taskrun and pipelinerun metrics have these values:

``` yaml
    metrics.taskrun.level: "task"
    metrics.taskrun.duration-type: "histogram"
    metrics.pipelinerun.level: "pipeline"
    metrics.running-pipelinerun.level: ""
    metrics.pipelinerun.duration-type: "histogram"
    metrics.count.enable-reason: "false"
```

Following values are available in the ConfigMap:

| ConfigMap data | value | description |
|---|---|---|
| metrics.taskrun.level | `taskrun` | Level of metrics is taskrun |
| metrics.taskrun.level | `task` | Level of metrics is task and taskrun label isn't present in the metrics |
| metrics.taskrun.level | `namespace` | Level of metrics is namespace, and task and taskrun label isn't present in the metrics |
| metrics.pipelinerun.level | `pipelinerun` | Level of metrics is pipelinerun |
| metrics.pipelinerun.level | `pipeline` | Level of metrics is pipeline and pipelinerun label isn't present in the metrics |
| metrics.pipelinerun.level | `namespace` | Level of metrics is namespace, pipeline and pipelinerun label isn't present in the metrics |
| metrics.running-pipelinerun.level | `pipelinerun` | Level of running-pipelinerun metrics is pipelinerun |
| metrics.running-pipelinerun.level | `pipeline` | Level of running-pipelinerun metrics is pipeline and pipelinerun label isn't present in the metrics |
| metrics.running-pipelinerun.level | `namespace` | Level of running-pipelinerun metrics is namespace, pipeline and pipelinerun label isn't present in the metrics |
| metrics.running-pipelinerun.level | `` | Level of running-pipelinerun metrics is cluster, namespace, pipeline and pipelinerun label isn't present in the metrics. |
| metrics.taskrun.duration-type | `histogram` | `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds` and `tekton_pipelines_controller_taskrun_duration_seconds` is of type histogram |
| metrics.taskrun.duration-type | `lastvalue` | `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds` and `tekton_pipelines_controller_taskrun_duration_seconds` is of type gauge or lastvalue |
| metrics.pipelinerun.duration-type | `histogram` | `tekton_pipelines_controller_pipelinerun_duration_seconds` is of type histogram |
| metrics.pipelinerun.duration-type | `lastvalue` | `tekton_pipelines_controller_pipelinerun_duration_seconds` is of type gauge or lastvalue |
| metrics.count.enable-reason | `false` | Sets if the `reason` label should be included on duration metrics (`*_duration_seconds`); never affects total counters (`*_total`) |
| metrics.taskrun.throttle.enable-namespace | `false` | Sets if the `namespace` label should be included on the `tekton_pipelines_controller_running_taskruns_throttled_by_quota` metric |

Histogram value isn't available when pipelinerun or taskrun labels are selected. The Lastvalue or Gauge will be provided. Histogram would serve no purpose because it would generate a single bar. TaskRun and PipelineRun level metrics aren't recommended because they lead to an unbounded cardinality which degrades the observability database.

### Verifying metrics

```shell
kubectl port-forward -n tekton-pipelines service/tekton-pipelines-controller 9090
```

Then check that changes have been applied to metrics coming from [http://127.0.0.1:9090/metrics](http://127.0.0.1:9090/metrics)
