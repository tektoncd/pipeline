<!--
---
linkTitle: "Installation"
weight: 100
---
-->

# Installing Tekton Pipelines

This guide explains how to install Tekton Pipelines. It covers the following topics:

- [Before you begin](#before-you-begin)
- [Installing Tekton Pipelines on Kubernetes](#installing-tekton-pipelines-on-kubernetes)
    - [Installing Tekton Pipelines on OpenShift](#installing-tekton-pipelines-on-openshift)
- [Configuring PipelineResource storage](#configuring-pipelineresource-storage)
    - [Configuring a persistent volume](#configuring-a-persistent-volume)
    - [Configuring a cloud storage bucket](#configuring-a-cloud-storage-bucket)
        - [Example configuration for an S3 bucket](#example-configuration-for-an-s3-bucket)
        - [Example configuration for a GCS bucket](#example-configuration-for-a-gcs-bucket)
- [Configuring CloudEvents notifications](#configuring-cloudevents-notifications)
- [Configuring self-signed cert for private registry](#configuring-self-signed-cert-for-private-registry)
- [Customizing basic execution parameters](#customizing-basic-execution-parameters)
    - [Customizing the Pipelines Controller behavior](#customizing-the-pipelines-controller-behavior)
    - [Alpha Features](#alpha-features)
- [Configuring High Availability](#configuring-high-availability)
- [Configuring tekton pipeline controller performance](#configuring-tekton-pipeline-controller-performance)
- [Creating a custom release of Tekton Pipelines](#creating-a-custom-release-of-tekton-pipelines)
- [Verify Tekton Pipelines release](#verify-tekton-pipelines-release)
    - [Verify signatures using `cosign`](#verify-signatures-using-cosign)
    - [Verify the tansparency logs using `rekor-cli`](#verify-the-transparency-logs-using-rekor-cli)
- [Next steps](#next-steps)

## Before you begin

1. You must have a Kubernetes cluster running version 1.21 or later.

   If you don't already have a cluster, you can create one for testing with `kind`.
   [Install `kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) and create a cluster by running [`kind create cluster`](https://kind.sigs.k8s.io/docs/user/quick-start/#creating-a-cluster). This
   will create a cluster running locally, with RBAC enabled and your user granted
   the `cluster-admin` role.

1. If you want to support high availability usecases, install a [Metrics Server](https://github.com/kubernetes-sigs/metrics-server) on your cluster.

1. Choose the version of Tekton Pipelines you want to install. You have the following options:

   * **[Official](https://github.com/tektoncd/pipeline/releases)** - install this unless you have
     a specific reason to go for a different release.
   * **[Nightly](../tekton/README.md#nightly-releases)** - may contain bugs,
     install at your own risk. Nightlies live at `gcr.io/tekton-nightly`.
   * **[`HEAD`]** - this is the bleeding edge. It contains unreleased code that may result
     in unpredictable behavior. To get started, see the [development guide](https://github.com/tektoncd/pipeline/blob/main/DEVELOPMENT.md) instead of this page.

1. Grant `cluster-admin` permissions to the current user.

   See [Role-based access control](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control#prerequisites_for_using_role-based_access_control)
   for more information.

## Installing Tekton Pipelines on Kubernetes

To install Tekton Pipelines on a Kubernetes cluster:

1. Run the following command to install Tekton Pipelines and its dependencies:

   ```bash
   kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
   ```

   Or, for the nightly release, use:

   ```bash
   kubectl apply --filename https://storage.googleapis.com/tekton-releases-nightly/pipeline/latest/release.yaml
   ```

   You can install a specific release using `previous/$VERSION_NUMBER`. For example:

   ```bash
    kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.2.0/release.yaml
   ```

   If your container runtime does not support `image-reference:tag@digest`
   (for example, like `cri-o` used in OpenShift 4.x), use `release.notags.yaml` instead:

   ```bash
   kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.notags.yaml
   ```

1. **Note**: Some cloud providers (such as [GKE](https://github.com/tektoncd/pipeline/issues/3317#issuecomment-708066087))
   may also require you to allow port 8443 in your firewall rules so that the Tekton Pipelines webhook is reachable.

1. Monitor the installation using the following command until all components show a `Running` status:

   ```bash
   kubectl get pods --namespace tekton-pipelines --watch
   ```

   **Note:** Hit CTRL+C to stop monitoring.

Congratulations! You have successfully installed Tekton Pipelines on your Kubernetes cluster. Next, see the following topics:

* [Configuring PipelineResource storage](#configuring-pipelineresource-storage) to set up artifact storage for Tekton Pipelines.
* [Customizing basic execution parameters](#customizing-basic-execution-parameters) if you need to customize your service account, timeout, or Pod template values.

### Installing Tekton Pipelines on OpenShift

To install Tekton Pipelines on OpenShift, you must first apply the `anyuid` security
context constraint to the `tekton-pipelines-controller` service account. This is required to run the webhook Pod.
See
[Security Context Constraints](https://docs.openshift.com/container-platform/4.3/authentication/managing-security-context-constraints.html)
for more information.

1. Log on as a user with `cluster-admin` privileges. The following example
   uses the default `system:admin` user:

   ```bash
   oc login -u system:admin
   ```

1. Set up the namespace (project) and configure the service account:

   ```bash
   oc new-project tekton-pipelines
   oc adm policy add-scc-to-user anyuid -z tekton-pipelines-controller
   oc adm policy add-scc-to-user anyuid -z tekton-pipelines-webhook
   ```
1. Install Tekton Pipelines:

   ```bash
   oc apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.notags.yaml
   ```
   See the
   [OpenShift CLI documentation](https://docs.openshift.com/container-platform/4.3/cli_reference/openshift_cli/getting-started-cli.html)
   for more information on the `oc` command.

1. Monitor the installation using the following command until all components show a `Running` status:

   ```bash
   oc get pods --namespace tekton-pipelines --watch
   ```

   **Note:** Hit CTRL + C to stop monitoring.

Congratulations! You have successfully installed Tekton Pipelines on your OpenShift environment. Next, see the following topics:

* [Configuring PipelineResource storage](#configuring-pipelineresource-storage) to set up artifact storage for Tekton Pipelines.
* [Customizing basic execution parameters](#customizing-basic-execution-parameters) if you need to customize your service account, timeout, or Pod template values.

If you want to run OpenShift 4.x on your laptop (or desktop), you
should take a look at [Red Hat CodeReady Containers](https://github.com/code-ready/crc).

## Configuring PipelineResource storage

> :warning: **`PipelineResources` are [deprecated](deprecations.md#deprecation-table).**
>
> For storage, consider using [`Workspaces`](workspaces.md) with [`VolumeClaimTemplates`](https://github.com/tektoncd/pipeline/blob/main/docs/workspaces.md#volumeclaimtemplate)
> to automatically provision and manage Persistent Volume Claims (PVCs). Read more in [TEP-0074](https://github.com/tektoncd/community/blob/main/teps/0074-deprecate-pipelineresources.md).

PipelineResources are one of the ways that Tekton passes data between Tasks. If you intend to
use PipelineResources in your Pipelines then you'll need to configure a storage location
for that data to be put so that it can be shared between Tasks in the Pipeline.

The storage options available for sharing PipelineResources between Tasks in a Pipeline are:

  * [A persistent volume](#configuring-a-persistent-volume)
  * [A cloud storage bucket](#configuring-a-cloud-storage-bucket)

Either option provides the same functionality to Tekton Pipelines. Choose the option that
best suits your business needs. For example:

 - In some environments, creating a persistent volume could be slower than transferring files to/from a cloud storage bucket.
 - If the cluster is running in multiple zones, accessing a persistent volume could be unreliable.

**Note:** To customize the names of the `ConfigMaps` for artifact persistence (e.g. to avoid collisions with other services), rename the `ConfigMap` and update the env value defined [controller.yaml](https://github.com/tektoncd/pipeline/blob/e153c6f2436130e95f6e814b4a792fb2599c57ef/config/controller.yaml#L66-L75).

### Configuring a persistent volume

To configure a [persistent volume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/), use a `ConfigMap` with the name `config-artifact-pvc` and the following attributes:

- `size`: the size of the volume. Default is 5GiB.
- `storageClassName`: the [storage class](https://kubernetes.io/docs/concepts/storage/storage-classes/) of the volume. The possible values depend on the cluster configuration and the underlying infrastructure provider. Default is the default storage class.

### Configuring a cloud storage bucket

To configure either an [S3 bucket](https://aws.amazon.com/s3/) or a [GCS bucket](https://cloud.google.com/storage/),
use a `ConfigMap` with the name `config-artifact-bucket` and the following attributes:

- `location` - the address of the bucket, for example `gs://mybucket` or `s3://mybucket`.
- `bucket.service.account.secret.name` - the name of the secret containing the credentials for the service account with access to the bucket.
- `bucket.service.account.secret.key` - the key in the secret with the required
  service account JSON file.
- `bucket.service.account.field.name` - the name of the environment variable to use when specifying the
  secret path. Defaults to `GOOGLE_APPLICATION_CREDENTIALS`. Set to `BOTO_CONFIG` if using S3 instead of GCS.

**Important:** Configure your bucket's retention policy to delete all files after your `Tasks` finish running.

**Note:** You can only use an S3 bucket located in the `us-east-1` region. This is a limitation of [`gsutil`](https://cloud.google.com/storage/docs/gsutil) running a `boto` configuration behind the scenes to access the S3 bucket.


#### Example configuration for an S3 bucket

Below is an example configuration that uses an S3 bucket:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tekton-storage
  namespace: tekton-pipelines
type: kubernetes.io/opaque
stringData:
  boto-config: |
    [Credentials]
    aws_access_key_id = AWS_ACCESS_KEY_ID
    aws_secret_access_key = AWS_SECRET_ACCESS_KEY
    [s3]
    host = s3.us-east-1.amazonaws.com
    [Boto]
    https_validate_certificates = True
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-artifact-bucket
  namespace: tekton-pipelines
data:
  location: s3://mybucket
  bucket.service.account.secret.name: tekton-storage
  bucket.service.account.secret.key: boto-config
  bucket.service.account.field.name: BOTO_CONFIG
```

#### Example configuration for a GCS bucket

Below is an example configuration that uses a GCS bucket:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tekton-storage
  namespace: tekton-pipelines
type: kubernetes.io/opaque
stringData:
  gcs-config: |
    {
      "type": "service_account",
      "project_id": "gproject",
      "private_key_id": "some-key-id",
      "private_key": "-----BEGIN PRIVATE KEY-----\nME[...]dF=\n-----END PRIVATE KEY-----\n",
      "client_email": "tekton-storage@gproject.iam.gserviceaccount.com",
      "client_id": "1234567890",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/tekton-storage%40gproject.iam.gserviceaccount.com"
    }
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-artifact-bucket
  namespace: tekton-pipelines
data:
  location: gs://mybucket
  bucket.service.account.secret.name: tekton-storage
  bucket.service.account.secret.key: gcs-config
  bucket.service.account.field.name: GOOGLE_APPLICATION_CREDENTIALS
```

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

## Customizing basic execution parameters

You can specify your own values that replace the default service account (`ServiceAccount`), timeout (`Timeout`), and Pod template (`PodTemplate`) values used by Tekton Pipelines in `TaskRun` and `PipelineRun` definitions. To do so, modify the ConfigMap `config-defaults` with your desired values.

The example below customizes the following:

- the default service account from `default` to `tekton`.
- the default timeout from 60 minutes to 20 minutes.
- the default `app.kubernetes.io/managed-by` label is applied to all Pods created to execute `TaskRuns`.
- the default Pod template to include a node selector to select the node where the Pod will be scheduled by default. A list of supported fields is available [here](https://github.com/tektoncd/pipeline/blob/main/docs/podtemplates.md#supported-fields).
  For more information, see [`PodTemplate` in `TaskRuns`](./taskruns.md#specifying-a-pod-template) or [`PodTemplate` in `PipelineRuns`](./pipelineruns.md#specifying-a-pod-template).
- the default `Workspace` configuration can be set for any `Workspaces` that a Task declares but that a TaskRun does not explicitly provide

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
```

**Note:** The `_example` key in the provided [config-defaults.yaml](./../config/config-defaults.yaml)
file lists the keys you can customize along with their default values.

### Customizing the Pipelines Controller behavior

To customize the behavior of the Pipelines Controller, modify the ConfigMap `feature-flags` as follows:

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

- `running-in-environment-with-injected-sidecars`: set this flag to `"true"` to allow the
Tekton controller to set the `tekton.dev/ready` annotation at pod creation time for
TaskRuns with no Sidecars specified. Enabling this option should decrease the time it takes for a TaskRun to
start running. However, for clusters that use injected sidecars e.g. istio
enabling this option can lead to unexpected behavior.

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

- `enable-custom-tasks`: set this flag to `"true"` to enable the
use of custom tasks in pipelines.

- `enable-api-fields`: set this flag to "stable" to allow only the
most stable features to be used. Set it to "alpha" to allow [alpha
features](#alpha-features) to be used.

- `embedded-status`: set this flag to "full" to enable full embedding of `TaskRun` and `Run` statuses in the 
 `PipelineRun` status. Set it to "minimal" to populate the `ChildReferences` field in the `PipelineRun` status with
  name, kind, and API version information for each `TaskRun` and `Run` in the `PipelineRun` instead. Set it to "both" to 
  do both. For more information, see [Configuring usage of `TaskRun` and `Run` embedded statuses](pipelineruns.md#configuring-usage-of-taskrun-and-run-embedded-statuses).

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

Alpha features are still in development and their syntax is subject to change.
To enable these, set the `enable-api-fields` feature flag to `"alpha"` in
the `feature-flags` ConfigMap alongside your Tekton Pipelines deployment.

Features currently in "alpha" are:

| Feature                                                                                               | TEP                                                                                                                  | Release                                                              | Individual Flag             |
|:------------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------------------------------------------------------|:---------------------------------------------------------------------|:----------------------------|
| [Bundles ](./pipelineruns.md#tekton-bundles)                                                          | [TEP-0005](https://github.com/tektoncd/community/blob/main/teps/0005-tekton-oci-bundles.md)                          | [v0.18.0](https://github.com/tektoncd/pipeline/releases/tag/v0.18.0) | `enable-tekton-oci-bundles` |
| [`Runs` and `Custom Tasks`](./runs.md)                                                                | [TEP-0002](https://github.com/tektoncd/community/blob/main/teps/0002-custom-tasks.md)                                | [v0.19.0](https://github.com/tektoncd/pipeline/releases/tag/v0.19.0) | `enable-custom-tasks`       |
| [Isolated `Step` & `Sidecar` `Workspaces`](./workspaces.md#isolated-workspaces)                       | [TEP-0029](https://github.com/tektoncd/community/blob/main/teps/0029-step-workspaces.md)                             | [v0.24.0](https://github.com/tektoncd/pipeline/releases/tag/v0.24.0) |                             |
| [Hermetic Execution Mode](./hermetic.md)                                                              | [TEP-0025](https://github.com/tektoncd/community/blob/main/teps/0025-hermekton.md)                                   | [v0.25.0](https://github.com/tektoncd/pipeline/releases/tag/v0.25.0) |                             |
| [`PipelineRun` Timeouts](./pipelineruns.md#configuring-a-failure-timeout)                             | [TEP-0046](https://github.com/tektoncd/community/blob/main/teps/0046-finallytask-execution-post-timeout.md)          | [v0.25.0](https://github.com/tektoncd/pipeline/releases/tag/v0.25.0) |                             |
| [Implicit `Parameters`](./taskruns.md#implicit-parameters)                                            | [TEP-0023](https://github.com/tektoncd/community/blob/main/teps/0023-implicit-mapping.md)                            | [v0.28.0](https://github.com/tektoncd/pipeline/releases/tag/v0.28.0) |                             |
| [Windows Scripts](./tasks.md#windows-scripts)                                                         | [TEP-0057](https://github.com/tektoncd/community/blob/main/teps/0057-windows-support.md)                             | [v0.28.0](https://github.com/tektoncd/pipeline/releases/tag/v0.28.0) |                             |
| [Remote Tasks](./taskruns.md#remote-tasks) and [Remote Pipelines](./pipelineruns.md#remote-pipelines) | [TEP-0060](https://github.com/tektoncd/community/blob/main/teps/0060-remote-resolutiond.md)                          |                                                                      |                             |
| [Debug](./debug.md)                                                                                   | [TEP-0042](https://github.com/tektoncd/community/blob/main/teps/0042-taskrun-breakpoint-on-failure.md)               | [v0.26.0](https://github.com/tektoncd/pipeline/releases/tag/v0.26.0) |                             |
| [Step and Sidecar Overrides](./taskruns.md#overriding-task-steps-and-sidecars)                        | [TEP-0094](https://github.com/tektoncd/community/blob/main/teps/0094-specifying-resource-requirements-at-runtime.md) |                                                                      |                             |
| [Matrix](./matrix.md)                                                                                 | [TEP-0090](https://github.com/tektoncd/community/blob/main/teps/0090-matrix.md)                                      |                                                                      |                             |
| [Embedded Statuse](pipelineruns.md#configuring-usage-of-taskrun-and-run-embedded-statuses)                                                                                  | [TEP-0100](https://github.com/tektoncd/community/blob/main/teps/0100-embedded-taskruns-and-runs-status-in-pipelineruns.md)                                      |                                                                      |                             |

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

Additional components to support PipelineResources may be available for other architectures as well.

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
      "name": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/kubeconfigwriter",
      "digest": {
        "sha256": "55963ed3fb6157e5f8dac7a315a794ebe362e46714631f9c79d79d33fe769e4d"
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
              "-kubeconfig-writer-image",
              "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/kubeconfigwriter:v0.28.1@sha256:55963ed3fb6157e5f8dac7a315a794ebe362e46714631f9c79d79d33fe769e4d",
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
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/kubeconfigwriter
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/git-init
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/entrypoint
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/imagedigestexporter
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/pullrequest-init
gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook
```

## Next steps

To get started with Tekton Pipelines, see the [Tekton Pipelines Tutorial](./tutorial.md) and take a look at our [examples](https://github.com/tektoncd/pipeline/tree/main/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
