# Installing Tekton Pipelines on OpenShift/MiniShift

**Important**: If you would like to use Tekton Pipelines with a standard
Kubernetes cluster, see [Installing Tekton Pipelines](./install.md)
instead.

## Before you begin

1. [Set up your OpenShift/MiniShift cluster](https://openshift.com/learn/get-started/) and
[install the OpenShift CLI tool](https://docs.openshift.com/container-platform/3.11/cli_reference/get_started_cli.html).

2. Log in your OpenShift/MiniShift cluster as a user with the `cluster-admin`
privileges. The following example uses the default `system:admin` user
(`admin:admin` for MiniShift):

    ```bash
    # Use oc login -u admin:admin with MiniShift instead
    oc login -u system:admin
    ```

2. Set up a new project/namespace, such as `tekton-pipelines`:

    ```bash
    oc new-project tekton-pipelines
    ```

3. Tekton Pipelines uses the `tekton-pipelines-controller` service account,
which requires the [`anyuid` security context constraint](https://docs.openshift.com/container-platform/3.11/admin_guide/manage_scc.html)
for running. To add this security context constraint:

    ```bash
    oc adm policy add-scc-to-user anyuid -z tekton-pipelines-controller
    ```

## Installation

1. To install Tekton pipelines and its dependencies, run the following command:

    ```bash
    oc apply --filename https://storage.googleapis.com/tekton-releases/latest/release.yaml
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
    oc get pods --namespace tekton-pipelines
    ```

    If Tekton Pipelines has been installed successfully, the output should
    look like:

    ```
    NAME                                          READY   STATUS    RESTARTS   AGE
    tekton-pipelines-controller-...               1/1     Running   0          ...
    tekton-pipelines-webhook-...                  1/1     Running   0          ...
    ```

    All the components of Tekton Pipelines should be of the status `Running`.

    **Note**: You may use the `--watch` flag with the `oc get` command to
    see the update of status in real time. To exit, press CTRL + C. 

## What's next

* To get started on using Tekton Pipelines, see [Tekton Pipelines Hello World Tutorial](./tutorial.md).
* [A number of Tekton Pipelines examples are available on GitHub](https://github.com/tektoncd/pipeline/tree/master/examples).