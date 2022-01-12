<!--
---
linkTitle: "LimitRange"
weight: 300
---
-->

# `LimitRange` support in Pipeline

## `LimitRange`s, `Requests` and `Limits`

Taken from the [LimitRange in kubernetes docs](https://kubernetes.io/docs/concepts/policy/limit-range/).

By default, containers run with unbounded [compute resources](/docs/concepts/configuration/manage-resources-containers/) on a Kubernetes cluster.
With resource quotas, cluster administrators can restrict resource consumption and creation on a `namespace` basis.
Within a namespace, a Pod or Container can consume as much CPU and memory as defined by the namespace's resource quota. There is a concern that one Pod or Container could monopolize all available resources. A LimitRange is a policy to constrain resource allocations (to Pods or Containers) in a namespace.

A _LimitRange_ provides constraints that can:

- Enforce minimum and maximum compute resources usage per Pod or Container in a namespace.
- Enforce minimum and maximum storage request per PersistentVolumeClaim in a namespace.
- Enforce a ratio between request and limit for a resource in a namespace.
- Set default request/limit for compute resources in a namespace and automatically inject them to Containers at runtime.

`LimitRange` are validating and mutating `Requests` and `Limits`. Let's look, *in a nutshell*, on how those work in Kubernetes.

- **Requests** are not enforced. If the node has more resource available than the request, the container can use it.
- **Limits** on the other hand, are a hard stop. A container going over the limit, will be killed.

Resource types for both are:
-   CPU
-   Memory
-   Ephemeral storage

The next question is : how pods with resource and limits are run/scheduled ?
The scheduler *computes* the amount of CPU and memory requests (using **Requests**) and tries to find a node to schedule it.

A pod's [effective resource requests and limits](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/#resources) are the higher of:
- the sum of all app containers request/limit for a resource
- the effective init request/limit for a resource

For example, if you have the following requests:
-   initContainer1 : 1 CPU, 100Mi memory
-   initContainer2 : 2 CPU, 200Mi memory
-   container1 : 1 CPU, 50Mi memory
-   container2 : 2 CPU, 250Mi memory
-   container3 : 3 CPU, 500Mi memory

The computation will be:
-   CPU : the max init container CPU is 2, and the sum of the container CPUs is 6. The resulting pod will use 6 CPUs, the max of the init container and container CPU values.
-   Memory: the max init container memory is 200Mi, and the sum of the container memory requests is 800Mi.
  The resulting pod will use 800Mi, the max of the init container and container memory values.

## Tekton support

The way Limits and Requests works in Kubernetes is because it is assumed that all containers run in parallel, and init container run before, each one after the others.

That assumption — containers running in parallel — is not true in Tekton. They do all start together (because there is no way around this) **but** the *[entrypoint](https://github.com/tektoncd/pipeline/tree/main/cmd/entrypoint#entrypoint) hack* is making sure they actually run in sequence and thus there is always only one container that is actually consuming some resource at the same time.

This means, we need to handle limits, request and LimitRanges in a *non-standard* way. Since the existing LimitRange mutating webhook won't take Tekton's requirements into account, Tekton needs to fully control all the values the LimitRange webhook might set. Let's try to define that. Tekton needs to take into account all the aspect of the LimitRange : the min/max as well as the default. If there is no default, but there is min/max, Tekton need then to **set** a default value that is between the min/max. If we set the value too low, the Pod won't be able to be created, similar if we set the value too high. **But** those values are set on **containers**, so we **have to** do our own computation to know what request to put on each containers.


## A LimitRange is in the namespace

We need to get the default (limits), default requests, min and max values (if they are here).

Here are the rules for container's resources (requests and limits) computations:
- **init containers:** they won't be summed, so the rules are simple
  - a container needs to have request and limits at least at the `min` and set to the `default` if any.
  - *use the default requests and the default limits (coming from the defaultLimit, or the min, …)*
- **containers:** those will be summed at the end, so it gets a bit complex
  - a container needs to have request and limits at least at the `min`
  - the sum of the container request/limits **should be** as small as possible. This should be
    ensured by using the "smallest" possible request on it.

One thing to note is that, in the case of a LimitRange being present, we need to **not rely** on the pod mutation webhook that takes the default into account ; what this means is, we need to specify all request and limits ourselves so that the mutation webhook doesn't have any work to do.

- **No default value:** if there is no default value, we need to treat the min as the
  default. I think that's also what k8s does, at least in our computation.
- **Default value:** we need to "try" to respect that as much as possible.
  - `defaultLimit` but no `defaultRequest`, then we set `defaultRequest` to be same as `min` (if present).
  - `defaultRequest` but no `defaultlimit`, then we use the `max` limit as the `defaultLimit`
  - no `defaultLimit`, no `defaultRequest`, then we use the `min` as `defaultRequest` and the `max` as `defaultLimit`.

## Multiple LimitRange are in the namespace

Similar to on LimitRange, except we need to act as if it was one LimitRange (virtual) with
the correct value from each of them.

- Take the maximum of the min values
- Take the minimum of the max values
- Take the default request that fits into the previous 2 min/max

Once we have this "virtual" LimitRange, we can act as there was one `LimitRange`. Note that it is possible to define multiple `LimitRange` that would go conflict with each other and block any `Pod` scheduling. Tekton Pipeline will not do anything to try to go around this as it is a behaviour of Kubernetes itself.

# References

- [LimitRange in k8s docs](https://kubernetes.io/docs/concepts/policy/limit-range/)
- [Configure default memory requests and limits for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/)
- [Configure default CPU requests and limits for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/cpu-default-namespace/)
- [Configure Minimum and Maximum CPU constraints for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/cpu-constraint-namespace/)
- [Configure Minimum and Maximum Memory constraints for a Namespace](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-constraint-namespace/)
- [Managing Resources for Containers](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
- [Kubernetes best practices: Resource requests and limits](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-resource-requests-and-limits)
- [Restrict resource consumption with limit ranges](https://docs.openshift.com/container-platform/4.8/nodes/clusters/nodes-cluster-limit-ranges.html)
