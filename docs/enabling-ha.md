<!--
---
linkTitle: "High Availability Support"
weight: 106
---
-->

# HA Support for Tekton Pipeline Controllers

  - [Overview](#overview)
  - [Controller HA](#controller-ha)
    - [Configuring Controller Replicas](#configuring-controller-replicas)
    - [Configuring Leader Election](#configuring-leader-election)
    - [Disabling Controller HA](#disabling-controller-ha)
  - [Webhook HA](#webhook-ha)
    - [Configuring Webhook Replicas](#configuring-webhook-replicas)
    - [Avoiding Disruptions](#avoiding-disruptions)

## Overview

This document is aimed at helping Cluster Admins when configuring High Availability (HA) support for the Tekton Pipeline [Controller](./../config/controller.yaml) and [Webhook](./../config/webhook.yaml) components. HA support allows components to remain operational when a disruption occurs, such as nodes being drained for upgrades.

## Controller HA

For the Controller, HA is achieved by following an active/active model, where all replicas of the Controller can receive and process work items. In this HA approach the workqueue is distributed across buckets, where each replica owns a subset of those buckets and can process the load if the given replica is the leader of that bucket.

By default, only one Controller replica is configured, to reduce resource usage. This effectively disables HA for the Controller by default.

### Configuring Controller Replicas

In order to achieve HA for the Controller, the number of replicas for the Controller should be greater than one. This allows other instances to take over in case of any disruption on the current active controller.

You can modify the replicas number in the [Controller deployment](./../config/controller.yaml) under `spec.replicas`, or apply an update to a running deployment:

```sh
kubectl -n tekton-pipelines scale deployment tekton-pipelines-controller --replicas=3
```

### Configuring Leader Election

Leader election can be configured in [config-leader-election.yaml](./../config/config-leader-election.yaml). The ConfigMap defines the following parameters:

| Parameter            | Default  |
| -------------------- | -------- |
| `data.buckets`       | 1        |
| `data.leaseDuration` | 15s      |
| `data.renewDeadline` | 10s      |
| `data.retryPeriod`   | 2s       |

_Note_: The maximum value of `data.buckets` at this time is 10.

### Disabling Controller HA

If HA is not required, you can disable it by scaling the deployment back to one replica. You can also modify the [controller deployment](./../config/controller.yaml), by specifying in the `tekton-pipelines-controller` container the `disable-ha` flag. For example:

```yaml
spec:
  serviceAccountName: tekton-pipelines-controller
  containers:
    - name: tekton-pipelines-controller
      # ...
      args: [
          # Other flags defined here...
          "-disable-ha=true",
        ]
```

**Note:** If you set `-disable-ha=false` and run multiple replicas of the Controller, each replica will process work items separately, which will lead to unwanted behavior when creating resources (e.g., `TaskRuns`, etc.).

In general, setting `-disable-ha=false` is not recommended. Instead, to disable HA, simply run one replica of the Controller deployment.

## Webhook HA

The Webhook deployment is stateless, which means it can more easily be configured for HA, and even autoscale replicas in response to load.

By default, only one Webhook replica is configured, to reduce resource usage. This effectively disables HA for the Webhook by default.

### Configuring Webhook Replicas

In order to achieve HA for the Webhook deployment, you can modify the `replicas` number in the [Webhook deployment](./../config/webhook.yaml) under `spec.replicas`, or apply an update to a running deployment:

```sh
kubectl -n tekton-pipelines scale deployment tekton-pipelines-webhook --replicas=3
```

You can also modify the [HorizontalPodAutoscaler](./../config/webhook-hpa.yaml) to set a minimum number of replicas:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tekton-pipelines-webhook
# ...
spec:
  minReplicas: 1
```

By default, the Webhook deployment is _not_ configured to block a [Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) from scaling down the node that's running the only replica of the deployment using the `cluster-autoscaler.kubernetes.io/safe-to-evict` annotation.
This means that during node drains, the Webhook might be unavailable temporarily, during which time Tekton resources can't be created, updated or deleted.
To avoid this, you can add the `safe-to-evict` annotation set to `false` to block node drains during autoscaling, or, better yet, configure multiple replicas of the Webhook deployment.

### Avoiding Disruptions

To avoid the Webhook Service becoming unavailable during node unavailability (e.g., during node upgrades), you can ensure that a minimum number of Webhook replicas are available at time by defining a [`PodDisruptionBudget`](https://kubernetes.io/docs/tasks/run-application/configure-pdb/) which sets a `minAvailable` greater than zero:

```yaml
apiVersion: policy/v1beta1
kind: PodDisruptionBudget
metadata:
  name: tekton-pipelines-webhook
  namespace: tekton-pipelines
  labels:
    app.kubernetes.io/name: webhook
    app.kubernetes.io/component: webhook
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
    # ...
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: webhook
      app.kubernetes.io/component: webhook
      app.kubernetes.io/instance: default
      app.kubernetes.io/part-of: tekton-pipelines
```

Webhook replicas are configured to avoid being scheduled onto the same node by default, so that a single node disruption doesn't make all Webhook replicas unavailable.
