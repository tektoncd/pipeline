# Installing Tekton Pipelines

This guide explains how to install Tekton Pipelines. It covers the following topics:

* [Before you begin](#before-you-begin)
* [Installing Tekton Pipelines on Kubernetes](#installing-tekton-pipelines-on-kubernetes)
* [Installing Tekton PIpelines on OpenShift/MiniShift](#installing-tekton-pipelines-on-openshiftminishift)
* [Configuring artifact storage](#configuring-artifact-storage)
* [Customizing basic execution parameters](#configuring-basic-execution-parameters)
* [Creating a custom release of Tekton Pipelines](#creating-a-custom-release-of-tekton-pipelines)
* [Next steps](#next-steps)

## Before you begin

1. Choose the version of Tekton Pipelines you want to install. You have the following options:

   * **[Official](https://github.com/tektoncd/pipeline/releases)** - install this unless you have
     a specific reason to go for a different release.

   * **[Nightly](../tekton/README.md#nightly-releases)** - may contain bugs,
     install at your own risk. Nightlies live at `gcr.io/tekton-nightly`.
   * **[`HEAD`]** - this is the bleeding edge. It contains unreleased code that may result
     in unpredictable behavior. To get started, see the [development guide](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md) instead of this page.

2. If you don't have an existing Kubernetes cluster, set one up, version 1.15 or later:

   ```bash
   gcloud container clusters create $CLUSTER_NAME \
     --zone=$CLUSTER_ZONE
   ```

3. Grant `cluster-admin` permissions to the current user:

   ```bash
   kubectl create clusterrolebinding cluster-admin-binding \
   --clusterrole=cluster-admin \
   --user=$(gcloud config get-value core/account)
   ```

   See
   [Role-based access control](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control#prerequisites_for_using_role-based_access_control)
   for more information.

## Installing Tekton Pipelines on Kubernetes

To install Tekton Pipelines on a Kubernetes cluster:

1. Run the following command to install Tekton Pipelines and its dependencies:

   ```bash
   kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
   ```
   You can install a specific release using `previous/$VERSION_NUMBER`.
   For example, https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.2.0/release.yaml.
   
   If your container runtime does not support `image-reference:tag@digest`
   (for example, like `cri-o` used in OpenShift 4.x), use `release.notags.yaml` instead:

   ```bash
   kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.notags.yaml
   ```

1. Monitor the installation using the following command until all components show a `Running` status:

   ```bash
   kubectl get pods --namespace tekton-pipelines --watch
   ```

   **Note:** Hit CTRL+C to stop monitoring.

Congratulations! You have successfully installed Tekton Pipelines on your Kubernetes cluster. Next, see the following topics:

* [Configuring artifact storage](#configuring-artifact-storage) to set up artifact storage for Tekton Pipelines.
* [Customizing basic execution parameters](#customizing-basic-execution-parameters) if you need to customize your service account, timeout, or Pod template values.

### Installing Tekton Pipelines on OpenShift/MiniShift

To install Tekton Pipelines on OpenShift/MiniShift, you must first apply the `anyuid` security
context constraint to the `tekton-pipelines-controller` service account. This is required to run the webhook Pod.
See
[Security Context Constraints](https://docs.openshift.com/container-platform/3.11/admin_guide/manage_scc.html)
for more information.

1. Log on as a user with `cluster-admin` privileges. The following example
   uses the default `system:admin` user (`admin:admin` for MiniShift):

   ```bash
   oc login -u system:admin
   ```

1. Set up the project and name space:

   ```bash
   oc new-project tekton-pipelines
   oc adm policy add-scc-to-user anyuid -z tekton-pipelines-controller
   oc apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.notags.yaml
   ```
1. Install Tekton Pipelines:

   ```bash
   oc apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.notags.yaml
   ```
   See the
   [OpenShift CLI documentation](https://docs.openshift.com/container-platform/3.11/cli_reference/get_started_cli.html)
   for more inforomation on the `oc` command.

1. Monitor the installation using the following command until all components show a `Running` status:

   ```bash
   oc get pods --namespace tekton-pipelines --watch
   ```
   
   **Note:** Hit CTRL + C to stop monitoring.

Congratulations! You have successfully installed Tekton Pipelines on your OpenShift/MiniShift environment. Next, see the following topics:

* [Configuring artifact storage](#configuring-artifact-storage) to set up artifact storage for Tekton Pipelines.
* [Customizing basic execution parameters](#customizing-basic-execution-parameters) if you need to customize your service account, timeout, or Pod template values.

## Configuring artifact storage

`Tasks` in Tekton Pipelines need to ingest inputs from and store outputs to one or more common locations.
 You can use one of the following solutions to set up resource storage for Tekton Pipelines:

  * [A persistent volume](#configuring-a-persistent-volume)
  * [A cloud storage bucket](#configuring-a-cloud-storage-bucket)

**Note:** Inputs and output locations for `Tasks` are defined via [`PipelineResources`](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md).

Either option provides the same functionality to Tekton Pipelines. Choose the option that
best suits your business needs. For example:

 - In some environments, creating a persistent volume could be slower than transferring files to/from a cloud storage bucket. 
 - If the cluster is running in multiple zones, accessing a persistent volume could be unreliable. 

### Configuring a persistent volume

To configure a [persistent volume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/), use a `ConfigMap` with the name `config-artifact-pvc` and the following attributes:

- `size`: the size of the volume. Default is 5GiB.
- `storageClassName`: the [storage class](https://kubernetes.io/docs/concepts/storage/storage-classes/) of the volume. The possible values depend on the cluster configuration and the underlying infrastructure provider. Default is the default storage class.

### Configuring a cloud storage bucket

To configure either an [S3 bucket](https://aws.amazon.com/s3/) or a [GCS storage bucket](https://cloud.google.com/storage/), 
use a `ConfigMap` with the name `config-artifact-bucket` and the following attributes:

- `location` - the address of the bucket, for example `gs://mybucket` or `s3://mybucket`.
- `bucket.service.account.secret.name` - the name of the secret containing the credentials for the service account with access to the bucket.
- `bucket.service.account.secret.key` - the key in the secret with the required
  service account JSON file.
- `bucket.service.account.field.name` - the name of the environment variable to use when specifying the
  secret path. Defaults to `GOOGLE_APPLICATION_CREDENTIALS`. Set to `BOTO_CONFIG` if using S3 instead of GCS.
  
**Important:** Configure your bucket's retention policy to delete all files after your `Tasks` finish running.

**Note:** You can only use an S3 bucket locakted in the `us-east-1` region. This is a limitation of [`gsutil`](https://cloud.google.com/storage/docs/gsutil) running a `boto` configuration behind the scenes to access the S3 bucket.

Below is an example configuration that uses an S3 bucket:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: tekton-storage
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
  name: config-artifact-pvc
data:
  location: s3://mybucket
  bucket.service.account.secret.name: tekton-storage
  bucket.service.account.secret.key: boto-config
  bucket.service.account.field.name: BOTO_CONFIG
```

## Customizing basic execution parameters

You can specify your own values that replace the default service account (`ServiceAccount`), timeout (`Timeout`), and Pod template (`PodTemplate`) values used by Tekton Pipelines in `TaskRun` and `PipelineRun` definitions. To do so, use the ConfigMap `config-defaults`.

The example below customizes the following:
- the default service account from `default` to `tekton`.
- the default timeout from 60 minutes to 20 minutes.
- the default `app.kuberrnetes.io/managed-by` label is applied to all Pods created to execute `TaskRuns`.
- the default Pod template to include a node selector to select the node where the Pod will be scheduled by default. 
  For more information, see [`PodTemplate` in `TaskRuns`](./taskruns.md#pod-template) or [`PodTemplate` in `PipelineRuns`](./pipelineruns.md#pod-template).

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
```

**Note:** The `_example` key in the provided [config-defaults.yaml](./../config/config-defaults.yaml)
file lists the keys you can customize along with their default values.

## Creating a custom release of Tekton Pipelines

You can create a custom release of Tekton Pipelines by following and customizing the steps in [Creating an official release](https://github.com/tektoncd/pipeline/blob/master/tekton/README.md#create-an-official-release). For example, you might want to customize the container images built and used by Tekton Pipelines.

## Next steps

To get started with Tekton Pipelines, see the [Tekton Pipelines Tutorial](./tutorial.md) and take a look at our [examples](https://github.com/tektoncd/pipeline/tree/master/examples).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
