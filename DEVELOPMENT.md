# Developing

## Getting started

1. Create [a GitHub account](https://github.com/join)
1. Setup
   [GitHub access via SSH](https://help.github.com/articles/connecting-to-github-with-ssh/)
1. [Create and checkout a repo fork](#checkout-your-fork)
1. Set up your [shell environment](#environment-setup)
1. Install [requirements](#requirements)
1. [Set up a Kubernetes cluster](#kubernetes-cluster)
1. [Configure kubectl to use your cluster](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
1. [Set up a docker repository you can push to](https://github.com/knative/serving/blob/master/docs/setting-up-a-docker-registry.md)

Then you can [iterate](#iterating) (including
[runing the controllers with `ko`](#install-pipeline)).

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/knative/build-pipeline` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/knative
cd ${GOPATH}/src/github.com/knative
git clone git@github.com:${YOUR_GITHUB_USERNAME}/build-pipeline.git
cd build-pipeline
git remote add upstream git@github.com:knative/build-pipeline.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

### Requirements

You must install these tools:

1. [`go`](https://golang.org/doc/install): The language Tekton Pipelines is
   built in
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`dep`](https://github.com/golang/dep): For managing external Go
   dependencies. - Please Install dep v0.5.0 or greater.
1. [`ko`](https://github.com/google/go-containerregistry/tree/master/cmd/ko):
   For development.
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster

Your [`$GOPATH`] setting is critical for `ko apply` to function properly: a
successful run will typically involve building pushing images instead of only
configuring Kubernetes resources.

## Kubernetes cluster

Docker for Desktop using an edge version has been proven to work for both
developing and running Pipelines. Your Kubernetes version must be 1.11 or later.

To setup a cluster with GKE:

1. [Install required tools and setup GCP project](https://github.com/knative/docs/blob/master/install/Knative-with-GKE.md#before-you-begin)
   (You may find it useful to save the ID of the project in an environment
   variable (e.g. `PROJECT_ID`).
1. [Create a GKE cluster](https://github.com/knative/docs/blob/master/install/Knative-with-GKE.md#creating-a-kubernetes-cluster)

Note that
[the `--scopes` argument to `gcloud container cluster create`](https://cloud.google.com/sdk/gcloud/reference/container/clusters/create#--scopes)
controls what GCP resources the cluster's default service account has access to;
for example to give the default service account full access to your GCR
registry, you can add `storage-full` to your `--scopes` arg.

## Environment Setup

To [run your controllers with `ko`](#install-pipeline) you'll need to set these
environment variables (we recommend adding them to your `.bashrc`):

1. `GOPATH`: If you don't have one, simply pick a directory and add
   `export GOPATH=...`
1. `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.
1. `KO_DOCKER_REPO`: The docker repository to which developer images should be
   pushed (e.g. `gcr.io/[gcloud-project]`). You can also run a local registry
   and set `KO_DOCKER_REPO` to reference the registry (e.g. at
   `localhost:5000/mypipelineimages`).

`.bashrc` example:

```shell
export GOPATH="$HOME/go"
export PATH="${PATH}:${GOPATH}/bin"
export KO_DOCKER_REPO='gcr.io/my-gcloud-project-name'
```

Make sure to configure
[authentication](https://cloud.google.com/container-registry/docs/advanced-authentication#standalone_docker_credential_helper)
for your `KO_DOCKER_REPO` if required. To be able to push images to
`gcr.io/<project>`, you need to run this once:

```shell
gcloud auth configure-docker
```

The user you are using to interact with your k8s cluster must be a cluster admin
to create role bindings:

```shell
# Using gcloud to get your current user
USER=$(gcloud config get-value core/account)
# Make that user a cluster admin
kubectl create clusterrolebinding cluster-admin-binding \
  --clusterrole=cluster-admin \
  --user="${USER}"
```

## Iterating

While iterating on the project, you may need to:

1. [Install/Run everything](#install-pipeline)
1. Verify it's working by [looking at the logs](#accessing-logs)
1. Update your (external) dependencies with: `./hack/update-deps.sh`.

   **Running dep ensure manually, will pull a bunch of scripts deleted
   [here](./hack/update-deps.sh#L29)**

1. Update your type definitions with: `./hack/update-codegen.sh`.
1. [Add new CRD types](#adding-new-types)
1. [Add and run tests](./test/README.md#tests)

To make changes to these CRDs, you will probably interact with:

- The CRD type definitions in
  [./pkg/apis/pipeline/alpha1](./pkg/apis/pipeline/v1alpha1)
- The reconcilers in [./pkg/reconciler](./pkg/reconciler)
- The clients are in [./pkg/client](./pkg/client) (these are generated by
  `./hack/update-codegen.sh`)

## Install Pipeline

You can stand up a version of this controller on-cluster (to your
`kubectl config current-context`):

```shell
ko apply -f config/
```

### Redeploy controller

As you make changes to the code, you can redeploy your controller with:

```shell
ko apply -f config/controller.yaml
```

### Tear it down

You can clean up everything with:

```shell
ko delete -f config/
```

## Accessing logs

To look at the controller logs, run:

```shell
kubectl -n tekton-pipelines logs $(kubectl -n tekton-pipelines get pods -l app=tekton-pipelines-controller -o name)
```

To look at the webhook logs, run:

```shell
kubectl -n tekton-pipelines logs $(kubectl -n tekton-pipelines get pods -l app=tekton-pipelines-webhook -o name)
```

## Adding new types

If you need to add a new CRD type, you will need to add:

1. A yaml definition in [config/](./config)
1. Add the type to the cluster roles in
   [200-clusterrole.yaml](./config/200-clusterrole.yaml)

_See [the API compatibility policy](api_compatibility_policy.md)._
