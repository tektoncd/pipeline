<!--
---
linkTitle: "Pipeline Metrics"
weight: 1200
---
-->
# Pipeline Controller Metrics

Following pipeline metrics are exposed at `controller-service` on port `9090`

There are different kinds of exporters available: Prometheus, Google Stackdriver, etc. They can be configured 
using [observability configuration](../config/config-observability.yaml). 

| Metric name| Metric type | Labels/tags | Status |
| ---------- | ----------- | ----------- | ----------- |
| tekton_pipelinerun_duration_seconds_[bucket, sum, count] | Histogram | `pipeline`=&lt;pipeline_name&gt; <br> `pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `namespace`=&lt;pipelinerun-namespace&gt; | experimental |
| tekton_pipelinerun_taskrun_duration_seconds_[bucket, sum, count] | Histogram | `pipeline`=&lt;pipeline_name&gt; <br> `pipelinerun`=&lt;pipelinerun_name&gt; <br> `status`=&lt;status&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt;| experimental |
| tekton_pipelinerun_count| Counter | `status`=&lt;status&gt; | experimental |
| tekton_running_pipelineruns_count | Gauge | | experimental | 
| tekton_taskrun_duration_seconds_[bucket, sum, count] | Histogram | `status`=&lt;status&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> `namespace`=&lt;pipelineruns-taskruns-namespace&gt; | experimental | 
| tekton_taskrun_count | Counter | `status`=&lt;status&gt; | experimental | 
| tekton_running_taskruns_count | Gauge | | experimental |
| tekton_taskruns_pod_latency | Gauge | `namespace`=&lt;taskruns-namespace&gt; <br> `pod`= &lt; taskrun_pod_name&gt; <br> `task`=&lt;task_name&gt; <br> `taskrun`=&lt;taskrun_name&gt;<br> | experimental |
