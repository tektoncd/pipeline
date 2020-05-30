<!--
---
linkTitle: "Pipeline Metrics"
weight: 14
---
-->
# Pipeline Controller Metrics

The following pipeline metrics are available at `controller-service` on port `9090`.

We expose several kinds of exporters, including Prometheus, Google Stackdriver, and many others. You can set them up using [observability configuration](../config/config-observability.yaml).

|  Name | Type | Labels/Tags | Status |
| ---------- | ----------- | ----------- | ----------- |
| `tekton_pipelinerun_duration_seconds_[bucket, sum, count]` | Histogram | `pipeline`=&lt;pipeline_name&gt; <br> `pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `namespace`=&lt;pipelinerun-namespace&gt; | experimental |
| `tekton_pipelinerun_taskrun_duration_seconds_[bucket, sum, count]` | Histogram | `pipeline`=&lt;pipeline_name&gt; <br> `pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt;| experimental |
| `tekton_pipelinerun_count` | Counter | `status`=&lt;status&gt; | experimental |
| `tekton_running_pipelineruns_count` | Gauge | | experimental |
| `tekton_taskrun_duration_seconds_[bucket, sum, count]` | Histogram | `status`=&lt;status&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt; | experimental |
| `tekton_taskrun_count` | Counter | `status`=&lt;status&gt; | experimental |
| `tekton_running_taskruns_count` | Gauge | | experimental |
| `tekton_taskruns_pod_latency` | Gauge | `namespace`=&lt;taskruns-namespace&gt; <br> `pod`= &lt; taskrun_pod_name&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> | experimental |
| `tekton_taskruns_pod_latency` | Gauge | `namespace`=&lt;taskruns-namespace&gt; <br> `pod`= &lt; taskrun_pod_name&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> | experimental |
| `tekton_cloudevent_count` | Counter | `pipeline`=&lt;pipeline_name&gt; <br> `pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt;| experimental |
