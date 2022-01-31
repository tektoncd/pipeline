# Tekton Repo CI/CD

_Why does Tekton pipelines have a folder called `tekton`? Cuz we think it would be cool
if the `tekton` folder were the place to look for CI/CD logic in most repos!_

We dogfood our project by using Tekton Pipelines to build, test and release
Tekton Pipelines! This directory contains the
[`Tasks`](https://github.com/tektoncd/pipeline/blob/main/docs/tasks.md) and
[`Pipelines`](https://github.com/tektoncd/pipeline/blob/main/docs/pipelines.md)
that we use.

* [How to create a release](#create-an-official-release)
* [How to create a patch release](#create-a-patch-release)
* [Automated nightly releases](#nightly-releases)
* [Setup releases](#setup)

## Create an official release

To create an official release, follow the steps in the [release-cheat-sheet](./release-cheat-sheet.md).

## Create a patch release

Sometimes we'll find bugs that we want to backport fixes for into previous releases
or discover things that were missing from a release that are required by upstream
consumers of a project. In that case we'll make a patch release. To make one:

1. Create a milestone to track issues and pull requests to include in the release,
   e.g. [v0.12.1](https://github.com/tektoncd/pipeline/milestone/26)
1. The issues when possible should first be fixed and merged into master. As they
   are fixed, add the issues to the milestone and tag them with
   [`needs-cherry-pick`](https://github.com/tektoncd/pipeline/pulls?q=label%3Aneeds-cherry-pick).
1. Create a branch for the release named `release-<version number>x`, e.g. [`release-v0.13.0x`](https://github.com/tektoncd/pipeline/tree/release-v0.13.0x)
   and push it to the repo https://github.com/tektoncd/pipeline (you may need help from
   [an OWNER](../OWNERS_ALIASES) with permission to push) if that release branch does not exist.
1. Use [git cherry-pick](https://git-scm.com/docs/git-cherry-pick) to cherry pick the
   fixes from master into the release branch you have created (use `-x` to include
   the original commit information).
1. Check that you have cherry picked all issues in the milestone and look for any
   pull requests you may have missed with with
   [`needs-cherry-pick`](https://github.com/tektoncd/pipeline/pulls?q=label%3Aneeds-cherry-pick).
1. Remove [`needs-cherry-pick`](https://github.com/tektoncd/pipeline/pulls?q=label%3Aneeds-cherry-pick)
   from all issues that have been cherry picked.
1. [Create an official release](#create-an-official-release) for the patch, with the
   [patch version incremented](https://semver.org/)
1. Close the milestone.

## Nightly releases

[The nightly release pipeline](release-pipeline-nightly.yaml) is
[triggered nightly by Tekton](https://github.com/tektoncd/plumbing/tree/main/tekton).

This Pipeline uses:

- [publish.yaml](publish.yaml)

## Setup

To start from scratch and use these Pipelines and Tasks:

1. [Install Tekton](#install-tekton)
1. [Setup the Tasks and Pipelines](#install-tasks-and-pipelines)
1. [Create the required service account + secrets](#service-account-and-secrets)
1. [Setup post-processing](#setup-post-processing)

### Install Tekton

```bash
# If this is your first time installing Tekton in the cluster you might need to give yourself permission to do so
kubectl create clusterrolebinding cluster-admin-binding-someusername \
  --clusterrole=cluster-admin \
  --user=$(gcloud config get-value core/account)

# Example, Tekton v0.9.1
export TEKTON_VERSION=0.9.1
kubectl apply --filename  https://storage.googleapis.com/tekton-releases/pipeline/previous/v${TEKTON_VERSION}/release.yaml
```

### Install tasks and pipelines

Add all the `Tasks` to the cluster, including the
[`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
Tasks from the
[`tektoncd/catalog`](https://github.com/tektoncd/catalog), and the
[release](https://github.com/tektoncd/plumbing/tree/main/tekton/resources/release) Tasks from
[`tektoncd/plumbing`](https://github.com/tektoncd/plumbing).

Use a version of the [`tektoncdcatalog`](https://github.com/tektoncd/catalog)
tasks that is compatible with version of Tekton being released, usually `master`.
Install Task from plumbing too:

```bash
# Apply the Tasks we are using from the catalog
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/main/task/golang-build/0.3/golang-build.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/main/task/golang-test/0.2/golang-test.yaml

# Apply Tasks and other resources from Plumbing.
#
# If you want to install everything, including tekton-nightly components,
# run this command from the root of the plumbing repo (this requires
# "tekton-nightly" namespace to already be created in your cluster):
kubectl kustomize ./tekton/resources/release | kubectl apply -f -

# If you don't want the tekton-nightly components then run the following
# command from the root of the plumbing repo:
kubectl kustomize ./tekton/resources/release/overlays/default | kubectl apply -f -
```

Apply the tasks from the `pipeline` repo:
```bash
# Apply the Tasks and Pipelines we use from this repo
kubectl apply -f tekton/publish.yaml
kubectl apply -f tekton/release-pipeline.yaml
kubectl apply -f tekton/release-pipeline-nightly.yaml

# Apply the resources - note that when manually releasing you'll re-apply these
kubectl apply -f tekton/resources.yaml
```

`Tasks` and `Pipelines` from this repo are:

- [`publish.yaml`](publish.yaml) - This `Task` uses
  [`kaniko`](https://github.com/GoogleContainerTools/kaniko) to build and
  publish base images, and uses
  [`ko`](https://github.com/google/ko) to build all of the container images we
release and generate the `release.yaml`
- [`release-pipeline.yaml`](./release-pipeline.yaml) - This `Pipeline`
  uses the
  [`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
  `Task`s from the
  [`tektoncd/catalog`](https://github.com/tektoncd/catalog) and
  [`publish.yaml`](publish.yaml)'s `Task`.

### Service account and secrets

In order to release, these Pipelines use the `release-right-meow` service account,
which uses `release-secret` and has
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
access to
[`tekton-releases`]((https://github.com/tektoncd/plumbing/blob/main/gcp.md))
and
[`tekton-releases-nightly`]((https://github.com/tektoncd/plumbing/blob/main/gcp.md)).

After creating these service accounts in GCP, the kubernetes service account and
secret were created with:

```bash
KEY_FILE=release.json
GENERIC_SECRET=release-secret
ACCOUNT=release-right-meow

# Connected to the `prow` in the `tekton-releases` GCP project
GCP_ACCOUNT="$ACCOUNT@tekton-releases.iam.gserviceaccount.com"

# 1. Create a private key for the service account
gcloud iam service-accounts keys create $KEY_FILE --iam-account $GCP_ACCOUNT

# 2. Create kubernetes secret, which we will use via a service account and directly mounting
kubectl create secret generic $GENERIC_SECRET --from-file=./$KEY_FILE

# 3. Add the docker secret to the service account
kubectl apply -f tekton/account.yaml
kubectl patch serviceaccount $ACCOUNT \
  -p "{\"secrets\": [{\"name\": \"$GENERIC_SECRET\"}]}"
```

### Setup post processing

Post-processing services perform post release automated tasks. Today the only
service available collects the `PipelineRun` logs uploads them to the release
bucket. To use release post-processing services, the PipelineResource in
[`resources.yaml`](./resources.yaml) must be configured with a valid targetURL in the
cloud event `PipelineResource` named `post-release-trigger`:

```yaml
apiVersion: tekton.dev/v1alpha1
kind: PipelineResource
metadata:
  name: post-release-trigger
spec:
  type: cloudEvent
  params:
    - name: targetURI
      value: http://el-pipeline-release-post-processing.default.svc.cluster.local:8080 # This has to be changed to a valid URL
```

The targetURL should point to the event listener configured in the cluster.
The example above is configured with the correct value for the  `dogfooding`
cluster, using the event listener `pipeline-release-post-processing`.

## Supporting scripts and images

Some supporting scripts have been written using Python3:

- [koparse](./koparse) - Contains logic for parsing `release.yaml` files created
  by `ko`

### ko image

In order to run `ko`, and to be able to use a cluster's default credentials, we
need an image which contains:

- `ko`
- `golang` - Required by `ko` to build
- `gcloud` - Required to auth with default namespace credentials

The image which we use for this is built from
[tekton/ko/Dockerfile](./ko/Dockerfile).

_[go-containerregistry#383](https://github.com/google/go-containerregistry/issues/383)
is about publishing a `ko` image, which hopefully we'll be able to move it._
