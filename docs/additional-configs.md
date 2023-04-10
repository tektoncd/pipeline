<!--
---
title: "Additional Configuration Options"
linkTitle: "Additional Configuration Options"
weight: 109
description: >
  Additional configurations when installing Tekton Pipelines
---
-->

This document describes additional options to configure your Tekton Pipelines
installation.

## Table of Contents

  - [Configuring built-in remote Task and Pipeline resolution](#configuring-built-in-remote-task-and-pipeline-resolution)
  - [Configuring CloudEvents notifications](#configuring-cloudevents-notifications)
  - [Configuring self-signed cert for private registry](#configuring-self-signed-cert-for-private-registry)
  - [Configuring environment variables](#configuring-environment-variables)
  - [Customizing basic execution parameters](#customizing-basic-execution-parameters)
    - [Customizing the Pipelines Controller behavior](#customizing-the-pipelines-controller-behavior)
    - [Alpha Features](#alpha-features)
    - [Beta Features](#beta-features)
  - [Enabling larger results using sidecar logs](#enabling-larger-results-using-sidecar-logs)
  - [Configuring High Availability](#configuring-high-availability)
  - [Configuring tekton pipeline controller performance](#configuring-tekton-pipeline-controller-performance)
  - [Platform Support](#platform-support)
  - [Creating a custom release of Tekton Pipelines](#creating-a-custom-release-of-tekton-pipelines)
  - [Verify Tekton Pipelines Release](#verify-tekton-pipelines-release)
    - [Verify signatures using `cosign`](#verify-signatures-using-cosign)
    - [Verify the transparency logs using `rekor-cli`](#verify-the-transparency-logs-using-rekor-cli)
  - [Verify Tekton Resources](#verify-tekton-resources)
  - [Pipelinerun with Affinity Assistant](#pipelineruns-with-affinity-assistant)
  - [Next steps](#next-steps)


## Configuring built-in remote Task and Pipeline resolution

Four remote resolvers are currently provided as part of the Tekton Pipelines installation.
By default, these remote resolvers are enabled. Each resolver can be disabled by setting
the appropriate feature flag in the `resolvers-feature-flags` ConfigMap in the `tekton-pipelines-resolvers`
namespace:

1. [The `bundles` resolver](./bundle-resolver.md), disabled by setting the `enable-bundles-resolver`
  feature flag to `false`.
1. [The `git` resolver](./git-resolver.md), disabled by setting the `enable-git-resolver`
   feature flag to `false`.
1. [The `hub` resolver](./hub-resolver.md), disabled by setting the `enable-hub-resolver`
   feature flag to `false`.
1. [The `cluster` resolver](./cluster-resolver.md), disabled by setting the `enable-cluster-resolver`
   feature flag to `false`.

## Configuring CloudEvents notifications

When configured so, Tekton can generate `CloudEvents` for `TaskRun`,
`PipelineRun` and `Run`lifecycle events. The main configuration parameter is the
URL of the sink. When not set, no notification is generated.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-defaults
  namespace: tekton-pipelines
  labels:
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
data:
  default-cloud-events-sink: https://my-sink-url
```

Additionally, CloudEvents for `Runs` require an extra configuration to be
enabled. This setting exists to avoid collisions with CloudEvents that might
be sent by custom task controllers:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feature-flags
  namespace: tekton-pipelines
  labels:
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
data:
  send-cloudevents-for-runs: true
```

## Configuring self-signed cert for private registry

The `SSL_CERT_DIR` is set to `/etc/ssl/certs` as the default cert directory. If you are using a self-signed cert for private registry and the cert file is not under the default cert directory, configure your registry cert in the `config-registry-cert` `ConfigMap` with the key `cert`.

## Configuring environment variables

Environment variables can be configured in the following ways, mentioned in order of precedence from lowest to highest.

1. Implicit environment variables
2. `Step`/`StepTemplate` environment variables
3. Environment variables specified via a `default` `PodTemplate`.
4. Environment variables specified via a `PodTemplate`.

The environment variables specified by a `PodTemplate` supercedes all other ways of specifying environment variables. However, there exists a configuration i.e. `default-forbidden-env`, the environment variable specified in this list cannot be updated via a `PodTemplate`.

For example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-defaults
  namespace: tekton-pipelines
data:
  default-timeout-minutes: "50"
  default-service-account: "tekton"
  default-forbidden-env: "TEST_TEKTON"
---
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: mytask
  namespace: default
spec:
  steps:
    - name: echo-env
      image: ubuntu
      command: ["bash", "-c"]
      args: ["echo $TEST_TEKTON "]
      env:
          - name: "TEST_TEKTON"
            value: "true"
---
apiVersion: tekton.dev/v1beta1
kind: TaskRun
metadata:
  name: mytaskrun
  namespace: default
spec:
  taskRef:
    name: mytask
  podTemplate:
    env:
        - name: "TEST_TEKTON"
          value: "false"
```

_In the above example the environment variable `TEST_TEKTON` will not be overriden by value specified in podTemplate, because the `config-default` option `default-forbidden-env` is configured with value `TEST_TEKTON`._


## Customizing basic execution parameters

You can specify your own values that replace the default service account (`ServiceAccount`), timeout (`Timeout`), resolver (`Resolver`), and Pod template (`PodTemplate`) values used by Tekton Pipelines in `TaskRun` and `PipelineRun` definitions. To do so, modify the ConfigMap `config-defaults` with your desired values.

The example below customizes the following:

- the default service account from `default` to `tekton`.
- the default timeout from 60 minutes to 20 minutes.
- the default `app.kubernetes.io/managed-by` label is applied to all Pods created to execute `TaskRuns`.
- the default Pod template to include a node selector to select the node where the Pod will be scheduled by default. A list of supported fields is available [here](https://github.com/tektoncd/pipeline/blob/main/docs/podtemplates.md#supported-fields).
  For more information, see [`PodTemplate` in `TaskRuns`](./taskruns.md#specifying-a-pod-template) or [`PodTemplate` in `PipelineRuns`](./pipelineruns.md#specifying-a-pod-template).
- the default `Workspace` configuration can be set for any `Workspaces` that a Task declares but that a TaskRun does not explicitly provide.
- the default maximum combinations of `Parameters` in a `Matrix` that can be used to fan out a `PipelineTask`. For
more information, see [`Matrix`](matrix.md).
- the default resolver type to `git`.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-defaults
data:
  default-service-account: "tekton"
  default-timeout-minutes: "20"
  default-pod-template: |
    nodeSelector:
      kops.k8s.io/instancegroup: build-instance-group
  default-managed-by-label-value: "my-tekton-installation"
  default-task-run-workspace-binding: |
    emptyDir: {}
  default-max-matrix-combinations-count: "1024"
  default-resolver-type: "git"
```

**Note:** The `_example` key in the provided [config-defaults.yaml](./../config/config-defaults.yaml)
file lists the keys you can customize along with their default values.

### Customizing the Pipelines Controller behavior

To customize the behavior of the Pipelines Controller, modify the ConfigMap `feature-flags` via
`kubectl edit configmap feature-flags -n tekton-pipelines`.

**Note:** Changing feature flags may result in undefined behavior for TaskRuns and PipelineRuns
that are running while the change occurs.

The flags in this ConfigMap are as follows:

- `disable-affinity-assistant` - set this flag to `true` to disable the [Affinity Assistant](./workspaces.md#specifying-workspace-order-in-a-pipeline-and-affinity-assistants)
  that is used to provide Node Affinity for `TaskRun` pods that share workspace volume.
  The Affinity Assistant is incompatible with other affinity rules
  configured for `TaskRun` pods.

  **Note:** Affinity Assistant use [Inter-pod affinity and anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity)
  that require substantial amount of processing which can slow down scheduling in large clusters
  significantly. We do not recommend using them in clusters larger than several hundred nodes

  **Note:** Pod anti-affinity requires nodes to be consistently labelled, in other words every
  node in the cluster must have an appropriate label matching `topologyKey`. If some or all nodes
  are missing the specified `topologyKey` label, it can lead to unintended behavior.

- `await-sidecar-readiness`: set this flag to `"false"` to allow the Tekton controller to start a
TasksRun's first step immediately without waiting for sidecar containers to be running first. Using
this option should decrease the time it takes for a TaskRun to start running, and will allow TaskRun
pods to be scheduled in environments that don't support [Downward API](https://kubernetes.io/docs/tasks/inject-data-application/downward-api-volume-expose-pod-information/)
volumes (e.g. some virtual kubelet implementations). However, this may lead to unexpected behaviour
with Tasks that use sidecars, or in clusters that use injected sidecars (e.g. Istio). Setting this flag
to `"false"` will mean the `running-in-environment-with-injected-sidecars` flag has no effect.

- `running-in-environment-with-injected-sidecars`: set this flag to `"false"` to allow the
Tekton controller to start a TasksRun's first step immediately if it has no Sidecars specified.
Using this option should decrease the time it takes for a TaskRun to start running.
However, for clusters that use injected sidecars (e.g. Istio) this can lead to unexpected behavior.

- `require-git-ssh-secret-known-hosts`: set this flag to `"true"` to require that
Git SSH Secrets include a `known_hosts` field. This ensures that a git remote server's
key is validated before data is accepted from it when authenticating over SSH. Secrets
that don't include a `known_hosts` will result in the TaskRun failing validation and
not running.

- `enable-tekton-oci-bundles`: set this flag to `"true"` to enable the
  tekton OCI bundle usage (see [the tekton bundle
  contract](./tekton-bundle-contracts.md)). Enabling this option
  allows the use of `bundle` field in `taskRef` and `pipelineRef` for
  `Pipeline`, `PipelineRun` and `TaskRun`. By default, this option is
  disabled (`"false"`), which means it is disallowed to use the
  `bundle` field.

- `disable-creds-init` - set this flag to `"true"` to [disable Tekton's built-in credential initialization](auth.md#disabling-tektons-built-in-auth)
and use Workspaces to mount credentials from Secrets instead.
The default is `false`. For more information, see the [associated issue](https://github.com/tektoncd/pipeline/issues/3399).

- `enable-api-fields`: When using v1beta1 APIs, setting this field to "stable" or "beta"
enables [beta features](#beta-features). When using v1 APIs, setting this field to "stable"
allows only stable features, and setting it to "beta" allows only beta features.
Set this field to "alpha" to allow [alpha features](#alpha-features) to be used.

- `trusted-resources-verification-no-match-policy`: Setting this flag to `fail` will fail the taskrun/pipelinerun if no matching policies found. Setting to `warn` will skip verification and log a warning if no matching policies are found, but not fail the taskrun/pipelinerun. Setting to `ignore` will skip verification if no matching policies found.
Defaults to "ignore".

- `results-from`: set this flag to "termination-message" to use the container's termination message to fetch results from. This is the default method of extracting results. Set it to "sidecar-logs" to enable use of a results sidecar logs to extract results instead of termination message.

- `enable-provenance-in-status`: Set this flag to `"true"` to enable populating
  the `provenance` field in `TaskRun` and `PipelineRun` status. The `provenance`
  field contains metadata about resources used in the TaskRun/PipelineRun such as the
  source from where a remote Task/Pipeline definition was fetched. By default, this is set to `true`.
  To disable populating this field, set this flag to `"false"`.

For example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: feature-flags
data:
  enable-api-fields: "alpha" # Allow alpha fields to be used in Tasks and Pipelines.
```

### Alpha Features

Alpha features in the following table are still in development and their syntax is subject to change.
- To enable the features ***without*** an individual flag:
  set the `enable-api-fields` feature flag to `"alpha"` in the `feature-flags` ConfigMap alongside your Tekton Pipelines deployment via `kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"enable-api-fields":"alpha"}}'`.
- To enable the features ***with*** an individual flag:
  set the individual flag accordingly in the `feature-flag` ConfigMap alongside your Tekton Pipelines deployment. Example: `kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"<FLAG-NAME>":"<FLAG-VALUE>"}}'`.


Features currently in "alpha" are:

| Feature                                                                                             | Proposal                                                                                                                   | Release                                                              | Individual Flag               |
|:----------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------|:------------------------------|
| [Bundles ](./pipelineruns.md#tekton-bundles)                                                        | [TEP-0005](https://github.com/tektoncd/community/blob/main/teps/0005-tekton-oci-bundles.md)                                | [v0.18.0](https://github.com/tektoncd/pipeline/releases/tag/v0.18.0) | `enable-tekton-oci-bundles`   |
| [Isolated `Step` & `Sidecar` `Workspaces`](./workspaces.md#isolated-workspaces)                     | [TEP-0029](https://github.com/tektoncd/community/blob/main/teps/0029-step-workspaces.md)                                   | [v0.24.0](https://github.com/tektoncd/pipeline/releases/tag/v0.24.0) |                               |
| [Hermetic Execution Mode](./hermetic.md)                                                            | [TEP-0025](https://github.com/tektoncd/community/blob/main/teps/0025-hermekton.md)                                         | [v0.25.0](https://github.com/tektoncd/pipeline/releases/tag/v0.25.0) |                               |
| [Windows Scripts](./tasks.md#windows-scripts)                                                       | [TEP-0057](https://github.com/tektoncd/community/blob/main/teps/0057-windows-support.md)                                   | [v0.28.0](https://github.com/tektoncd/pipeline/releases/tag/v0.28.0) |                               |
| [Debug](./debug.md)                                                                                 | [TEP-0042](https://github.com/tektoncd/community/blob/main/teps/0042-taskrun-breakpoint-on-failure.md)                     | [v0.26.0](https://github.com/tektoncd/pipeline/releases/tag/v0.26.0) |                               |
| [Step and Sidecar Overrides](./taskruns.md#overriding-task-steps-and-sidecars)                      | [TEP-0094](https://github.com/tektoncd/community/blob/main/teps/0094-specifying-resource-requirements-at-runtime.md)       | [v0.34.0](https://github.com/tektoncd/pipeline/releases/tag/v0.34.0) |                               |
| [Matrix](./matrix.md)                                                                               | [TEP-0090](https://github.com/tektoncd/community/blob/main/teps/0090-matrix.md)                                            | [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0) |                               |
| [Task-level Resource Requirements](compute-resources.md#task-level-compute-resources-configuration) | [TEP-0104](https://github.com/tektoncd/community/blob/main/teps/0104-tasklevel-resource-requirements.md)                   | [v0.39.0](https://github.com/tektoncd/pipeline/releases/tag/v0.39.0) |                               |
| [Trusted Resources](./trusted-resources.md)                                                         | [TEP-0091](https://github.com/tektoncd/community/blob/main/teps/0091-trusted-resources.md)                                 | N/A                                                                  | `trusted-resources-verification-no-match-policy`  |
| [Larger Results via Sidecar Logs](#enabling-larger-results-using-sidecar-logs)                      | [TEP-0127](https://github.com/tektoncd/community/blob/main/teps/0127-larger-results-via-sidecar-logs.md)                   | [v0.43.0](https://github.com/tektoncd/pipeline/releases/tag/v0.43.0) | `results-from`                |
| [Configure Default Resolver](./resolution.md#configuring-built-in-resolvers)                        | [TEP-0133](https://github.com/tektoncd/community/blob/main/teps/0133-configure-default-resolver.md)                        | N/A                                 |                                |

### Beta Features

Beta features are fields of stable CRDs that follow our "beta" [compatibility policy](../api_compatibility_policy.md).
To enable these features, set the `enable-api-fields` feature flag to `"beta"` in
the `feature-flags` ConfigMap alongside your Tekton Pipelines deployment via
`kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"enable-api-fields":"beta"}}'`.

For beta versions of Tekton CRDs, setting `enable-api-fields` to "beta" is the same as setting it to "stable".

Features currently in "beta" are:

| Feature                                                            | Proposal                                                                                        | Alpha Release                                                        | Beta Release                                                         | Individual Flag |
|:-------------------------------------------------------------------|:------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------|:---------------------------------------------------------------------|:----------------|
| [Array Results and Array Indexing](pipelineruns.md#specifying-parameters)             | [TEP-0076](https://github.com/tektoncd/community/blob/main/teps/0076-array-result-types.md)     | [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0) | [v0.45.0](https://github.com/tektoncd/pipeline/releases/tag/v0.45.0) |                 |
| [Object Parameters and Results](pipelineruns.md#specifying-parameters) | [TEP-0075](https://github.com/tektoncd/community/blob/main/teps/0075-object-param-and-result-types.md) | [v0.46.0](https://github.com/tektoncd/pipeline/releases/tag/v0.46.0) |
| [Remote Tasks](./taskruns.md#remote-tasks) and [Remote Pipelines](./pipelineruns.md#remote-pipelines) | [TEP-0060](https://github.com/tektoncd/community/blob/main/teps/0060-remote-resolution.md) | [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0) |
| [`Provenance` field in Status](pipeline-api.md#provenance)| [issue#5550](https://github.com/tektoncd/pipeline/issues/5550)| [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0)| [v0.48.0](https://github.com/tektoncd/pipeline/releases/tag/v0.48.0) | `enable-provenance-in-status`|
| [CSI workspaces](workspaces.md#csi)| [issue#4446](https://github.com/tektoncd/pipeline/issues/4446)| [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0)| [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0) | |
| [Projected volume workspaces](workspaces.md#projected)| [issue#5075](https://github.com/tektoncd/pipeline/issues/5075)| [v0.38.0](https://github.com/tektoncd/pipeline/releases/tag/v0.38.0)| [v0.41.0](https://github.com/tektoncd/pipeline/releases/tag/v0.41.0) | |

## Enabling larger results using sidecar logs

**Note**: The maximum size of a Task's results is limited by the container termination message feature of Kubernetes, as results are passed back to the controller via this mechanism. At present, the limit is per task is “4096 bytes”. All results produced by the task share this upper limit.

To exceed this limit of 4096 bytes, you can enable larger results using sidecar logs. By enabling this feature, you will have a configurable limit (with a default of 4096 bytes) per result with no restriction on the number of results. The results are still stored in the taskrun crd so they should not exceed the 1.5MB CRD size limit.

**Note**: to enable this feature, you need to grant `get` access to all `pods/log` to the `Tekton pipeline controller`. This means that the tekton pipeline controller has the ability to access the pod logs.

1. Create a cluster role and rolebinding by applying the following spec to provide log access to `tekton-pipelines-controller`.

```
kubectl apply -f optional_config/enable-log-access-to-controller/
```

2. Set the `results-from` feature flag to use sidecar logs by setting `results-from: sidecar-logs` in the [configMap](#customizing-the-pipelines-controller-behavior).

```
kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"results-from":"sidecar-logs"}}'
```

3. If you want the size per result to be something other than 4096 bytes, you can set the `max-result-size` feature flag in bytes by setting `max-result-size: 8192(whatever you need here)`. **Note:** The value you can set here cannot exceed the size of the CRD limit of 1.5 MB.

```
kubectl patch cm feature-flags -n tekton-pipelines -p '{"data":{"max-result-size":"<VALUE-IN-BYTES>"}}'
```

## Configuring High Availability

If you want to run Tekton Pipelines in a way so that webhooks are resiliant against failures and support
high concurrency scenarios, you need to run a [Metrics Server](https://github.com/kubernetes-sigs/metrics-server) in
your Kubernetes cluster. This is required by the [Horizontal Pod Autoscalers](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
to compute replica count.

See [HA Support for Tekton Pipeline Controllers](./enabling-ha.md) for instructions on configuring
High Availability in the Tekton Pipelines Controller.

The default configuration is defined in [webhook-hpa.yaml](./../config/webhook-hpa.yaml) which can be customized
to better fit specific usecases.

## Configuring tekton pipeline controller performance

Out-of-the-box, Tekton Pipelines Controller is configured for relatively small-scale deployments but there have several options for configuring Pipelines' performance are available. See the [Performance Configuration](tekton-controller-performance-configuration.md) document which describes how to change the default ThreadsPerController, QPS and Burst settings to meet your requirements.

## Running TaskRuns and PipelineRuns with restricted pod security standards

To allow TaskRuns and PipelineRuns to run in namespaces with [restricted pod security standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/),
set the "set-security-context" feature flag to "true" in the [feature-flags configMap](#customizing-the-pipelines-controller-behavior). This configuration option applies a [SecurityContext](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
to any containers injected into TaskRuns by the Pipelines controller. This SecurityContext may not be supported in all Kubernetes implementations (for example, OpenShift).

**Note**: running TaskRuns and PipelineRuns in the "tekton-pipelines" namespace is discouraged.

## Platform Support

The Tekton project provides support for running on x86 Linux Kubernetes nodes.

The project produces images capable of running on other architectures and operating systems, but may not be able to help debug issues specific to those platforms as readily as those that affect Linux on x86.

The controller and webhook components are currently built for:

- linux/amd64
- linux/arm64
- linux/arm (Arm v7)
- linux/ppc64le (PowerPC)
- linux/s390x (IBM Z)

The entrypoint component is also built for Windows, which enables TaskRun workloads to execute on Windows nodes.
See [Windows documentation](windows.md) for more information.

## Creating a custom release of Tekton Pipelines

You can create a custom release of Tekton Pipelines by following and customizing the steps in [Creating an official release](https://github.com/tektoncd/pipeline/blob/main/tekton/README.md#create-an-official-release). For example, you might want to customize the container images built and used by Tekton Pipelines.

## Verify Tekton Pipelines Release

> We will refine this process over time to be more streamlined. For now, please follow the steps listed in this section
to verify Tekton pipeline release.

Tekton Pipeline's images are being signed by [Tekton Chains](https://github.com/tektoncd/chains) since [0.27.1](https://github.com/tektoncd/pipeline/releases/tag/v0.27.1). You can verify the images with
`cosign` using the [Tekton's public key](https://raw.githubusercontent.com/tektoncd/chains/main/tekton.pub).

### Verify signatures using `cosign`

With Go 1.16+, you can install `cosign` by running:

```shell
go install github.com/sigstore/cosign/cmd/cosign@latest
```

You can verify Tekton Pipelines official images using the Tekton public key:

```shell
cosign verify -key https://raw.githubusercontent.com/tektoncd/chains/main/tekton.pub gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.28.1
```

which results in:

```shell
Verification for gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.28.1 --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - The signatures were verified against the specified public key
  - Any certificates were verified against the Fulcio roots.
{
  "Critical": {
    "Identity": {
      "docker-reference": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller"
    },
    "Image": {
      "Docker-manifest-digest": "sha256:0c320bc09e91e22ce7f01e47c9f3cb3449749a5f72d5eaecb96e710d999c28e8"
    },
    "Type": "Tekton container signature"
  },
  "Optional": {}
}
```

The verification shows a list of checks performed and returns the digest in `Critical.Image.Docker-manifest-digest`
which can be used to retrieve the provenance from the transparency logs for that image using `rekor-cli`.

### Verify the transparency logs using `rekor-cli`

Install the `rekor-cli` by running:

```shell
go install -v github.com/sigstore/rekor/cmd/rekor-cli@latest
```

Now, use the digest collected from the previous [section](#verify-signatures-using-cosign) in
`Critical.Image.Docker-manifest-digest`, for example,
`sha256:0c320bc09e91e22ce7f01e47c9f3cb3449749a5f72d5eaecb96e710d999c28e8`.

Search the transparency log with the digest just collected:

```shell
rekor-cli search --sha sha256:0c320bc09e91e22ce7f01e47c9f3cb3449749a5f72d5eaecb96e710d999c28e8
```

which results in:

```shell
Found matching entries (listed by UUID):
68a53d0e75463d805dc9437dda5815171502475dd704459a5ce3078edba96226
```

Tekton Chains generates provenance based on the custom [format](https://github.com/tektoncd/chains/blob/main/PROVENANCE_SPEC.md)
in which the `subject` holds the list of artifacts which were built as part of the release. For the Pipeline release,
`subject` includes a list of images including pipeline controller, pipeline webhook, etc. Use the `UUID` to get the provenance:

```shell
rekor-cli get --uuid 68a53d0e75463d805dc9437dda5815171502475dd704459a5ce3078edba96226 --format json | jq -r .Attestation | base64 --decode | jq
```

which results in:

```shell
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "https://tekton.dev/chains/provenance",
  "subject": [
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller",
      "digest": {
        "sha256": "0c320bc09e91e22ce7f01e47c9f3cb3449749a5f72d5eaecb96e710d999c28e8"
      }
    },
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint",
      "digest": {
        "sha256": "2fa7f7c3408f52ff21b2d8c4271374dac4f5b113b1c4dbc7d5189131e71ce721"
      }
    },
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init",
      "digest": {
        "sha256": "83d5ec6addece4aac79898c9631ee669f5fee5a710a2ed1f98a6d40c19fb88f7"
      }
    },
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/imagedigestexporter",
      "digest": {
        "sha256": "e4d77b5b8902270f37812f85feb70d57d6d0e1fed2f3b46f86baf534f19cd9c0"
      }
    },
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop",
      "digest": {
        "sha256": "59b5304bcfdd9834150a2701720cf66e3ebe6d6e4d361ae1612d9430089591f8"
      }
    },
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/pullrequest-init",
      "digest": {
        "sha256": "4992491b2714a73c0a84553030e6056e6495b3d9d5cc6b20cf7bc8c51be779bb"
      }
    },
    {
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook",
      "digest": {
        "sha256": "bf0ef565b301a1981cb2e0d11eb6961c694f6d2401928dccebe7d1e9d8c914de"
      }
    }
  ],
  ...
```

Now, verify the digest in the `release.yaml` by matching it with the provenance, for example, the digest for the release `v0.28.1`:

```shell
curl -s https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.28.1/release.yaml | grep github.com/tektoncd/pipeline/cmd/controller:v0.28.1 | awk -F"github.com/tektoncd/pipeline/cmd/controller:v0.28.1@" '{print $2}'
```

which results in:

```shell
sha256:0c320bc09e91e22ce7f01e47c9f3cb3449749a5f72d5eaecb96e710d999c28e8
```

Now, you can verify the deployment specifications in the `release.yaml` to match each of these images and their digest.
The `tekton-pipelines-controller` deployment specification has a container named `tekton-pipeline-controller` and a
list of image references with their digest as part of the `args`:

```yaml
      containers:
        - name: tekton-pipelines-controller
          image: gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.28.1@sha256:0c320bc09e91e22ce7f01e47c9f3cb3449749a5f72d5eaecb96e710d999c28e8
          args: [
            # These images are built on-demand by `ko resolve` and are replaced
            # by image references by digest.
              "-git-image",
              "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init:v0.28.1@sha256:83d5ec6addece4aac79898c9631ee669f5fee5a710a2ed1f98a6d40c19fb88f7",
              "-entrypoint-image",
              "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint:v0.28.1@sha256:2fa7f7c3408f52ff21b2d8c4271374dac4f5b113b1c4dbc7d5189131e71ce721",
              "-nop-image",
              "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop:v0.28.1@sha256:59b5304bcfdd9834150a2701720cf66e3ebe6d6e4d361ae1612d9430089591f8",
              "-imagedigest-exporter-image",
              "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/imagedigestexporter:v0.28.1@sha256:e4d77b5b8902270f37812f85feb70d57d6d0e1fed2f3b46f86baf534f19cd9c0",
              "-pr-image",
              "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/pullrequest-init:v0.28.1@sha256:4992491b2714a73c0a84553030e6056e6495b3d9d5cc6b20cf7bc8c51be779bb",
```

Similarly, you can verify the rest of the images which were published as part of the Tekton Pipelines release:

```shell
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/imagedigestexporter
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/pullrequest-init
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook
```

## Verify Tekton Resources

Trusted Resources is a feature to verify Tekton Tasks and Pipelines. The current
version of feature supports `v1beta1` `Task` and `Pipeline`. For more details
please take a look at [Trusted Resources](./trusted-resources.md).

## Pipelineruns with Affinity Assistant

The cluster operators can review the [guidelines](developers/affinity-assistant.md) to `cordon` a node in the cluster
with the tekton controller and the affinity assistant is enabled.

## Next steps

To get started with Tekton check the [Introductory tutorials][quickstarts],
the [how-to guides][howtos], and the [examples folder][examples].

---

{{% comment %}}
Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License][cca4], and code samples are licensed
under the [Apache 2.0 License][apache2l].
{{% /comment %}}

[cca4]: https://creativecommons.org/licenses/by/4.0/
[apache2l]: https://www.apache.org/licenses/LICENSE-2.0
