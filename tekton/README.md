# Tekton Repo CI/CD

We dogfood our project by using Tekton Pipelines to build, test and release
Tekton Pipelines!

This directory contains the
[`Tasks`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md) and
[`Pipelines`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md)
that we (will) use.

TODO(#538): In #538 or #537 we will update
[Prow](https://github.com/tektoncd/pipeline/blob/master/CONTRIBUTING.md#pull-request-process)
to invoke these `Pipelines` automatically, but for now we will have to invoke
them manually.

## Pull Request Pipeline

The `Tasks` which are going to make up our Pull Request Pipeline are:

- [`ci-unit-test.yaml`](ci-unit-test.yaml) — This `Task` uses `go test` to run
  the Pipeline unit tests.
- [`ci-lint.yaml`](ci-lint.yaml) — This `Task` uses
  [`golangci-lint`](https://github.com/golangci/golangci-lint) to validate the
  code based on common best practices.

TODO(#922) & TODO(#860): Add the Pipeline and hook it up with Prow, for now all
we have are `Tasks` which we can invoke individually by creating
[`TaskRuns`](https://github.com/tektoncd/pipeline/blob/master/docs/taskruns.md)
and
[`PipelineResources`](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md).

## Release Pipeline

The `Tasks` which make up our release `Pipeline` are:

- [`ci-images.yaml`](ci-images.yaml) - This `Task` uses
  [`kaniko`](https://github.com/GoogleContainerTools/kaniko) to build and
  publish [images for the CI itself](#supporting-images), which can then be used
  as `steps` in downstream `Tasks`
- [`publish.yaml`](publish.yaml) - This `Task` uses
  [`kaniko`](https://github.com/GoogleContainerTools/kaniko) to build and
  publish base images, and uses
  [`ko`](https://github.com/google/go-containerregistry/tree/master/cmd/ko) to
  build all of the container images we release and generate the `release.yaml`

### Running

To run these `Pipelines` and `Tasks`, you must have Tekton Pipelines installed
(in your own kubernetes cluster) either via
[an official release](https://github.com/tektoncd/pipeline/blob/master/docs/install.md)
or
[from `HEAD`](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#install-pipeline).

TODO(#531): Add the Pipeline, for now all we have are `Tasks` which we can
invoke individually by creating
[`TaskRuns`](https://github.com/tektoncd/pipeline/blob/master/docs/taskruns.md)
and
[`PipelineResources`](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md).

TODO(#569): Normally we'd use the image `PipelineResources` to control which
image registry the images are pushed to. However since we have so many images,
all going to the same registry, we are cheating and using a parameter for the
image registry instead.

- [`ciimages-run.yaml`](ci-images-run.yaml) - This example `TaskRun` and
  `PipelineResources` demonstrate how to invoke `ci-images.yaml`:

  ```bash
  kubectl apply -f tekton/ci-images.yaml
  kubectl apply -f tekton/ci-images-run.yaml
  ```

- [`publish-run.yaml`](publish-run.yaml) - This example `TaskRun` and
  `PipelineResources` demonstrate how to invoke `publish.yaml` (see
  [Creating a new release](#creating-a-new-release))

#### Creating a new release

The `TaskRun` will use

- The kubernetes service account [`release-right-meow`](account.yaml), which by
  default has no associated secrets
- A secret called `release-secret`

It needs to run with a service account in the target GCP project with
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
access).

To run the `publish-tekton-pipelines` `Task` and create a release:

1. Pick the revision you want to release and replace the value of the
   `PipelineResource` [`tekton-pipelines` `revision`](publish-run.yaml#11),
   e.g.:

   ```yaml
   - name: revision
     value: 67efb48746a9d5d7d3b9b5c5cc210de7a47c6ebc # REPLACE with your own commit
   ```

2. Change the value of the `PipelineRun` [`publish-run`'s `versionTag`
   parameter], e.g.:

   ```yaml
   params:
     - name: versionTag
       value: 0.2.0 # REPLACE with the version you want to release
   ```

3. To run against your own infrastructure (not needed for actual releases), also
   replace the `imageRegistry` param:

   ```yaml
   - name: imageRegistry
     value: gcr.io/tekton-releases # REPLACE with your own registry
   ```

   And the `location` of the `tekton-bucket`:

   ```yaml
   - name: location
     value: gs://tekton-releases # REPLACE with your own bucket
   ```

4. Setup the required credentials for the `release-right-meow` service acount,
   either:

   - For
     [the GCP service account `release-right-meow@tekton-releases.iam.gserviceaccount.com`](#production-service-account)
     which has the proper authorization to release the images and yamls in
     [our `tekton-releases` GCP project](https://github.com/tektoncd/plumbing#prow)
   - For
     [your own GCP service account](https://cloud.google.com/iam/docs/creating-managing-service-accounts)
     if running against your own infrastructure

   ```bash
   KEY_FILE=release.json
   GENERIC_SECRET=release-secret
   ACCOUNT=release-right-meow
   # Replace with your own service account if using your own infra
   GCP_ACCOUNT="release-right-meow@tekton-releases.iam.gserviceaccount.com"

    # 1. Create a private key for the service account, which you can use
    gcloud iam service-accounts keys create --iam-account $GCP_ACCOUNT $KEY_FILE

    # 2. Create kubernetes secret, which we will use via a service account and directly mounting
    kubectl create secret generic $GENERIC_SECRET --from-file=./$KEY_FILE

    # 3. Add the docker secret to the service account
    kubectl apply -f tekton/account.yaml
    kubectl patch serviceaccount $ACCOUNT \
     -p "{\"secrets\": [{\"name\": \"$GENERIC_SECRET\"}]}"
   ```

5. Run the `publish-tekton-pipelines` `Task`:

   ```bash
   kubectl apply -f tekton/publish.yaml
   kubectl apply -f tekton/publish-run.yaml
   ```

### Authentication

Users executing the publish task must be able to:

- Push to the image registry (production registry is `gcr.io/tekton-releases`)
- Write to the GCS bucket (production bucket is `gs://tekton-releases`)

TODO: To be able to publish images via `kaniko` or `ko`, you must be able to
push to your image registry. At the moment, the publish `Task` will try to use
your default service account in the namespace where you create the `TaskRun`. If
that default service account is able to push to your image registry, you are
good to go. Otherwise, you need to use
[a secret annotated with your docker registry credentials](https://github.com/tektoncd/pipeline/blob/master/docs/auth.md#basic-authentication-docker).

#### Production credentials

[Members of the Tekton governing board](https://github.com/tektoncd/community/blob/master/governance.md)
[have access to the underlying resources](https://github.com/tektoncd/community/blob/master/governance.md#permissions-and-access).

Users who need access to our production registry (`gcr.io/tekton-releases`) and
production GCS bucket (`gs://tekton-releases`) should ping
[a member of the governing board](https://github.com/tektoncd/community/blob/master/governance.md)
to request access to
[the production service account](#production-service-account).

##### Production service account

TODO(christiewilson, dlorenc): add a group which has access to this service
account

The GCP service account for creating release is
`release-right-meow@tekton-releases.iam.gserviceaccount.com`. This account has
the role
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
in order to be able to read and write buckets and images.

Users who need access to this service account should ping
[a member of the governing board](https://github.com/tektoncd/community/blob/master/governance.md).

## Supporting scripts

Some supporting scripts have been written using Python 2.7:

- [koparse](./koparse) - Contains logic for parsing `release.yaml` files created
  by `ko`

## Supporting images

TODO(#639) Ensure we are using the images that are published by the `Pipeline`
itself.

These images are built and published to be used by the release Pipeline itself.

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
