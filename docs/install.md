<!--
---
title: "Install Tekton Pipelines"
linkTitle: "Install Tekton Pipelines"
weight: 101
description: >
  Install Tekton Pipelines on your cluster
---
-->

{{% comment %}}
To view the full contents of this page, go to the 
<a href="http://tekton.dev/docs/installation/pipelines/">Tekton website</a>.
{{% /comment %}}

{{% pageinfo %}}
{{% readfile "/vendor/disclaimer.md" %}}
{{% /pageinfo %}}

This guide explains how to install Tekton Pipelines.

## Prerequisites

-   A [Kubernetes cluster][k8s] running version 1.23 or later.
-   If you are running on `macOS`, make sure Docker is running
-   [Kubectl][].
-   Grant `cluster-admin` privileges to the current user. See the [Kubernetes
    role-based access control (RBAC) docs][rbac] for more information.
-   (Optional) Install a [Metrics Server][metrics] if you need support for high
    availability use cases.

See the [local installation guide][local-install] if you want to test Tekton on
your computer.

## Installation

{{% tabs %}}

{{% tab "Kubernetes" %}}
To install Tekton Pipelines on a Kubernetes cluster:

1. Run one of the following commands depending on which version of Tekton
   Pipelines you want to install:

   - **Latest official release:**

     ```bash
     kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
     ```

   - **Nightly release:**

     ```bash
     kubectl apply --filename https://storage.googleapis.com/tekton-releases-nightly/pipeline/latest/release.yaml
     ```

   - **Specific release:**

     ```bash
      kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/previous/<version_number>/release.yaml
     ```

     Replace `<version_number>` with the numbered version you want to install.
     For example, `v0.26.0`.

   - **Untagged release:**

     If your container runtime does not support `image-reference:tag@digest`:

     ```bash
     kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.notags.yaml
     ```

1. Monitor the installation:

   ```bash
   kubectl get pods --namespace tekton-pipelines --watch
   ```

   When all components show `1/1` under the `READY` column, the installation is
   complete. Hit *Ctrl + C* to stop monitoring.

Congratulations! You have successfully installed Tekton Pipelines on your
Kubernetes cluster.

{{% /tab %}}

{{% tab "Google Cloud" %}}

{{% readfile file="/vendor/google/pipelines-install.md" %}}

{{% /tab %}}

{{% tab "OpenShift" %}}

{{% readfile file="/vendor/redhat/pipelines-install.md" %}}

{{% /tab %}}

{{% /tabs %}}

## Additional configuration options

You can enable additional alpha and beta features, customize execution
parameters, configure availability, and many more options. See the
[addition configurations options][post-install] for more information.

## Next steps

To get started with Tekton check the [Introductory tutorials][quickstarts],
the [how-to guides][howtos], and the [examples folder][examples].

---

{{% comment %}}
Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License][cca4], and code samples are licensed
under the [Apache 2.0 License][apache2l].
{{% /comment %}}

[quickstarts]: https://tekton.dev/docs/getting-started/
[howtos]: https://tekton.dev/docs/how-to-guides/
[examples]: https://github.com/tektoncd/pipeline/tree/main/examples/
[cca4]: https://creativecommons.org/licenses/by/4.0/
[apache2l]: https://www.apache.org/licenses/LICENSE-2.0
[post-install]: ./additional-configs.md

## Troubleshooting

1. If `kind create cluster` fails to create cluster with below message. Please check if `Docker` is running on the machine.

  ```
    $ kind create cluster
    ERROR: failed to create cluster: failed to list nodes: command "docker ps -a --filter label=io.x-k8s.kind.cluster=kind --format '{{.Names}}'" failed with error: exit status 1
    Command Output: Cannot connect to the Docker daemon at unix:///Users/USER/.docker/run/docker.sock. Is the docker daemon running?
    Ms-MacBook-Pro:~$ docker ps
    Cannot connect to the Docker daemon at unix:///Users/USER/.docker/run/docker.sock. Is the docker daemon running?
  ```
  After starting docker

  ```
  Ms-MacBook-Pro:~$ kind create cluster
  Creating cluster "kind" ...
  ✓ Ensuring node image (kindest/node:v1.25.3) 🖼 
  ✓ Preparing nodes 📦  
  ✓ Writing configuration 📜 
  ✓ Starting control-plane 🕹️ 
  ✓ Installing CNI 🔌 
  ✓ Installing StorageClass 💾 
  Set kubectl context to "kind-kind"
  You can now use your cluster with:

  kubectl cluster-info --context kind-kind

  Have a question, bug, or feature request? Let us know! https://kind.sigs.k8s.io/#community 🙂
  ```
