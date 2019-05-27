# Installing Tekton Pipelines

Use this page to add the component to an existing Kubernetes cluster.

## Pre-requisites

1. A Kubernetes cluster (_if you don't have an existing cluster_):

   ```bash
   # Example cluster creation command on GKE
   gcloud container clusters create $CLUSTER_NAME \
     --zone=$CLUSTER_ZONE
   ```

2. Grant cluster-admin permissions to the current user:

   ```bash
   kubectl create clusterrolebinding cluster-admin-binding \
   --clusterrole=cluster-admin \
   --user=$(gcloud config get-value core/account)
   ```

   _See
   [Role-based access control](https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control#prerequisites_for_using_role-based_access_control)
   for more information_.

## Adding the Tekton Pipelines

To add the Tekton Pipelines component to an existing cluster:

1. Run the
   [`kubectl apply`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#apply)
   command to install [Tekton Pipelines](https://github.com/tektoncd/pipeline)
   and its dependencies:

   ```bash
   kubectl apply --filename https://storage.googleapis.com/tekton-releases/latest/release.yaml
   ```

   _(Previous versions will be available at `previous/$VERSION_NUMBER`, e.g.
   https://storage.googleapis.com/tekton-releases/previous/0.2.0/release.yaml.)_

1. Run the
   [`kubectl get`](https://kubernetes.io/docs/reference/generated/kubectl/kubectl-commands#get)
   command to monitor the Tekton Pipelines components until all of the
   components show a `STATUS` of `Running`:

   ```bash
   kubectl get pods --namespace tekton-pipelines
   ```

   Tip: Instead of running the `kubectl get` command multiple times, you can
   append the `--watch` flag to view the component's status updates in real
   time. Use CTRL + C to exit watch mode.

You are now ready to create and run Tekton Pipelines:

- See [Tekton Pipeline tutorial](./tutorial.md) to get started.
- Look at the
  [examples](https://github.com/tektoncd/pipeline/tree/master/examples)

### Installing Tekton Pipelines on OpenShift/MiniShift

The `tekton-pipelines-controller` service account needs the `anyuid` security
context constraint in order to run the webhook pod.

_See
[Security Context Constraints](https://docs.openshift.com/container-platform/3.11/admin_guide/manage_scc.html)
for more information_

1. First, login as a user with `cluster-admin` privileges. The following example
   uses the default `system:admin` user (`admin:admin` for MiniShift):

   ```bash
   # For MiniShift: oc login -u admin:admin
   oc login -u system:admin
   ```

1. Run the following commands to set up the project/namespace, and to install
   Tekton Pipelines:

   ```bash
   oc new-project tekton-pipelines
   oc adm policy add-scc-to-user anyuid -z tekton-pipelines-controller
   oc apply --filename https://storage.googleapis.com/tekton-releases/latest/release.yaml
   ```

   _See
   [here](https://docs.openshift.com/container-platform/3.11/cli_reference/get_started_cli.html)
   for an overview of the `oc` command-line tool for OpenShift._

1. Run the `oc get` command to monitor the Tekton Pipelines components until all
   of the components show a `STATUS` of `Running`:

   ```bash
   oc get pods --namespace tekton-pipelines --watch
   ```

## Configuring Tekton Pipelines

### How are resources shared between tasks

Pipelines need a way to share resources between tasks. The alternatives are a
[Persistent volume](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
or a [GCS storage bucket](https://cloud.google.com/storage/)

The PVC option can be configured using a ConfigMap with the name
`config-artifact-pvc` and the following attributes:

- size: the size of the volume (5Gi by default)

The GCS storage bucket can be configured using a ConfigMap with the name
`config-artifact-bucket` with the following attributes:

- location: the address of the bucket (for example gs://mybucket)
- bucket.service.account.secret.name: the name of the secret that will contain
  the credentials for the service account with access to the bucket
- bucket.service.account.secret.key: the key in the secret with the required
  service account json.
- The bucket is recommended to be configured with a retention policy after which
  files will be deleted.

Both options provide the same functionality to the pipeline. The choice is based
on the infrastructure used, for example in some Kubernetes platforms, the
creation of a persistent volume could be slower than uploading/downloading files
to a bucket, or if the the cluster is running in multiple zones, the access to
the persistent volume can fail.

## Custom Releases

The [release Task](./../tekton/README.md) can be used for creating a custom
release of Tekton Pipelines. This can be useful for advanced users that need to
configure the container images built and used by the Pipelines components.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
