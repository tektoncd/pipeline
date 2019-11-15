# Tekton Repo CI/CD

_Why does Tekton pipelines have a folder called `tekton`? Cuz we think it would be cool
if the `tekton` folder were the place to look for CI/CD logic in most repos!_

We dogfood our project by using Tekton Pipelines to build, test and release
Tekton Pipelines!

This directory contains the
[`Tasks`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md) and
[`Pipelines`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md)
that we use.

The Pipelines and Tasks in this folder are used for:

1. [Manually creating official releases from the official cluster](#create-an-official-release)
1. [Automated nightly releases](#nightly-releases)

To start from scratch and use these Pipelines and Tasks:

1. [Install Tekton v0.3.1](#install-tekton)
1. [Setup the Tasks and Pipelines](#setup)
1. [Create the required service account + secrets](#service-account-and-secrets)

## Create an official release

Official releases are performed from the `dogfooding` cluster
[in the `tekton-releases` GCP project](https://github.com/tektoncd/plumbing/blob/master/gcp.md).
This cluster [already has the correct version of Tekton installed](#install-tekton).

To make a new release:

1. (Optionally) [Apply the latest versions of the Tasks + Pipelines](#setup)
1. (If you haven't already) [Install `tkn`](https://github.com/tektoncd/cli#installing-tkn)
1. [Run the Pipeline](#run-the-pipeline)
1. Create the new tag and release in GitHub
   ([see one of way of doing that here](https://github.com/tektoncd/pipeline/issues/530#issuecomment-477409459)).
   _TODO(#530): Automate as much of this as possible with Tekton._
1. Add an entry to [the README](../README.md) at `HEAD` for docs and examples for
   the new release ([README.md#read-the-docs](../README.md#read-the-docs)).
1. Update the new release in GitHub with the same links to the docs and examples, see
   [v0.1.0](https://github.com/tektoncd/pipeline/releases/tag/v0.1.0) for example.

### Run the Pipeline

To use [`tkn`](https://github.com/tektoncd/cli) to run the `publish-tekton-pipelines` `Task` and create a release:

1. Pick the revision you want to release and update the
   [`resources.yaml`](./resources.yaml) file to add a
   `PipelineResoruce` for it, e.g.:

   ```yaml
   apiVersion: tekton.dev/v1alpha1
   kind: PipelineResource
   metadata:
     name: tekton-pipelines-vX-Y-Z
   spec:
     type: git
     params:
     - name: url
       value: https://github.com/tektoncd/pipeline
     - name: revision
       value: revision-for-vX.Y.Z-invalid-tags-boouuhhh # REPLACE with the commit you'd like to build from (not a tag, since that's not created yet)
   ```

 1. To use release post-processing services, update the
    [`resources.yaml`](./resources.yaml) file to add a valid targetURL in the
    cloud event `PipelineResoruce` named `post-release-trigger`:

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
    cluster.

1. To run against your own infrastructure (if you are running
   [in the production cluster](https://github.com/tektoncd/plumbing#prow) the
   default account should already have these creds, this is just a bonus - plus
   `release-right-meow` might already exist in the cluster!), also setup the
   required credentials for the `release-right-meow` service account, either:

   - For
     [the GCP service account `release-right-meow@tekton-releases.iam.gserviceaccount.com`](#production-service-account)
     which has the proper authorization to release the images and yamls in
     [our `tekton-releases` GCP project](https://github.com/tektoncd/plumbing#prow)
   - For
     [your own GCP service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts)
     if running against your own infrastructure


1. [Connect to the production cluster](https://github.com/tektoncd/plumbing#prow):

    ```bash
    gcloud container clusters get-credentials dogfooding --zone us-central1-a --project tekton-releases
    ```

1. Run the `release-pipeline` (assuming you are using the production cluster and
   [all the Tasks and Pipelines already exist](#setup)):

   ```shell
   # Create the resoruces - i.e. set the revision that you wan to build from
   kubectl apply -f tekton/resources.yaml

   # Change the environment variable to the version you would like to use.
   # Be careful: due to #983 it is possible to overwrite previous releases.
   export VERSION_TAG=v0.X.Y
   export IMAGE_REGISTRY=gcr.io/tekton-releases

   # Double-check the git revision that is going to be used for the release:
   kubectl get pipelineresource/tekton-pipelines-git -o=jsonpath="{'Target Revision: '}{.spec.params[?(@.name == 'revision')].value}{'\n'}"

   tkn pipeline start \
		--param=versionTag=${VERSION_TAG} \
		--param=imageRegistry=${IMAGE_REGISTRY} \
		--serviceaccount=release-right-meow \
		--resource=source-repo=tekton-pipelines-git \
		--resource=bucket=tekton-bucket \
		--resource=builtBaseImage=base-image \
		--resource=builtEntrypointImage=entrypoint-image \
		--resource=builtKubeconfigWriterImage=kubeconfigwriter-image \
		--resource=builtCredsInitImage=creds-init-image \
		--resource=builtGitInitImage=git-init-image \
		--resource=builtNopImage=nop-image \
		--resource=builtControllerImage=controller-image \
		--resource=builtWebhookImage=webhook-image \
		--resource=builtDigestExporterImage=digest-exporter-image \
		--resource=builtPullRequestInitImage=pull-request-init-image \
		--resource=builtGcsFetcherImage=gcs-fetcher-image \
		--resource=notification=post-release-trigger
		pipeline-release
   ```

_TODO(#569): Normally we'd use the image `PipelineResources` to control which
image registry the images are pushed to. However since we have so many images,
all going to the same registry, we are cheating and using a parameter for the
image registry instead._

## Nightly releases

[The nightly release pipeline](release-pipeline-nightly.yaml) is
[triggered nightly by Prow](https://github.com/tektoncd/plumbing/tree/master/prow).

This Pipeline uses:

- [ci-images.yaml](ci-images.yaml)
- [publish-nightly.yaml](publish-nightly.yaml) (See [triggers#87](https://github.com/tektoncd/triggers/issues/87))

The nightly release Pipeline is currently missing Tasks which we want to add once we are able:

- The unit tests aren't run due to the data race reported in [#1124](http://github.com/tektoncd/pipeline/issues/1124)
- Linting isn't run due to it being flakey [#1205](http://github.com/tektoncd/pipeline/issues/1205)
- Build isn't run because it uses `workingDir` which is broken in v0.3.1 ([kubernetes/test-infra#13948](https://github.com/kubernetes/test-infra/issues/13948))

## Install Tekton

Some of the Pipelines and Tasks in this repo work with v0.3.1 due to
[Prow #13948](https://github.com/kubernetes/test-infra/issues/13948), so that
they can be used [with Prow](https://github.com/tektoncd/plumbing/tree/master/prow).

Specifically, nightly releases are triggered by Prow, so they are compatible with
v0.3.1, while full releases are triggered manually and require Tekton >= v0.7.0.

```bash
# If this is your first time installing Tekton in the cluster you might need to give yourself permission to do so
kubectl create clusterrolebinding cluster-admin-binding-someusername \
  --clusterrole=cluster-admin \
  --user=$(gcloud config get-value core/account)

# For Tekton v0.3.1 - apply version v0.3.1
kubectl apply --filename  https://storage.googleapis.com/tekton-releases/previous/v0.3.1/release.yaml

# For Tekton v0.7.0 - apply version v0.7.0 - Do not apply both versions in the same cluster!
kubectl apply --filename  https://storage.googleapis.com/tekton-releases/previous/v0.3.1/release.yaml
```

## Setup

Add all the `Tasks` to the cluster, including the
[`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
Tasks from the
[`tektoncd/catalog`](https://github.com/tektoncd/catalog), and the
[release pre-check](https://github.com/tektoncd/plumbing/tree/master/tekton/) Task from
[`tektoncd/plumbing`](https://github.com/tektoncd/plumbing).

For nightly releases, use a version of the [`tektoncdcatalog`](https://github.com/tektoncd/catalog)
tasks that is compatible with Tekton v0.3.1:

```bash
# Apply the Tasks we are using from the catalog
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/14d38f2041312b0ad17bc079cfa9c0d66895cc7a/golang/lint.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/14d38f2041312b0ad17bc079cfa9c0d66895cc7a/golang/build.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/14d38f2041312b0ad17bc079cfa9c0d66895cc7a/golang/tests.yaml
```

For full releases, use a version of the [`tektoncdcatalog`](https://github.com/tektoncd/catalog)
tasks that is compatible with Tekton v0.7.0 (`master`) and install the pre-release
check Task from plumbing too:

```bash
# Apply the Tasks we are using from the catalog
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/lint.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/build.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/tests.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/plumbing/master/tekton/prerelease_checks.yaml
```

Apply the tasks from the `pipeline` repo:
```bash
# Apply the Tasks and Pipelines we use from this repo
kubectl apply -f tekton/ci-images.yaml
kubectl apply -f tekton/publish.yaml
kubectl apply -f tekton/publish-nightly.yaml
kubectl apply -f tekton/release-pipeline.yaml
kubectl apply -f tekton/release-pipeline-nightly.yaml

# Apply the resources - note that when manually releasing you'll re-apply these
kubectl apply -f tekton/resources.yaml
```

`Tasks` from this repo are:

- [`ci-images.yaml`](ci-images.yaml) - This `Task` uses
  [`kaniko`](https://github.com/GoogleContainerTools/kaniko) to build and
  publish [images for the CI itself](#supporting-images), which can then be used
  as `steps` in downstream `Tasks`
- [`publish.yaml`](publish.yaml) - This `Task` uses
  [`kaniko`](https://github.com/GoogleContainerTools/kaniko) to build and
  publish base images, and uses
  [`ko`](https://github.com/google/go-containerregistry/tree/master/cmd/ko) to
  build all of the container images we release and generate the
  `release.yaml`
- [`release-pipeline.yaml`](./release-pipeline.yaml) - This `Pipeline`
  uses the
  [`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
  `Task`s from the
  [`tektoncd/catalog`](https://github.com/tektoncd/catalog) and
  [`publish.yaml`](publish.yaml)'s `Task`.

## Service account and secrets

In order to release, these Pipelines use the `release-right-meow` service account,
which uses `release-secret` and has
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
access to
[`tekton-releases`]((https://github.com/tektoncd/plumbing/blob/master/gcp.md))
and
[`tekton-releases-nightly`]((https://github.com/tektoncd/plumbing/blob/master/gcp.md)).

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

## Supporting scripts and images

Some supporting scripts have been written using Python 2.7:

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
