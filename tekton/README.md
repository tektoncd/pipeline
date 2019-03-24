# Tekton Repo CI/CD

We dogfood our project by using Tekton Pipelines to build, test and release
Tekton Pipelines!

This directory contains the
[`Tasks`](https://github.com/knative/build-pipeline/blob/master/docs/tasks.md)
and
[`Pipelines`](https://github.com/knative/build-pipeline/blob/master/docs/pipelines.md)
that we (will) use.

TODO(#538): In #538 or #537 we will update
[Prow](https://github.com/knative/build-pipeline/blob/master/CONTRIBUTING.md#pull-request-process)
to invoke these `Pipelines` automatically, but for now we will have to invoke
them manually.

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

To run these `Pipelines` and `Tasks`, you must have Tekton Pipelines installed,
either via
[an official release](https://github.com/knative/build-pipeline/blob/master/docs/install.md)
or
[from `HEAD`](https://github.com/knative/build-pipeline/blob/master/DEVELOPMENT.md#install-pipeline).

TODO(#531): Add the Pipeline, for now all we have are `Tasks` which we can
invoke individually by creating
[`TaskRuns`](https://github.com/knative/build-pipeline/blob/master/docs/taskruns.md)
and
[`PipelineResources`](https://github.com/knative/build-pipeline/blob/master/docs/resources.md).

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
  `PipelineResources` demonstrate how to invoke `publish.yaml`:

  ```bash
  kubectl apply -f tekton/publish.yaml
  kubectl apply -f tekton/publish-run.yaml
  ```

### Authentication

Users executing the publish task must be able to:

- Push to the image registry (production registry is `gcr.io/tekton-releases`)
- Write to the GCS bucket (production bucket is `gs://tekton-releases`)

To be able to publish images via `kaniko` or `ko`, you must be able to push to
your image registry. At the moment, the publish `Task` will try to use your
default service account in the namespace where you create the `TaskRun`. If that
default service account is able to push to your image registry, you are good to
go. Otherwise, you need to use
[a secret annotated with your docker registry credentials](https://github.com/tektoncd/pipeline/blob/master/docs/auth.md#basic-authentication-docker).

TODO(#631) Ensure that we are supporting folks using credentials other than the
cluster defaults; not sure how this will play out with publishing to our prod
registry!

#### Production credentials

TODO(dlorenc, bobcatfish): We need to setup a group which users can be added to,
as well as guidelines around who should be added to this group.

For now, users who need access to our production registry
(`gcr.io/tekton-releases`) and production GCS bucket (`gs://tekton-releases`)
should ping @bobcatfish or @dlorenc to get added to the authorized users.

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
