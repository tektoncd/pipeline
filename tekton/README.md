# Tekton Repo CI/CD

_Why does Tekton pipelines have a folder called `tekton`? Cuz we think it would be cool
if the `tekton` folder were the place to look for CI/CD logic in most repos!_

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

The pull request pipeline will use the
[`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
Tasks from the
[`tektoncd/catalog`](https://github.com/tektoncd/catalog). To add them
to your cluster:

```
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/lint.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/build.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/tests.yaml
```

TODO(#922) & TODO(#860): Add the Pipeline and hook it up with Prow, for now all
we have are `Tasks` which we can invoke individually by creating
[`TaskRuns`](https://github.com/tektoncd/pipeline/blob/master/docs/taskruns.md)
and
[`PipelineResources`](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md).

## Release Pipeline

The release pipeline uses the
[`golang`](https://github.com/tektoncd/catalog/tree/master/golang)
Tasks from the
[`tektoncd/catalog`](https://github.com/tektoncd/catalog). To add them
to your cluster:

```
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/lint.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/build.yaml
kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/tests.yaml
```

The *local* `Tasks` which make up our release `Pipeline` are:

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

The official releases [are performed from the `prow` cluster in the `tekton-releases`
GCP project](https://github.com/tektoncd/plumbing#prow). To release you will want to:

1. Install / update Tekton in the kubernetes cluster you'll be running against either via:

  * [An official release](https://github.com/tektoncd/pipeline/blob/master/docs/install.md)
  * [From `HEAD`](https://github.com/tektoncd/pipeline/blob/master/DEVELOPMENT.md#install-pipeline)

  If this is your first time running in the cluster, you will need to give yourself admin permissions
  in the cluster in order to deploy Tekton pipelines, e.g.:

  ```bash
  kubectl create clusterrolebinding cluster-admin-binding-someusername \
    --clusterrole=cluster-admin \
    --user=$(gcloud config get-value core/account)
  ```

2. [Run the Pipeline](#run-the-pipeline). Note that since we don't yet have an actual Pipeline (#531)
   we often just [create the release](#creating-a-new-release) and we skip the bit where we publish
   the ci images (which rarely change anyway). Hashtag lazy manual anti-pattern.

3. Create the new tag and release in GitHub
   ([see one of way of doing that here](https://github.com/tektoncd/pipeline/issues/530#issuecomment-477409459)).
   _TODO(#530): Automate as much of this as possible with Tekton._

4. Add an entry to [the README](../README.md) at `HEAD` for docs and examples for the new release
   ([README.md#read-the-docs](README.md#read-the-docs)).

5. Update the new release in GitHub with the same links to the docs and examples, see
   [v0.1.0](https://github.com/tektoncd/pipeline/releases/tag/v0.1.0) for example.

### Run the Pipeline

TODO(#531): Add the Pipeline, for now all we have are `Tasks` which we can
invoke individually by creating
[`TaskRuns`](https://github.com/tektoncd/pipeline/blob/master/docs/taskruns.md)
and
[`PipelineResources`](https://github.com/tektoncd/pipeline/blob/master/docs/resources.md).

TODO(#569): Normally we'd use the image `PipelineResources` to control which
image registry the images are pushed to. However since we have so many images,
all going to the same registry, we are cheating and using a parameter for the
image registry instead.

- [`ci-images-run.yaml`](ci-images-run.yaml) - This example `TaskRun` and
  `PipelineResources` demonstrate how to invoke `ci-images.yaml` (see
  [Build and push the CI image](#creating-ci-image))

- [`publish-run.yaml`](publish-run.yaml) - This example `TaskRun` and
  `PipelineResources` demonstrate how to invoke `publish.yaml` (see
  [Creating a new release](#creating-a-new-release))
  
- You can use [`tkn`](https://github.com/tektoncd/cli) to run the [release
  pipeline](./release-pipeline.yaml) (see [Creating a new
  release](#creating-a-new-release))

#### Setting up your credentials

Setup the required credentials for the `release-right-meow` service account, either:

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
```

The value of GCP_ACCOUNT for your own infrastructure is `[SA-NAME]@[PROJECT-ID].iam.gserviceaccount.com`.
`[SA-NAME]` is the name of the service account, and `[PROJECT-ID]` is the ID of
your Google Cloud Platform project. Make sure you have both of them created for your own
account, before proceeding with the following commands. Please refer to [Google Projects](https://cloud.google.com/resource-manager/docs/creating-managing-projects)
to create the project, and [Google Service Accounts](https://cloud.google.com/iam/docs/creating-managing-service-accounts) to create the service account, if necessary.

```bash
# 1. Create a private key for the service account, which you can use
gcloud iam service-accounts keys create --iam-account $GCP_ACCOUNT $KEY_FILE

# 2. Create kubernetes secret, which we will use via a service account and directly mounting
kubectl create secret generic $GENERIC_SECRET --from-file=./$KEY_FILE

# 3. Add the docker secret to the service account
kubectl apply -f tekton/account.yaml
kubectl patch serviceaccount $ACCOUNT \
  -p "{\"secrets\": [{\"name\": \"$GENERIC_SECRET\"}]}"
```

#### Creating CI image

After the credentials are configured, you can run the following commands to
build and push the CI image upstream.

```bash
kubectl apply -f tekton/ci-images.yaml
kubectl apply -f tekton/ci-images-run.yaml
```

#### Creating a new release

Currently, all the official release processes are conducted under [Google Kubernetes Engine](https://cloud.google.com/kubernetes-engine/docs/).
Please follow the tutorial [here](https://cloud.google.com/kubernetes-engine/docs/quickstart) to launch your own infrastructure, if needed.

The `TaskRun` will use

- The kubernetes service account [`release-right-meow`](account.yaml), which by
  default has no associated secrets
- A secret called `release-secret`

It needs to run with a service account in the target GCP project with
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
access), such as [the production service account](#production-service-account).

To run the `publish-tekton-pipelines` `Task` and create a release:

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
       value: https://github.com/tektoncd/pipeline # REPLACE with your own fork
     - name: revision
       value: vX.Y.Z-invalid-tags-boouuhhh # REPLACE with your own commit
   ```
   
   Also, validate that the `tektoncd-bucket` points to the correct
   bucket if you are running the release on your own infrastructure.
  
   ```yaml
   - name: location
     value: gs://tekton-releases # REPLACE with your own bucket
   ```

2. To run an official release [using the production cluster](https://github.com/tektoncd/plumbing#prow):

  ```bash
  gcloud container clusters get-credentials prow --zone us-central1-a --project tekton-releases
  ```

3. To run against your own infrastructure (if you are running
   [in the production cluster](https://github.com/tektoncd/plumbing#prow) the default account should
   already have these creds, this is just a bonus - plus `release-right-meow` might already exist in the
   cluster!), also setup the required credentials for the `release-right-meow` service account, either:

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

4. To run the release you can either create a `PipelineRun` using
   [`tkn`](https://github.com/tektoncd/cli), or using a yaml file.

	You will need to set the following parameters:
	- `versionTag`: to set the tag to use for published images
	
	  **TODO(#983) Be careful! if you use a tag that has already been released, you
	  can overwrite a previous release!**
	  
	- `imageRegistry`: the default value points to
      `gcr.io/tekton-releases`, to run against your own infrastructure
      (not needed for actual releases) set it to your registry.

6. Run the `release-pipeline`:

   ```shell
   # If you are running in a cluster you've run this in previously,
   # delete the previous run and resources

   # Apply golang tasks from the catalog
   kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/lint.yaml
   kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/build.yaml
   kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/master/golang/tests.yaml

   # Apply the publish Task
   kubectl apply -f tekton/publish.yaml

   # Create the resoruces
   kubectl apply -f tekton/resources.yaml
   ```

   If you are using [`tkn`](https://github.com/tektoncd/cli), you can
   run the following command.
	
   ```shell
   # Do not forget to change those environment variables !
   export VERSION_TAG=v0.X.Y

   tkn pipeline start \
		--param=versionTag=${VERSION_TAG} \
		--serviceaccount=release-right-meow \
		--resource=source-repo=tekton-pipelines-git \
		--resource=bucket=tekton-bucket \
		--resource=builtBaseImage=base-image \
		--resource=builtEntrypointImage=entrypoint-image \
		--resource=builtKubeconfigWriterImage=kubeconfigwriter-image \
		--resource=builtCredsInitImage=creds-init-image \
		--resource=builtGitInitImage=git-init-image \
		--resource=builtNopImage=nop-image \
		--resource=builtBashImage=bash-image \
		--resource=builtGsutilImage=gsutil-image \
		--resource=builtControllerImage=controller-image \
		--resource=builtWebhookImage=webhook-image \
		--resource=builtDigestExporterImage=digest-exporter-image \
		--resource=builtPullRequestInitImage=pull-request-init-image \
		pipeline-release
   ```
	
   If you don't want to use `tkn`, you can use
  [`release-pipeline-run.yaml`](./release-pipeline-run.yaml)'s
  `PipelineRun`. **Do not forget to update the `params` and the
  `source-repo` resource**.  

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

The GCP service account for creating release is
`release-right-meow@tekton-releases.iam.gserviceaccount.com`. This account has
the role
[`Storage Admin`](https://cloud.google.com/container-registry/docs/access-control)
in order to be able to read and write buckets and images.

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
