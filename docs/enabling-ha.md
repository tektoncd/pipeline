# HA Support for Tekton Pipeline Controllers

- [Overview](#overview)
- [Configuring HA](#configuring-ha)
  - [Configuring the controller replicas](#configuring-the-controller-replicas)
  - [Configuring the leader election](#configuring-the-leader-election)
- [Disabling leader election](#disabling-leader-election)
  - [Use the disable-ha flag](#use-the-disable-ha-flag)
  - [Scale down your replicas](#scale-down-your-replicas)

## Overview

---
This document is aimed at helping Cluster Admins when configuring High Availability(HA) support for the Tekton Pipeline [controller deployment](../../config/controller.yaml).

HA support allows components to remain operational when a disruption occurs. This is achieved by following an active/active model, where all replicas of the Tekton controller can receive workload. In this HA approach the reconcile space is distributed across buckets, where each replica owns a subset of those buckets and can process the load if the given replica is the leader of that bucket.

By default HA is enabled in the Tekton pipelines controller.

## Configuring HA

---
In order to achieve HA, the number of replicas for the Tekton Pipeline controller should be greater than one. This allows other instance(_s_) to take over in case of any disruption on the current active controller.

### Configuring the controller replicas

You can modify the replicas number in the [controller deployment](../../config/controller.yaml) under `spec.replicas` or apply an update to a running deployment:

```sh
kubectl -n tekton-pipelines scale deployment tekton-pipelines-controller --replicas=3
```

### Configuring the Leader Election

The leader election can be configured via the [config-leader-election.yaml](../../config/config-leader-election.yaml). The configmap defines the following parameters:

| Parameter            | Default  |
| -------------------- | -------- |
| `data.buckets`       | 1        |
| `data.leaseDuration` | 15s      |
| `data.renewDeadline` | 10s      |
| `data.retryPeriod`   | 2s       |

_Note_: When setting `data.buckets`, the underlying Knative library only allows a value between 1 and 10, making 10 the maximum number of allowed buckets.

## Disabling leader election

---

If HA is not required and running a single instance of the Tekton Pipeline controller is enough, there are two alternatives:

### Use the disable-ha flag

You can modify the [controller deployment](../../config/controller.yaml), by specifying in the `tekton-pipelines-controller` container the `disable-ha` flag. For example:

```yaml
spec:
  serviceAccountName: tekton-pipelines-controller
  containers:
  - name: tekton-pipelines-controller
    ...
    args: [
      # Other flags defined here...
      "-disable-ha=true"
    ]
```

by setting `disable-ha` to `true`, HA will be disable in the controllers.

**Note**: Please consider that when disabling HA and keeping multiple replicas of the [controller deployment](../../config/controller.yaml), each replica will act as an independent controller, leading to an unwanted behaviour when creating resources(e.g. `TaskRuns`, etc).

### Scale down your replicas

Although HA is enable by default, if your [controller deployment](../../config/controller.yaml) replicas are set to one, there would be no High Availability in the scenario where the running instance is deleted or fails. Therefore having a single replica even with HA enable, does not ensure high availability for the controller.
