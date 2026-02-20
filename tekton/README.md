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

### Existing approach:

[The nightly release pipeline](release-pipeline.yaml) is
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

### Dogfooding Cluster connectivity and secrets

1. To connect to the cloud instance and OKE cluster we need the Oracle Cloud CLI client. Install Oracle Cloud CLI from https://docs.oracle.com/en-us/iaas/Content/API/SDKDocs/cliinstall.htm

1. The next step is to establish connection from the local client to the cloud instance. Login to the Oracle Cloud Console and create a new `API key` from the user profile.
Follow the steps here: https://docs.oracle.com/en-us/iaas/Content/API/Concepts/apisigningkey.htm#two
Download a Private Key and Add a new API key as mentioned in the doc. Copy the config file to `~/.oci/config` and update the path to the private key file in config.
With this the config is ready for usage by the CLI.

1. Test the connection by doing a get of the OKE cluster id.
Refer here https://docs.oracle.com/en-us/iaas/tools/oci-cli/3.70.0/oci_cli_docs/cmdref/ce.html for the CLI options.
Command to create a kubeconfig in your local could be obtained from console navigating to the OKE > Actions > Access Cluster. Run the command pointing to the PUBLIC_ENDPOINT and we should be connected to the cluster.

1. [Setup a context to connect to the dogfooding cluster](./release-cheat-sheet.md#setup-dogfooding-context) 

1. NOTE: When executing release pipelines, some tasks require OCI CLI commands which need credentials. The OCI credentials secret is already deployed to the dogfooding cluster via terraform and is mounted as a workspace to tasks that require it (such as the precheck task). Release managers do not need to create this secret manually. This is stated here for troubleshooting purposes.

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

### GitHub Action based approach:

The GitHub Actions workflow provides an alternative approach for automated nightly releases with enhanced CI/CD capabilities and better integration with GitHub infrastructure.

[The nightly release workflow](../.github/workflows/nightly-release.yaml) is triggered daily and uses:

- [release-nightly-pipeline.yaml](release-nightly-pipeline.yaml) - Tekton Pipeline for nightly releases
- [publish-nightly.yaml](publish-nightly.yaml) - Tekton Task for building and publishing nightly images

#### Key Features:

**Automated Scheduling:**
- Runs daily at 03:00 UTC via cron schedule
- Supports manual triggering with customizable parameters
- Intelligent change detection - only releases when there are recent commits (configurable)

**Multi-mode Operation:**
- **Production mode**: For `tektoncd/pipeline` repository with full release capabilities
- **Fork mode**: For testing in forks with isolated buckets and registries

#### Usage:

**Scheduled Release:**
The workflow runs automatically every night and will create a release if:
- There have been commits in the last 25 hours, OR
- Force release is enabled, OR
- It's manually triggered

**Manual Release:**
```bash
# Trigger via GitHub UI or CLI
gh workflow run nightly-release.yaml \
  --field kubernetes_version=v1.33.0 \
  --field force_release=true \
  --field dry_run=false
```

**Fork Testing:**
For testing in forks, the workflow automatically:
- Uses a test bucket pattern: `gs://tekton-releases-nightly-{repo-owner}`
- Publishes to `ghcr.io/{owner}/pipeline/*` instead of production registry
- Skips certain production-only validations

#### Output:

The workflow generates:
- Container images tagged with `vYYYYMMDD-{sha7}` format
- Release YAML manifests uploaded to GCS bucket
- Multi-architecture image support
- Comprehensive build logs and artifacts

This approach provides better observability, easier debugging, and more flexible configuration compared to the traditional Tekton-only pipeline approach.

