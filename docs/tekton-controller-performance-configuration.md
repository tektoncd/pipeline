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

## Overview

---
This document will show us how to configure [tekton-pipeline-controller](./../config/controller.yaml)'s performance. In general, there are mainly have three parameters will impact the performance of tekton controller, they are `ThreadsPerController`, `QPS` and `Burst`.

- `ThreadsPerController`: Threads (goroutines) to create per controller. It's the number of threads to use when processing the controller's work queue.

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

**Note**:
Although in above example, you set QPS and Burst to be `50` and `50`. However, the actual values of them are [multiplied by `2`](https://github.com/pierretasci/pipeline/blob/master/cmd/controller/main.go#L83-L84), so the actual QPS and Burst is `100` and `100`.
