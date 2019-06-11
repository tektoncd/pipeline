# Developing

## Getting started

1. [Ramp up on kubernetes and CRDs](#ramp-up)
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
[running the controllers with `ko`](#install-pipeline)).

### Ramp up

Welcome to the project!! You may find these resources helpful to ramp up on some
of the technology this project is built on. This project extends Kubernetes (aka
`k8s`) with Custom Resource Definitions (CRDSs). To find out more:

- [The Kubernetes docs on Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) -
  These will orient you on what words like "Resource" and "Controller"
  concretely mean
- [Understanding Kubernetes objects](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/) -
  This will further solidify k8s nomenclature
- [API conventions - Types(kinds)](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#types-kinds) -
  Another useful set of words describing words. "Objects" and "Lists" in k8s
  land
- [Extend the Kubernetes API with CustomResourceDefinitions](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/)-
  A tutorial demonstrating how a Custom Resource Definition can be added to
  Kubernetes without anything actually "happening" beyond being able to list
  Objects of that kind

At this point, you may find it useful to return to these `Tekton Pipeline` docs:

- [Tekton Pipeline README](https://github.com/tektoncd/pipeline/blob/master/docs/README.md) -
  Some of the terms here may make more sense!
- Install via
  [official installation docs](https://github.com/tektoncd/pipeline/blob/master/docs/install.md)
  or continue though [getting started for development](#getting-started)
- [Tekton Pipeline "Hello World" tutorial](https://github.com/tektoncd/pipeline/blob/master/docs/tutorial.md) -
  Define `Tasks`, `Pipelines`, and `PipelineResources`, see what happens when
  they are run

### Checkout your fork

The Go tools require that you clone the repository to the
`src/github.com/tektoncd/pipeline` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/tektoncd
cd ${GOPATH}/src/github.com/tektoncd
git clone git@github.com:${YOUR_GITHUB_USERNAME}/pipeline.git
cd pipeline
git remote add upstream git@github.com:tektoncd/pipeline.git
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
1. [`ko`](https://github.com/google/ko): For development. `ko` version v0.1 or
   higher is required for `pipeline` to work correctly.
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   interacting with your kube cluster

Your [`$GOPATH`] setting is critical for `ko apply` to function properly: a
successful run will typically involve building pushing images instead of only
configuring Kubernetes resources.

## Kubernetes cluster

Docker for Desktop using an edge version has been proven to work for both
developing and running Pipelines. The recommended configuration is:

- Kubernetes version 1.11 or later
- 4 vCPU nodes (`n1-standard-4`)
- Node autoscaling, up to 3 nodes
- API scopes for cloud-platform

To setup a cluster with GKE:

1. [Install required tools and setup GCP project](https://github.com/knative/docs/blob/master/docs/install/Knative-with-GKE.md#before-you-begin)
   (You may find it useful to save the ID of the project in an environment
   variable (e.g. `PROJECT_ID`).

1. Create a GKE cluster (with `--cluster-version=latest` but you can use any
   version 1.11 or later):

   ```bash
   export PROJECT_ID=my-gcp-project
   export CLUSTER_NAME=mycoolcluster

   gcloud container clusters create $CLUSTER_NAME \
    --enable-autoscaling \
    --min-nodes=1 \
    --max-nodes=3 \
    --scopes=cloud-platform \
    --enable-basic-auth \
    --no-issue-client-certificate \
    --project=$PROJECT_ID \
    --region=us-central1 \
    --machine-type=n1-standard-4 \
    --image-type=cos \
    --num-nodes=1 \
    --cluster-version=latest
   ```

   Note that
   [the `--scopes` argument to `gcloud container cluster create`](https://cloud.google.com/sdk/gcloud/reference/container/clusters/create#--scopes)
   controls what GCP resources the cluster's default service account has access
   to; for example to give the default service account full access to your GCR
   registry, you can add `storage-full` to your `--scopes` arg.

1. Grant cluster-admin permissions to the current user:

   ```bash
   kubectl create clusterrolebinding cluster-admin-binding \
   --clusterrole=cluster-admin \
   --user=$(gcloud config get-value core/account)
   ```

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

### Install in custom namespace

1. To install into a different namespace you can use this script :

```shell
#!/usr/bin/env bash
set -e

# Set your target namespace here
TARGET_NAMESPACE=new-target-namespace

ko resolve -f config | sed -e '/kind: Namespace/!b;n;n;s/:.*/: '"${TARGET_NAMESPACE}"'/' | \
    sed "s/namespace: tekton-pipelines$/namespace: ${TARGET_NAMESPACE}/" | \
    kubectl apply -f-
kubectl set env deployments --all SYSTEM_NAMESPACE=${TARGET_NAMESPACE} -n ${TARGET_NAMESPACE}
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

To look at the logs for individual `TaskRuns` or `PipelineRuns`, see
[docs on accessing logs](docs/logs.md).

## Adding new types

If you need to add a new CRD type, you will need to add:

1. A yaml definition in [config/](./config)
1. Add the type to the cluster roles in
   [200-clusterrole.yaml](./config/200-clusterrole.yaml)

_See [the API compatibility policy](api_compatibility_policy.md)._
