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

-   A [Kubernetes cluster][k8s] running version 1.24 or later.
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
[addition configurations options](./additional-configs.md) for more information.

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
[k8s]: https://www.downloadkubernetes.com/
[kubectl]: https://www.downloadkubernetes.com/
[rbac]: https://kubernetes.io/docs/reference/access-authn-authz/rbac/
[metrics]: https://github.com/kubernetes-sigs/metrics-server
[local-install]: https://tekton.dev/docs/installation/local-installation/

