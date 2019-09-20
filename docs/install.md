# Installing Tekton Pipelines

This document explains how to install Tekton pipelines in a Kubernetes cluster.

**Important**: If you would like to use Tekton Pipelines with an
OpenShift/MiniShift cluster, see [Installing Tekton Pipelines on OpenShift/MiniShift](./install-openshift.md)
instead.

## Before you begin

1. Set up your Kubernetes cluster. Tekton pipelines requires Kubernetes 1.11 or
newer versions.

    **Note**: For more information about Kubernetes cluster setup and/or
    upgrade, see the documentation of your Kubernetes service provider. If you
    are using [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/),
    for example, see [Google Kubernetes Engine Quickstart](https://cloud.google.com/kubernetes-engine/docs/quickstart). 

2. Enable [Role-Based Access Control (RBAC)](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
in your Kubernetes cluster and grant your user account the `cluster-admin` role.
[Google Kubernetes Engine clusters have RBAC enabled by default](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control).

    ```bash
    kubectl create clusterrolebinding cluster-admin-binding \
    --clusterrole=cluster-admin \
    --user=[YOUR-USER-ACCOUNT]
    ```

    Replace `[YOUR-USER-ACCOUNT]` with the account of your own. If you are
    using Google Kubernetes Engine and have [Google Cloud SDK](https://cloud.google.com/sdk)
    installed, you can find this value using the command `gcloud config get-value core/account`.
    For other Kubernetes service providers, see their documentation for
    instructions.

## Installation

1. To install Tekton pipelines and its dependencies, run the following command:

    ```bash
    kubectl apply --filename https://storage.googleapis.com/tekton-releases/latest/release.yaml
    ```

    **Note**: The command above installs automatically the latest official
    release of Tekton pipelines. If you prefer using a different version,
    see [Tekton Pipelines Releases](https://github.com/tektoncd/pipeline/releases).
    Alternatively, you may also use [Tekton Pipelines Nightly Releases](../tekton/README.md#nightly-releases)
    available at `gcr.io/tekton-nightly`, [the most recent unreleased Tekton
    Pipelines code directly from the repository](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md),
    or [a custom release of your own](../tekton/README.md).

2. It may take a few minutes to complete the installation. To check the
installation progress, run the command below:

    ```bash
    kubectl get pods --namespace tekton-pipelines
    ```

    If Tekton Pipelines has been installed successfully, the output should
    look like:

    ```
    NAME                                          READY   STATUS    RESTARTS   AGE
    tekton-pipelines-controller-...               1/1     Running   0          ...
    tekton-pipelines-webhook-...                  1/1     Running   0          ...
    ```

    All the components of Tekton Pipelines should be of the status `Running`.

    **Note**: You may use the `--watch` flag with the `kubectl get` command to
    see the update of status in real time. To exit, press CTRL + C. 

## What's next

* To get started on using Tekton Pipelines, see [Tekton Pipelines Hello World Tutorial](./tutorial.md).
* [A number of Tekton Pipelines examples are available on GitHub](https://github.com/tektoncd/pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
