<!--
---
linkTitle: "Tekton Controller Performance Configuration"
weight: 105
---
-->

# Tekton Controller Performance Configuration
Configure ThreadsPerController, QPS and Burst

- [Overview](#overview)
- [Performance Configuration](#performance-configuration)
  - [Configure Thread, QPS and Burst](#configure-thread-qps-and-burst)
- [Informer Cache Transform (Memory Optimization)](#informer-cache-transform-memory-optimization)
  - [What Gets Stripped](#what-gets-stripped)
  - [Memory Savings](#memory-savings)
  - [Important Notes](#important-notes)

## Overview

---
This document will show us how to configure [tekton-pipeline-controller](./../config/controller.yaml)'s performance. In general, there are mainly have three parameters will impact the performance of tekton controller, they are `ThreadsPerController`, `QPS` and `Burst`.

- `ThreadsPerController`: Threads (goroutines) to create per controller. It's the number of threads to use when processing the controller's work queue.
<!-- wokeignore:rule=master -->
- `QPS`: Queries per Second. Maximum QPS to the master from this client.

- `Burst`: Maximum burst for throttle.

## Performance Configuration

---
#### Configure Thread, QPS and Burst

---
Default, the value of ThreadsPerController, QPS and Burst is [2](https://github.com/knative/pkg/blob/main/controller/controller.go#L58), [5.0](https://github.com/tektoncd/pipeline/blob/main/vendor/k8s.io/client-go/rest/config.go#L44) and [10](https://github.com/tektoncd/pipeline/blob/main/vendor/k8s.io/client-go/rest/config.go#L45) accordingly.

Sometimes, above default values can't meet performance requirements, then you need to overwrite these values. You can modify them in the [tekton controller deployment](./../config/controller.yaml). You can specify these customized values in the `tekton-pipelines-controller` container via `threads-per-controller`, `kube-api-qps` and `kube-api-burst` flags accordingly. For example:

```yaml
spec:
  serviceAccountName: tekton-pipelines-controller
  containers:
    - name: tekton-pipelines-controller
      image: ko://github.com/tektoncd/pipeline/cmd/controller
      args: [
          "-kube-api-qps", "50",
          "-kube-api-burst", "50",
          "-threads-per-controller", "32",
          # other flags defined here...
        ]
```

Now, the ThreadsPerController, QPS and Burst have been changed to be `32`, `50` and `50`.

You can also set `THREADS_PER_CONTROLLER`, `KUBE_API_QPS` and `KUBE_API_BURST` environment variables to overwrite default values.
If flags and environment variables are set, command line args will take precedence.

**Note**:
<!-- wokeignore:rule=master -->
Although in above example, you set QPS and Burst to be `50` and `50`. However, the actual values of them are [multiplied by `2`](https://github.com/pierretasci/pipeline/blob/master/cmd/controller/main.go#L83-L84), so the actual QPS and Burst is `100` and `100`.

## Informer Cache Transform (Memory Optimization)

---

The Tekton controller uses informer caches to watch and react to changes in Kubernetes objects. By default, these caches store the complete object, including fields that are not needed for reconciliation. For clusters with large numbers of PipelineRuns, TaskRuns, and Pods, this can result in significant memory usage.

To reduce memory usage, Tekton Pipeline applies **cache transform functions** that strip unnecessary fields from objects before they are stored in the informer cache. This optimization is enabled by default and requires no configuration.

### What Gets Stripped

The transform functions remove the following fields:

#### All Objects (PipelineRuns, TaskRuns, CustomRuns, Pods)

- `metadata.managedFields` - Server-side apply metadata (can be 25-30% of object size)
- `kubectl.kubernetes.io/last-applied-configuration` annotation - Client-side apply metadata

#### Completed PipelineRuns

For PipelineRuns that have finished (succeeded, failed, or cancelled):
- `status.pipelineSpec` - The resolved Pipeline specification
- `status.provenance` - Build provenance metadata
- `status.spanContext` - Tracing context

#### Completed TaskRuns

For TaskRuns that have finished:
- `status.taskSpec` - The resolved Task specification
- `status.provenance` - Build provenance metadata
- `status.spanContext` - Tracing context
- `status.steps` - Detailed step execution state
- `status.sidecars` - Detailed sidecar execution state

**Note:** The following fields are intentionally **preserved** for completed TaskRuns:
- `status.podName` - Required for debugging and referenced by retry status entries
- `status.retriesStatus` - Required for retry tracking; the controller appends to this slice during retries
- `status.completionTime` - Part of the observable API and used for metrics/debugging

#### Completed CustomRuns

For CustomRuns that have finished:
- `status.retriesStatus` - Retry history
- `status.completionTime` - Completion timestamp

#### Pods

Pod objects are stripped more aggressively since the TaskRun controller only needs:
- `metadata.name`, `metadata.namespace`, `metadata.labels`, `metadata.ownerReferences`, `metadata.annotations`
- `status` (fully preserved)
- `spec.containers[].name` (for status sorting)

All other Pod spec fields are stripped.

### Memory Savings

Benchmark tests show approximately **78% memory reduction** for completed PipelineRuns with realistic configurations. The actual savings depend on the size and complexity of your Pipelines and Tasks.

### Important Notes

1. **Read-only optimization**: The transform only affects what is stored in the in-memory cache. The complete objects remain unchanged in etcd.

2. **No impact on API access**: When you use `kubectl get` or the Kubernetes API, you always get the complete object from etcd, not the cached version.

3. **Controller behavior unchanged**: The controller reconciliation logic continues to work correctly because it only needs the preserved fields for completed objects.

4. **Enabled by default**: This optimization is always active and requires no configuration.
