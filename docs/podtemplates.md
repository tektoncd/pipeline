# PodTemplates

A pod template specifies a subset of
[`PodSpec`](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.15/#pod-v1-core)
configuration that will be used as the basis for the `Task` pod.

This allows to customize some Pod specific field per `Task` execution, aka `TaskRun`.

Alternatively, you can also define a default pod template in tekton config, see [here](./install.md)
When a pod template is specified for a `PipelineRun` or `TaskRun`, the default pod template is ignored, ie
both templates are **NOT** merged, it's always one or the other.

---

The current fields supported are:

- `nodeSelector`: a selector which must be true for the pod to fit on
  a node, see [here](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/).
- `tolerations`: allow (but do not require) the pods to schedule onto
  nodes with matching taints.
- `affinity`: allow to constrain which nodes your pod is eligible to
  be scheduled on, based on labels on the node.
- `securityContext`: pod-level security attributes and common
  container settings, like `runAsUser` or `selinux`.
- `volumes`: list of volumes that can be mounted by containers
  belonging to the pod. This lets the user of a Task define which type
  of volume to use for a Task `volumeMount`
- `runtimeClassName`: the name of a
  [runtime class](https://kubernetes.io/docs/concepts/containers/runtime-class/)
  to use to run the pod.
- `automountServiceAccountToken`: whether the token for the service account
  being used by the pod should be automatically provided inside containers at a
  predefined path. Defaults to `true`.
- `dnsPolicy`: the
  [DNS policy](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy)
  for the pod, one of `ClusterFirst`, `Default`, or `None`. Defaults to
  `ClusterFirst`. Note that `ClusterFirstWithHostNet` is not supported by Tekton
  as Tekton pods cannot run with host networking.
- `dnsConfig`:
  [additional DNS configuration](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config)
  for the pod, such as nameservers and search domains.
- `enableServiceLinks`: whether services in the same namespace as the pod will
  be exposed as environment variables to the pod, similar to Docker service
  links. Defaults to `true`.
- `priorityClassName`: the name of the
  [priority class](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/)
  to use when running the pod. Use this, for example, to selectively enable
  preemption on lower priority workloads.


A pod template can be specified for `TaskRun` or `PipelineRun` resources.
See [here](./taskruns.md#pod-template) or [here](./pipelineruns.md#pod-template) for examples using pod templates.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
