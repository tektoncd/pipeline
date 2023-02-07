<!--
---
linkTitle: "Compute Resources Limits"
weight: 408
---
-->

# Compute Resources in Tekton

## Background: Resource Requirements in Kubernetes

Kubernetes allows users to specify CPU, memory, and ephemeral storage constraints
for [containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/).
Resource requests determine the resources reserved for a pod when it's scheduled,
and affect likelihood of pod eviction. Resource limits constrain the maximum amount of
a resource a container can use. A container that exceeds its memory limits will be killed,
and a container that exceeds its CPU limits will be throttled.

A pod's [effective resource requests and limits](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#resources)
are the higher of:
- the sum of all app containers request/limit for a resource
- the effective init container request/limit for a resource

This formula exists because Kubernetes runs init containers sequentially and app containers
in parallel. (There is no distinction made between app containers and sidecar containers
in Kubernetes; a sidecar is used in the following example to illustrate this.)

For example, consider a pod with the following containers:

| Container           | CPU request | CPU limit |
| ------------------- | ----------- | --------- |
| init container 1    | 1           | 2         |
| init container 2    | 2           | 3         |
| app container 1     | 1           | 2         |
| app container 2     | 2           | 3         |
| sidecar container 1 | 3           | no limit  |

The sum of all app container CPU requests is 6 (including the sidecar container), which is
greater than the maximum init container CPU request (2). Therefore, the pod's effective CPU
request will be 6.

Since the sidecar container has no CPU limit, this is treated as the highest CPU limit.
Therefore, the pod will have no effective CPU limit.

## Task-level Compute Resources Configuration

**([alpha only](https://github.com/tektoncd/pipeline/blob/main/docs/install.md#alpha-features))**

Tekton allows users to specify resource requirements of [`Steps`](./tasks.md#defining-steps),
which run sequentially. However, the pod's effective resource requirements are still the
sum of its containers' resource requirements. This means that when specifying resource 
requirements for `Step` containers, they must be treated as if they are running in parallel.

Tekton adjusts `Step` resource requirements to comply with [LimitRanges](#limitrange-support).
[ResourceQuotas](#resourcequota-support) are not currently supported.

Instead of specifying resource requirements on each `Step`, users can choose to specify resource requirements at the Task-level. If users specify a Task-level resource request, it will ensure that the kubelet reserves only that amount of resources to execute the `Task`'s `Steps`.
If users specify a Task-level resource limit, no `Step` may use more than that amount of resources.

Each of these details is explained in more depth below.

Some points to note:

- Task-level resource requests and limits do not apply to sidecars which can be configured separately.
- If only limits are configured in task-level, it will be applied as the task-level requests.
- Resource requirements configured in `Step` or `StepTemplate` of the referenced `Task` will be overridden by the task-level requirements.
- `TaskRun` configured with both `StepOverrides` and task-level requirements will be rejected.

### Configure Task-level Compute Resources

Task-level resource requirements can be configured in `TaskRun.ComputeResources`, or `PipelineRun.TaskRunSpecs.ComputeResources`.

e.g.

```yaml
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: foo 
spec:
  computeResources:
    requests:
      cpu: 1 
    limits:
      cpu: 2
```

The following TaskRun will be rejected, because it configures both stepOverrides and task-level compute resource requirements:

```yaml
kind: TaskRun
spec:
  stepOverrides:
    - name: foo
      resources:
        requests:
          cpu: 1
  computeResources:
    requests:
      cpu: 2 
```

```yaml
kind: PipelineRun
spec:
  taskRunSpecs:
    - pipelineTaskName: foo
      stepOverrides:
        - name: foo 
          resources:
            requests:
              cpu: 1 
      computeResources:
        requests:
          cpu: 2
```

### Configure Resource Requirements with Sidecar

Users can specify compute resources separately for a sidecar while configuring task-level resource requirements on TaskRun.

e.g.

```yaml
kind: TaskRun
spec:
  sidecarOverrides:
    - name: sidecar
      resources:
        requests:
          cpu: 750m
        limits:
          cpu: 1
  computeResources:
    requests:
      cpu: 2
```

## LimitRange Support

Kubernetes allows users to configure [LimitRanges]((https://kubernetes.io/docs/concepts/policy/limit-range/)),
which constrain compute resources of pods, containers, or PVCs running in the same namespace.

LimitRanges can:
- Enforce minimum and maximum compute resources usage per Pod or Container in a namespace.
- Enforce minimum and maximum storage request per PersistentVolumeClaim in a namespace.
- Enforce a ratio between request and limit for a resource in a namespace.
- Set default request/limit for compute resources in a namespace and automatically inject them to Containers at runtime.

Tekton applies the resource requirements specified by users directly to the containers
in a `Task's` pod, unless there is a LimitRange present in the namespace.
Tekton supports LimitRange minimum, maximum, and default resource requirements for containers,
but does not support LimitRange ratios between requests and limits ([#4230](https://github.com/tektoncd/pipeline/issues/4230)).
LimitRange types other than "Container" are not considered for purposes of resource requirements.

Tekton doesn't allow users to configure init containers for a `Task`, but any `default` and `defaultRequest` from a LimitRange
will be applied to the init containers that Tekton injects into a `TaskRun`'s pod.

### Requests

If a Step container does not have requests defined, Tekton will divide a LimitRange's `defaultRequests` by the number of Step containers and apply these requests to the Steps.
This results in a TaskRun with overall requests equal to LimitRange `defaultRequests`.
If this value is less than the LimitRange minimum, the LimitRange minimum will be used instead.
LimitRange `defaultRequests` are applied as-is to init containers or Sidecar containers that don't specify requests.

Containers that do specify requests will not be modified. If these requests are lower than LimitRange minimums, Kubernetes will reject the resulting TaskRun's pod.

### Limits

Tekton does not adjust container limits, regardless of whether a container is a Step, Sidecar, or init container.
If a container does not have limits defined, Kubernetes will apply the LimitRange `default` to the container's limits.
If a container does define limits, and they are less than the LimitRange `default`, Kubernetes will reject the resulting TaskRun's pod.

### Examples

Consider the following LimitRange:

```
apiVersion: v1
kind: LimitRange
metadata:
  name: limitrange-example
spec:
  limits:
  - default:  # The default limits
      cpu: 2
    defaultRequest:  # The default requests
      cpu: 1
    max:  # The maximum limits
      cpu: 3
    min:  # The minimum requests
      cpu: 300m
    type: Container
```

A `Task` with 2 `Steps` and no resources specified would result in a pod with the following containers:

| Container    | CPU request | CPU limit |
| ------------ | ----------- | --------- |
| container 1  | 500m        | 2         | 
| container 2  | 500m        | 2         |

Here, the default CPU request was divided among the step containers, and this value was used since it was greater
than the minimum request specified by the LimitRange.
The CPU limits are 2 for each container, as this is the default limit specifed in the LimitRange.

If we had a `Task` with 2 `Steps` and 1 `Sidecar` with no resources specified would result in a pod with the following containers:  

| Container    | CPU request | CPU limit |
| ------------ | ----------- | --------- |
| container 1  | 500m        | 2         |
| container 2  | 500m        | 2         |
| container 3  | 1           | 2         |

For the first two containers, the default CPU request was divided among the step containers, and this value was used since it was greater
than the minimum request specified by the LimitRange. The third container is a sidecar and since it is not a step container gets the full 
default CPU request of 1. AS before the CPU limits are 2 for each container, as this is the default limit specifed in the LimitRange.


Now, consider a `Task` with the following `Step`s:

| Step   | CPU request | CPU limit |
| ------ | ----------- | --------- |
| step 1 | 200m        | 2         |
| step 2 | 1           | 4         |

The resulting pod would have the following containers:

| Container    | CPU request | CPU limit |
| ------------ | ----------- | --------- |
| container 1  | 300m        | 2         |
| container 2  | 1           | 3         |

Here, the first `Step's` request was less than the LimitRange minimum, so the output request is the minimum (300m).
The second `Step's` request is unchanged. The first `Step's` limit is less than the maximum, so it is unchanged,
while the second `Step's` limit is greater than the maximum, so the maximum (3) is used.

### Support for multiple LimitRanges

Tekton supports running `TaskRuns` in namespaces with multiple LimitRanges.
For a given resource, the minumum used will be the largest of any of the LimitRanges' minimum values,
and the maximum used will be the smallest of any of the LimitRanges' maximum values.

The minimum resource requirement used will be the largest of any minimum for that resource,
and the maximum resource requirement will be the smallest of any of the maximum values defined.
The default value will be the minimum of any default values defined.
If the resulting default value is less than the resulting minimum value, the default value will be the minimum value.

It's possible for multiple LimitRanges to be defined which are not compatible with each other, preventing pods from being scheduled.

#### Example

Consider a namespaces with the following LimitRanges defined:

```
apiVersion: v1
kind: LimitRange
metadata:
  name: limitrange-1
spec:
  limits:
  - default:  # The default limits
      cpu: 2
    defaultRequest:  # The default requests
      cpu: 750m
    max:  # The maximum limits
      cpu: 3
    min:  # The minimum requests
      cpu: 500m
    type: Container
```

```
apiVersion: v1
kind: LimitRange
metadata:
  name: limitrange-2
spec:
  limits:
  - default:  # The default limits
      cpu: 1.5
    defaultRequest:  # The default requests
      cpu: 1
    max:  # The maximum limits
      cpu: 2.5
    min:  # The minimum requests
      cpu: 300m
    type: Container
```

A namespace with limitrange-1 and limitrange-2 would be treated as if it contained only the following LimitRange:

```
apiVersion: v1
kind: LimitRange
metadata:
  name: aggregate-limitrange
spec:
  limits:
  - default:  # The default limits
      cpu: 1.5
    defaultRequest:  # The default requests
      cpu: 750m
    max:  # The maximum limits
      cpu: 2.5
    min:  # The minimum requests
      cpu: 300m
    type: Container
```

Here, the minimum of the "max" values is the output "max" value, and likewise for "default" and "defaultRequest".
The maximum of the "min" values is the output "min" value.

## ResourceQuota Support

Kubernetes allows users to define [ResourceQuotas](https://kubernetes.io/docs/concepts/policy/resource-quotas/),
which restrict the maximum resource requests and limits of all pods running in a namespace.

To deploy Tekton TaskRuns or PipelineRuns in namespaces with ResourceQuotas, compute resource requirements
must be set for all containers in a `TaskRun`'s pod, including the init containers injected by Tekton.
`Step` and `Sidecar` resource requirements can be configured directly through the API, as described in
[Task Resource Requirements](#task-resource-requirements). To configure resource requirements for Tekton's init containers,
deploy a LimitRange in the same namespace. The LimitRange's `default` and `defaultRequest` will be applied to the init containers,
and divided among the `Steps` and `Sidecars`, as described in [LimitRange Support](#limitrange-support).

[#2933](https://github.com/tektoncd/pipeline/issues/2933) tracks support for running `TaskRuns` in a namespace with a ResourceQuota
without having to use LimitRanges.

ResourceQuotas consider the effective resource requests and limits of a pod, which Kubernetes determines by summing the resource requirements
of its containers (under the assumption that they run in parallel). When using LimitRanges to set compute resources for `TaskRun` pods,
LimitRange default requests are divided among `Step` containers, meaning that the pod's effective requests reflect the actual requests
that the pod needs. However, LimitRange default limits are not divided among containers, meaning the pod's effective limits are much larger
than the limits applied during execution of any given `Step`. For example, if a ResourceQuota restricts a namespace to a limit of 10 CPU,
and a user creates a TaskRun with 20 steps with a limit of 1 CPU each, the pod would not be schedulable even though it is
limited to 1 CPU at each point in time. Therefore, it is recommended to use ResourceQuotas to restrict only requests of `TaskRun` pods,
not limits (tracked in [#4976](https://github.com/tektoncd/pipeline/issues/4976)). 

## Quality of Service (QoS)

By default, pods that run Tekton TaskRuns will have a [Quality of Service (QoS)](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/)
of "BestEffort". If compute resource requirements are set for any Step or Sidecar, the pod will have a "Burstable" QoS.
To get a "Guaranteed" QoS, a TaskRun pod must have compute resources set for all of its containers, including init containers which are
injected by Tekton, and all containers must have their requests equal to their limits.
This can be achieved by using LimitRanges to apply default requests and limits.

# References

- [LimitRange in k8s docs](https://kubernetes.io/docs/concepts/policy/limit-range/)
- [Configure default memory requests and limits for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/)
- [Configure default CPU requests and limits for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/cpu-default-namespace/)
- [Configure Minimum and Maximum CPU constraints for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/cpu-constraint-namespace/)
- [Configure Minimum and Maximum Memory constraints for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-constraint-namespace/)
- [Managing Resources for Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [Kubernetes best practices: Resource requests and limits](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-resource-requests-and-limits)
- [Restrict resource consumption with limit ranges](https://docs.openshift.com/container-platform/4.8/nodes/clusters/nodes-cluster-limit-ranges.html)
