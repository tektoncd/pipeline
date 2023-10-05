<!--
---
linkTitle: "Pipeline Metrics"
weight: 304
---
-->

# Pipeline Controller Metrics

The following pipeline metrics are available at `controller-service` on port `9090`.

We expose several kinds of exporters, including Prometheus, Google Stackdriver, and many others. You can set them up using [observability configuration](../config/config-observability.yaml).

| Name                                                                                    | Type | Labels/Tags | Status |
|-----------------------------------------------------------------------------------------| ----------- | ----------- | ----------- |
| `tekton_pipelines_controller_pipelinerun_duration_seconds_[bucket, sum, count]`         | Histogram/LastValue(Gauge) | `*pipeline`=&lt;pipeline_name&gt; <br> `*pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `namespace`=&lt;pipelinerun-namespace&gt; | experimental |
| `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds_[bucket, sum, count]` | Histogram/LastValue(Gauge) | `*pipeline`=&lt;pipeline_name&gt; <br> `*pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `*task`=&lt;task_name&gt; <br> `*taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt;| experimental |
| `tekton_pipelines_controller_pipelinerun_count`                                         | Counter | `status`=&lt;status&gt; <br> `*reason`=&lt;reason&gt; | experimental |
| `tekton_pipelines_controller_running_pipelineruns_count`                                | Gauge | | experimental |
| `tekton_pipelines_controller_taskrun_duration_seconds_[bucket, sum, count]`             | Histogram/LastValue(Gauge) | `status`=&lt;status&gt; <br> `*task`=&lt;task_name&gt; <br> `*taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt; | experimental |
| `tekton_pipelines_controller_taskrun_count`                                             | Counter | `status`=&lt;status&gt; <br> `*reason`=&lt;reason&gt; | experimental |
| `tekton_pipelines_controller_running_taskruns_count`                                    | Gauge | | experimental |
| `tekton_pipelines_controller_running_taskruns_throttled_by_quota_count`                 | Gauge | | experimental |
| `tekton_pipelines_controller_running_taskruns_throttled_by_node_count`                  | Gauge | | experimental |
| `tekton_pipelines_controller_running_taskruns_waiting_on_task_resolution_count`         | Gauge | | experimental |
| `tekton_pipelines_controller_running_pipelineruns_waiting_on_pipeline_resolution_count` | Gauge | | experimental |
| `tekton_pipelines_controller_running_pipelineruns_waiting_on_task_resolution_count`     | Gauge | | experimental |
| `tekton_pipelines_controller_taskruns_pod_latency_milliseconds`                         | Gauge | `namespace`=&lt;taskruns-namespace&gt; <br> `pod`= &lt; taskrun_pod_name&gt; <br> `*task`=&lt;task_name&gt; <br> `*taskrun`=&lt;taskrun_name&gt;<br> | experimental |
| `tekton_pipelines_controller_client_latency_[bucket, sum, count]`                       | Histogram | | experimental |

The Labels/Tag marked as "*" are optional. And there's a choice between Histogram and LastValue(Gauge) for pipelinerun and taskrun duration metrics.


## Configuring Metrics using `config-observability` configmap

A sample config-map has been provided as [config-observability](./../config/config-observability.yaml). By default, taskrun and pipelinerun metrics have these values:

``` yaml
    metrics.taskrun.level: "task"
    metrics.taskrun.duration-type: "histogram"
    metrics.pipelinerun.level: "pipeline"
    metrics.pipelinerun.duration-type: "histogram"
    metrics.count.enable-reason: "false"
```

Following values are available in the configmap:

| configmap data | value | description |
| ---------- | ----------- | ----------- |
| metrics.taskrun.level | `taskrun` | Level of metrics is taskrun |
| metrics.taskrun.level | `task` | Level of metrics is task and taskrun label isn't present in the metrics |
| metrics.taskrun.level | `namespace` | Level of metrics is namespace, and task and taskrun label isn't present in the metrics
| metrics.pipelinerun.level | `pipelinerun` | Level of metrics is pipelinerun |
| metrics.pipelinerun.level | `pipeline` | Level of metrics is pipeline and pipelinerun label isn't present in the metrics |
| metrics.pipelinerun.level | `namespace` | Level of metrics is namespace, pipeline and pipelinerun label isn't present in the metrics |
| metrics.taskrun.duration-type | `histogram` | `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds` and `tekton_pipelines_controller_taskrun_duration_seconds` is of type histogram |
| metrics.taskrun.duration-type | `lastvalue` | `tekton_pipelines_controller_pipelinerun_taskrun_duration_seconds` and  `tekton_pipelines_controller_taskrun_duration_seconds` is of type gauge or lastvalue |
| metrics.pipelinerun.duration-type | `histogram` | `tekton_pipelines_controller_pipelinerun_duration_seconds` is of type histogram |
| metrics.pipelinerun.duration-type | `lastvalue` | `tekton_pipelines_controller_pipelinerun_duration_seconds` is of type gauge or lastvalue |
| metrics.count.enable-reason | `false` | Sets if the `reason` label should be included on count metrics |

Histogram value isn't available when pipelinerun or taskrun labels are selected. The Lastvalue or Gauge will be provided. Histogram would serve no purpose because it would generate a single bar. TaskRun and PipelineRun level metrics aren't recommended because they lead to an unbounded cardinality which degrades the observability database.

To check that appropriate values have been applied in response to configmap changes, use the following commands:
```shell
kubectl port-forward -n tekton-pipelines service/tekton-pipelines-controller 9090
```

And then check that changes have been applied to metrics coming from [http://127.0.0.1:9090/metrics](http://127.0.0.1:9090/metrics)
